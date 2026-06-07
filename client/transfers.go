package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gotd/td/tg"
)

// TransferState holds the persisted progress of a resumable upload or download.
type TransferState struct {
	// Key is the unique identifier for this transfer.
	Key string `json:"key"`

	// TotalSize is the total number of bytes to transfer.
	TotalSize int64 `json:"total_size"`

	// DoneBytes is how many bytes have been transferred so far.
	DoneBytes int64 `json:"done_bytes"`

	// FileID is the Telegram upload file ID (non-empty during uploads).
	FileID string `json:"file_id,omitempty"`

	// Parts contains the indices of parts that have been successfully uploaded.
	Parts []int `json:"parts,omitempty"`

	// CreatedAt is when the transfer was first started.
	CreatedAt time.Time `json:"created_at"`
}

// JsonStateStore persists TransferState values as JSON files in a directory.
type JsonStateStore struct {
	// Dir is the directory where state files are written.
	Dir string
}

func (s *JsonStateStore) stateFilePath(key string) string {
	safe := filepath.Base(key)
	if len(safe) > 64 {
		safe = safe[:64]
	}
	return filepath.Join(s.Dir, safe+".transfer.json")
}

// Load reads a previously saved TransferState for the given key.
// Returns nil, nil when no state exists for the key.
func (s *JsonStateStore) Load(key string) (*TransferState, error) {
	data, err := os.ReadFile(s.stateFilePath(key))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read transfer state %q: %w", key, err)
	}
	var state TransferState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("decode transfer state %q: %w", key, err)
	}
	return &state, nil
}

// Save writes state to disk atomically.
func (s *JsonStateStore) Save(state *TransferState) error {
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode transfer state %q: %w", state.Key, err)
	}
	path := s.stateFilePath(state.Key)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write temp state: %w", err)
	}
	return os.Rename(tmp, path)
}

// Delete removes a saved TransferState from disk.
func (s *JsonStateStore) Delete(key string) error {
	path := s.stateFilePath(key)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete transfer state %q: %w", key, err)
	}
	return nil
}

// ResumableUpload uploads a file with resume support backed by a JsonStateStore.
// If a previous upload for resumeKey exists in store, it resumes from where it
// left off; otherwise it starts a fresh upload.
// Returns the InputFile handle upon successful completion.
func (c *MCUBClient) ResumableUpload(ctx context.Context, filePath, resumeKey string, store *JsonStateStore) (tg.InputFileClass, error) {
	if resumeKey == "" {
		resumeKey = filePath
	}

	const partSize = 512 * 1024

	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat file: %w", err)
	}
	fileSize := info.Size()
	fileName := filepath.Base(filePath)
	isBig := fileSize > 10*1024*1024
	partCount := int((fileSize + partSize - 1) / partSize)

	// Restore or initialise state.
	state, err := store.Load(resumeKey)
	if err != nil {
		return nil, fmt.Errorf("load transfer state: %w", err)
	}

	var fileID int64
	startPart := 0

	if state != nil && state.TotalSize == fileSize {
		// Decode previously stored file ID.
		fmt.Sscanf(state.FileID, "%d", &fileID)
		// Determine how many complete parts are already uploaded.
		startPart = int(state.DoneBytes / partSize)
	} else {
		fileID = randInt63()
		state = &TransferState{
			Key:       resumeKey,
			TotalSize: fileSize,
			FileID:    fmt.Sprintf("%d", fileID),
			CreatedAt: time.Now(),
		}
		if saveErr := store.Save(state); saveErr != nil {
			return nil, fmt.Errorf("save initial state: %w", saveErr)
		}
	}

	// Seek to resume point.
	if startPart > 0 {
		if _, err := f.Seek(int64(startPart)*partSize, 0); err != nil {
			return nil, fmt.Errorf("seek to resume offset: %w", err)
		}
	}

	buf := make([]byte, partSize)
	var totalDone int64 = int64(startPart) * partSize

	for partIdx := startPart; partIdx < partCount; partIdx++ {
		n, err := readFull(f, buf)
		if n == 0 {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read part %d: %w", partIdx, err)
		}
		chunk := buf[:n]

		if isBig {
			_, uploadErr := c.client.API().UploadSaveBigFilePart(ctx, &tg.UploadSaveBigFilePartRequest{
				FileID:         fileID,
				FilePart:       partIdx,
				FileTotalParts: partCount,
				Bytes:          chunk,
			})
			if uploadErr != nil {
				return nil, fmt.Errorf("upload big part %d: %w", partIdx, uploadErr)
			}
		} else {
			_, uploadErr := c.client.API().UploadSaveFilePart(ctx, &tg.UploadSaveFilePartRequest{
				FileID:   fileID,
				FilePart: partIdx,
				Bytes:    chunk,
			})
			if uploadErr != nil {
				return nil, fmt.Errorf("upload part %d: %w", partIdx, uploadErr)
			}
		}

		totalDone += int64(n)
		state.DoneBytes = totalDone
		state.Parts = append(state.Parts, partIdx)
		if saveErr := store.Save(state); saveErr != nil {
			// Non-fatal: resume state write failure should not abort upload.
			_ = saveErr
		}
	}

	_ = store.Delete(resumeKey)

	if isBig {
		return &tg.InputFileBig{ID: fileID, Parts: partCount, Name: fileName}, nil
	}
	return &tg.InputFile{ID: fileID, Parts: partCount, Name: fileName}, nil
}

// ResumableDownloadFile downloads a Telegram file with resume support.
// If a previous partial download exists at params.FilePath, the download is
// continued from the byte offset stored in store under resumeKey.
func (c *MCUBClient) ResumableDownloadFile(ctx context.Context, params DownloadMediaParams, resumeKey string, store *JsonStateStore) error {
	if params.FilePath == "" {
		return fmt.Errorf("ResumableDownloadFile requires a non-empty FilePath")
	}
	if resumeKey == "" {
		resumeKey = params.FilePath
	}

	// Resolve media location from the message.
	peer, err := c.resolvePeer(ctx, params.ChatID)
	if err != nil {
		return fmt.Errorf("resolve peer: %w", err)
	}

	histResult, err := c.client.API().MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{
		Peer:     peer,
		OffsetID: params.MessageID + 1,
		Limit:    1,
		MaxID:    params.MessageID,
	})
	if err != nil {
		return fmt.Errorf("get message: %w", err)
	}

	msgs := extractMessages(histResult)
	if len(msgs) == 0 || msgs[0].ID != params.MessageID {
		return fmt.Errorf("message %d not found", params.MessageID)
	}

	msg := msgs[0]
	var location tg.InputFileLocationClass
	var fileSize int64

	switch media := msg.Media.(type) {
	case *tg.MessageMediaPhoto:
		photo, ok := media.Photo.(*tg.Photo)
		if !ok {
			return fmt.Errorf("photo is empty")
		}
		thumbType, sz := pickPhotoThumb(photo.Sizes, params.Thumb)
		fileSize = int64(sz)
		location = &tg.InputPhotoFileLocation{
			ID:            photo.ID,
			AccessHash:    photo.AccessHash,
			FileReference: photo.FileReference,
			ThumbSize:     thumbType,
		}
	case *tg.MessageMediaDocument:
		doc, ok := media.Document.(*tg.Document)
		if !ok {
			return fmt.Errorf("document is empty")
		}
		fileSize = doc.Size
		location = &tg.InputDocumentFileLocation{
			ID:            doc.ID,
			AccessHash:    doc.AccessHash,
			FileReference: doc.FileReference,
			ThumbSize:     "",
		}
	default:
		return fmt.Errorf("message %d has no downloadable media", params.MessageID)
	}

	// Load existing state to determine resume offset.
	state, err := store.Load(resumeKey)
	if err != nil {
		return fmt.Errorf("load transfer state: %w", err)
	}

	startOffset := int64(0)
	if state != nil && state.TotalSize == fileSize {
		startOffset = state.DoneBytes
	} else {
		state = &TransferState{
			Key:       resumeKey,
			TotalSize: fileSize,
			CreatedAt: time.Now(),
		}
		_ = store.Save(state)
	}

	const partSize = 512 * 1024

	flags := os.O_CREATE | os.O_WRONLY
	if startOffset > 0 {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}
	outFile, err := os.OpenFile(params.FilePath, flags, 0o644)
	if err != nil {
		return fmt.Errorf("open output file: %w", err)
	}
	defer outFile.Close()

	offset := startOffset
	for {
		res, err := c.client.API().UploadGetFile(ctx, &tg.UploadGetFileRequest{
			Location: location,
			Offset:   offset,
			Limit:    partSize,
		})
		if err != nil {
			state.DoneBytes = offset
			_ = store.Save(state)
			return fmt.Errorf("download chunk at offset %d: %w", offset, err)
		}
		file, ok := res.(*tg.UploadFile)
		if !ok {
			return fmt.Errorf("unexpected result type %T", res)
		}
		if len(file.Bytes) == 0 {
			break
		}
		if _, writeErr := outFile.Write(file.Bytes); writeErr != nil {
			return fmt.Errorf("write chunk: %w", writeErr)
		}
		offset += int64(len(file.Bytes))
		state.DoneBytes = offset

		_ = store.Save(state)

		if params.ProgressFn != nil {
			params.ProgressFn(offset, fileSize)
		}
		if len(file.Bytes) < partSize {
			break
		}
	}

	_ = store.Delete(resumeKey)
	return nil
}

// readFull reads from r until buf is full or EOF/error is reached.
// Returns the number of bytes read and any error (io.EOF is normalised).
func readFull(r interface{ Read([]byte) (int, error) }, buf []byte) (int, error) {
	n := 0
	for n < len(buf) {
		nn, err := r.Read(buf[n:])
		n += nn
		if err != nil {
			if n > 0 {
				return n, nil
			}
			return 0, err
		}
	}
	return n, nil
}

// randInt63 returns a random positive int64.
func randInt63() int64 {
	var b [8]byte
	_, _ = randRead(b[:])
	v := int64(b[0])<<56 | int64(b[1])<<48 | int64(b[2])<<40 | int64(b[3])<<32 |
		int64(b[4])<<24 | int64(b[5])<<16 | int64(b[6])<<8 | int64(b[7])
	if v < 0 {
		v = -v
	}
	return v
}

// randRead fills b with random bytes using a simple counter-based PRNG seeded
// from the current time. Only used internally to generate upload file IDs.
var randReadSeed = time.Now().UnixNano()

func randRead(b []byte) (int, error) {
	buf := new(bytes.Buffer)
	seed := randReadSeed
	for i := range b {
		seed = seed*6364136223846793005 + 1442695040888963407
		buf.WriteByte(byte(seed >> 56))
		b[i] = buf.Bytes()[0]
		buf.Reset()
	}
	randReadSeed = seed
	return len(b), nil
}
