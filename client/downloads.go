package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/gotd/td/tg"
	"github.com/nulls-brawl-site/telegram-mcub-go/session"
)

// FileTransferState is an alias for session.ResumeState for package ergonomics.
type FileTransferState = session.ResumeState

// DownloadParams holds parameters for downloading a file.
type DownloadParams struct {
	// Location is the Telegram file location to download.
	Location tg.InputFileLocationClass

	// DestPath is the local path where the file will be written.
	DestPath string

	// Resume controls whether to resume an interrupted download.
	Resume bool

	// ResumeKey is a stable string key identifying this download.
	// Defaults to DestPath if empty.
	ResumeKey string

	// StateStore persists download progress for resumption.
	// May be nil (disables resumption even when Resume=true).
	StateStore *session.StateStore

	// DCID is the DC where the file is stored.
	DCID int

	// ProgressFunc is called periodically with (bytesDone, totalBytes).
	ProgressFunc func(bytesDone, totalBytes int64)

	// PartSize is the download chunk size in bytes (default 512 KB).
	PartSize int
}

// DownloadFile downloads a Telegram file to disk, with optional resumption.
func (c *MCUBClient) DownloadFile(ctx context.Context, params DownloadParams) error {
	resumeKey := params.ResumeKey
	if resumeKey == "" {
		resumeKey = params.DestPath
	}

	partSize := params.PartSize
	if partSize <= 0 {
		partSize = 512 * 1024 // 512 KB default
	}

	// Load previous state if resuming.
	var state *FileTransferState
	if params.Resume && params.StateStore != nil {
		var err error
		state, err = params.StateStore.Load(resumeKey, "download")
		if err != nil {
			return fmt.Errorf("load resume state: %w", err)
		}
	}

	var startOffset int64
	if state != nil && !state.Completed {
		startOffset = state.BytesDone
	}

	// Open (or create) the destination file.
	flags := os.O_CREATE | os.O_WRONLY
	if startOffset > 0 {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}
	f, err := os.OpenFile(params.DestPath, flags, 0o644)
	if err != nil {
		return fmt.Errorf("open dest file: %w", err)
	}
	defer f.Close()

	var bytesDone int64 = startOffset
	offset := startOffset

	for {
		result, err := c.client.API().UploadGetFile(ctx, &tg.UploadGetFileRequest{
			Location: params.Location,
			Offset:   offset,
			Limit:    partSize,
		})
		if err != nil {
			if params.StateStore != nil {
				_ = params.StateStore.Save(&session.ResumeState{
					ID:        resumeKey,
					Kind:      "download",
					BytesDone: bytesDone,
					DestPath:  params.DestPath,
				})
			}
			return fmt.Errorf("download chunk at offset %d: %w", offset, err)
		}

		file, ok := result.(*tg.UploadFile)
		if !ok {
			return fmt.Errorf("unexpected upload result type %T", result)
		}

		if len(file.Bytes) == 0 {
			break // no more data
		}

		if _, err := f.Write(file.Bytes); err != nil {
			return fmt.Errorf("write chunk: %w", err)
		}

		bytesDone += int64(len(file.Bytes))
		offset += int64(len(file.Bytes))

		if params.ProgressFunc != nil {
			params.ProgressFunc(bytesDone, -1)
		}

		if len(file.Bytes) < partSize {
			break // last chunk
		}

		// Persist progress.
		if params.StateStore != nil {
			_ = params.StateStore.Save(&session.ResumeState{
				ID:        resumeKey,
				Kind:      "download",
				BytesDone: bytesDone,
				DestPath:  params.DestPath,
			})
		}
	}

	// Mark complete.
	if params.StateStore != nil {
		_ = params.StateStore.Save(&session.ResumeState{
			ID:        resumeKey,
			Kind:      "download",
			BytesDone: bytesDone,
			Completed: true,
			DestPath:  params.DestPath,
		})
	}

	return nil
}

// HistoryExportResult holds the result of a history export operation.
type HistoryExportResult struct {
	// PeerID is the chat the history was exported from.
	PeerID int64

	// TotalMessages is the number of messages exported.
	TotalMessages int

	// OutputPath is the path where the export was written (empty when in-memory only).
	OutputPath string

	// Messages holds all exported messages (in-memory).
	Messages []*tg.Message
}

// ExportHistoryParams holds parameters for exporting chat history.
type ExportHistoryParams struct {
	// PeerID is the numeric peer ID.
	PeerID int64

	// Limit is the max messages to export (0 = all).
	Limit int

	// OffsetID starts from a specific message ID.
	OffsetID int

	// MinDate/MaxDate filter by date range (Unix timestamps; 0 = no filter).
	MinDate int
	MaxDate int

	// BatchSize is the number of messages fetched per request (default 100).
	BatchSize int

	// ProgressFunc is called after each batch with (exportedSoFar, totalEstimate).
	ProgressFunc func(done, total int)
}

// ExportHistory fetches all messages from a chat and returns them as a slice.
func (c *MCUBClient) ExportHistory(ctx context.Context, params ExportHistoryParams) (*HistoryExportResult, error) {
	peer, err := c.resolvePeer(ctx, params.PeerID)
	if err != nil {
		return nil, err
	}

	batchSize := params.BatchSize
	if batchSize <= 0 {
		batchSize = 100
	}

	var (
		allMsgs  []*tg.Message
		offsetID = params.OffsetID
		done     bool
	)

	for !done {
		result, err := c.client.API().MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{
			Peer:       peer,
			OffsetID:   offsetID,
			OffsetDate: params.MaxDate,
			AddOffset:  0,
			Limit:      batchSize,
			MaxID:      0,
			MinID:      0,
			Hash:       0,
		})
		if err != nil {
			return nil, fmt.Errorf("get history: %w", err)
		}

		batch := extractMessages(result)
		if len(batch) == 0 {
			break
		}

		for _, m := range batch {
			if params.MinDate > 0 && m.Date < params.MinDate {
				done = true
				break
			}
			allMsgs = append(allMsgs, m)
		}

		if params.Limit > 0 && len(allMsgs) >= params.Limit {
			allMsgs = allMsgs[:params.Limit]
			break
		}

		if params.ProgressFunc != nil {
			params.ProgressFunc(len(allMsgs), -1)
		}

		offsetID = batch[len(batch)-1].ID

		if len(batch) < batchSize {
			break
		}
	}

	return &HistoryExportResult{
		PeerID:        params.PeerID,
		TotalMessages: len(allMsgs),
		Messages:      allMsgs,
	}, nil
}

// IterHistoryBatches paginates through chat history, calling fn for each batch.
// Return false from fn to stop iteration.
func (c *MCUBClient) IterHistoryBatches(
	ctx context.Context,
	params ExportHistoryParams,
	fn func(batch []*tg.Message) bool,
) error {
	peer, err := c.resolvePeer(ctx, params.PeerID)
	if err != nil {
		return err
	}

	batchSize := params.BatchSize
	if batchSize <= 0 {
		batchSize = 100
	}

	offsetID := params.OffsetID
	total := 0

	for {
		result, err := c.client.API().MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{
			Peer:       peer,
			OffsetID:   offsetID,
			OffsetDate: params.MaxDate,
			AddOffset:  0,
			Limit:      batchSize,
			MaxID:      0,
			MinID:      0,
			Hash:       0,
		})
		if err != nil {
			return fmt.Errorf("get history: %w", err)
		}

		batch := extractMessages(result)
		if len(batch) == 0 {
			break
		}

		// Apply min date filter.
		if params.MinDate > 0 {
			filtered := batch[:0]
			for _, m := range batch {
				if m.Date >= params.MinDate {
					filtered = append(filtered, m)
				}
			}
			batch = filtered
		}

		if len(batch) == 0 {
			break
		}

		if !fn(batch) {
			break
		}

		total += len(batch)

		if params.Limit > 0 && total >= params.Limit {
			break
		}

		if params.ProgressFunc != nil {
			params.ProgressFunc(total, -1)
		}

		offsetID = batch[len(batch)-1].ID

		if len(batch) < batchSize {
			break
		}
	}

	return nil
}

// WriteToFile is a helper that writes exported history messages as a text dump to the given writer.
func WriteToFile(w io.Writer, result *HistoryExportResult) error {
	_, err := fmt.Fprintf(w, "# History export for peer %d — %d messages\n",
		result.PeerID, result.TotalMessages)
	if err != nil {
		return err
	}
	for i, m := range result.Messages {
		if _, err := fmt.Fprintf(w, "[%d] id=%d date=%d text=%q\n",
			i+1, m.ID, m.Date, m.Message); err != nil {
			return err
		}
	}
	return nil
}

// DownloadMediaParams holds parameters for DownloadMedia.
type DownloadMediaParams struct {
	// ChatID is the numeric peer ID of the chat containing the message.
	ChatID int64

	// MessageID is the ID of the message whose media to download.
	MessageID int

	// FilePath is the local path to write the file to.
	// When empty the file is downloaded into memory and returned as bytes.
	FilePath string

	// Thumb is the thumbnail index to download instead of the full file.
	// -1 means the largest available thumbnail; 0 means the smallest.
	// Leave at 0 and set FilePath to download the full file.
	Thumb int

	// ProgressFn is called periodically with (bytesReceived, totalBytes).
	// totalBytes may be -1 when the total size is unknown.
	ProgressFn func(received, total int64)
}

// DownloadMedia downloads media from a message (photo, document, etc.).
// When FilePath is empty the file contents are returned as a byte slice.
// When FilePath is set the file is written to disk and nil bytes are returned.
func (c *MCUBClient) DownloadMedia(ctx context.Context, params DownloadMediaParams) ([]byte, error) {
	peer, err := c.resolvePeer(ctx, params.ChatID)
	if err != nil {
		return nil, fmt.Errorf("resolve peer: %w", err)
	}

	// Fetch the target message.
	histResult, err := c.client.API().MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{
		Peer:     peer,
		OffsetID: params.MessageID + 1,
		Limit:    1,
		MaxID:    params.MessageID,
	})
	if err != nil {
		return nil, fmt.Errorf("get message: %w", err)
	}

	msgs := extractMessages(histResult)
	if len(msgs) == 0 || msgs[0].ID != params.MessageID {
		return nil, fmt.Errorf("message %d not found in chat %d", params.MessageID, params.ChatID)
	}

	msg := msgs[0]
	var location tg.InputFileLocationClass
	var fileSize int64

	switch media := msg.Media.(type) {
	case *tg.MessageMediaPhoto:
		photo, ok := media.Photo.(*tg.Photo)
		if !ok {
			return nil, fmt.Errorf("photo is empty or unavailable")
		}
		thumbType, size := pickPhotoThumb(photo.Sizes, params.Thumb)
		fileSize = int64(size)
		location = &tg.InputPhotoFileLocation{
			ID:            photo.ID,
			AccessHash:    photo.AccessHash,
			FileReference: photo.FileReference,
			ThumbSize:     thumbType,
		}

	case *tg.MessageMediaDocument:
		doc, ok := media.Document.(*tg.Document)
		if !ok {
			return nil, fmt.Errorf("document is empty or unavailable")
		}
		fileSize = doc.Size
		location = &tg.InputDocumentFileLocation{
			ID:            doc.ID,
			AccessHash:    doc.AccessHash,
			FileReference: doc.FileReference,
			ThumbSize:     "",
		}

	default:
		return nil, fmt.Errorf("message %d has no downloadable media (type %T)", params.MessageID, msg.Media)
	}

	if params.FilePath == "" {
		return downloadToMemory(ctx, c, location, fileSize, params.ProgressFn)
	}

	dlErr := c.DownloadFile(ctx, DownloadParams{
		Location:     location,
		DestPath:     params.FilePath,
		ProgressFunc: params.ProgressFn,
	})
	return nil, dlErr
}

// pickPhotoThumb selects a thumbnail type string and byte-size from a slice
// of PhotoSizeClass values. thumb is interpreted as in DownloadMediaParams.Thumb.
func pickPhotoThumb(sizes []tg.PhotoSizeClass, thumb int) (string, int) {
	if len(sizes) == 0 {
		return "s", 0
	}
	idx := thumb
	if idx < 0 || idx >= len(sizes) {
		idx = len(sizes) - 1
	}
	switch s := sizes[idx].(type) {
	case *tg.PhotoSize:
		return s.Type, s.Size
	case *tg.PhotoCachedSize:
		return s.Type, len(s.Bytes)
	case *tg.PhotoSizeProgressive:
		sz := 0
		if len(s.Sizes) > 0 {
			sz = s.Sizes[len(s.Sizes)-1]
		}
		return s.Type, sz
	default:
		return "s", 0
	}
}

// downloadToMemory downloads a Telegram file location into a byte slice.
func downloadToMemory(ctx context.Context, c *MCUBClient, location tg.InputFileLocationClass, fileSize int64, progressFn func(int64, int64)) ([]byte, error) {
	const partSize = 512 * 1024
	var buf bytes.Buffer
	offset := int64(0)
	for {
		res, err := c.client.API().UploadGetFile(ctx, &tg.UploadGetFileRequest{
			Location: location,
			Offset:   offset,
			Limit:    partSize,
		})
		if err != nil {
			return nil, fmt.Errorf("download chunk at offset %d: %w", offset, err)
		}
		file, ok := res.(*tg.UploadFile)
		if !ok {
			return nil, fmt.Errorf("unexpected file result type %T", res)
		}
		if len(file.Bytes) == 0 {
			break
		}
		buf.Write(file.Bytes)
		if progressFn != nil {
			progressFn(int64(buf.Len()), fileSize)
		}
		if len(file.Bytes) < partSize {
			break
		}
		offset += int64(len(file.Bytes))
	}
	return buf.Bytes(), nil
}

// DownloadProfilePhoto downloads the profile photo of a user or chat.
// entityID is the numeric peer ID (positive = user, negative = group/channel).
// The file is saved to filePath. Pass an empty filePath to get the
// destination chosen automatically (current directory, "profile_photo.jpg").
func (c *MCUBClient) DownloadProfilePhoto(ctx context.Context, entityID int64, filePath string) error {
	if filePath == "" {
		filePath = fmt.Sprintf("profile_photo_%d.jpg", entityID)
	}

	var location tg.InputFileLocationClass
	if entityID > 0 {
		// User: use photos.getUserPhotos to get the first photo
		result, err := c.client.API().PhotosGetUserPhotos(ctx, &tg.PhotosGetUserPhotosRequest{
			UserID: &tg.InputUser{UserID: entityID},
			Offset: 0,
			MaxID:  0,
			Limit:  1,
		})
		if err != nil {
			return fmt.Errorf("get user photos: %w", err)
		}

		var photos []tg.PhotoClass
		switch r := result.(type) {
		case *tg.PhotosPhotos:
			photos = r.Photos
		case *tg.PhotosPhotosSlice:
			photos = r.Photos
		}
		if len(photos) == 0 {
			return fmt.Errorf("user %d has no profile photo", entityID)
		}
		photo, ok := photos[0].(*tg.Photo)
		if !ok {
			return fmt.Errorf("profile photo is empty")
		}
		thumbType, _ := pickPhotoThumb(photo.Sizes, -1)
		location = &tg.InputPhotoFileLocation{
			ID:            photo.ID,
			AccessHash:    photo.AccessHash,
			FileReference: photo.FileReference,
			ThumbSize:     thumbType,
		}
	} else {
		// Chat / channel: use InputPeerPhotoFileLocation
		peer, err := c.resolvePeer(ctx, entityID)
		if err != nil {
			return fmt.Errorf("resolve peer: %w", err)
		}
		location = &tg.InputPeerPhotoFileLocation{
			Peer:    peer,
			PhotoID: 0,
			Big:     true,
		}
	}

	return c.DownloadFile(ctx, DownloadParams{
		Location: location,
		DestPath: filePath,
	})
}

// DownloadStickerSet downloads every sticker in the named sticker set.
// setName is the short name of the set (e.g. "Animals").
// Each sticker is saved as a file inside dir.
func (c *MCUBClient) DownloadStickerSet(ctx context.Context, setName string, dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create sticker dir: %w", err)
	}

	result, err := c.client.API().MessagesGetStickerSet(ctx, &tg.MessagesGetStickerSetRequest{
		Stickerset: &tg.InputStickerSetShortName{ShortName: setName},
		Hash:       0,
	})
	if err != nil {
		return fmt.Errorf("get sticker set %q: %w", setName, err)
	}

	set, ok := result.(*tg.MessagesStickerSet)
	if !ok {
		return fmt.Errorf("sticker set %q not found or not modified", setName)
	}

	for i, docClass := range set.Documents {
		doc, ok := docClass.(*tg.Document)
		if !ok {
			continue
		}

		ext := ".webp"
		for _, attr := range doc.Attributes {
			if fn, ok := attr.(*tg.DocumentAttributeFilename); ok {
				ext = filepath.Ext(fn.FileName)
				break
			}
		}

		destPath := filepath.Join(dir, fmt.Sprintf("sticker_%04d%s", i, ext))
		location := &tg.InputDocumentFileLocation{
			ID:            doc.ID,
			AccessHash:    doc.AccessHash,
			FileReference: doc.FileReference,
			ThumbSize:     "",
		}

		if err := c.DownloadFile(ctx, DownloadParams{
			Location: location,
			DestPath: destPath,
		}); err != nil {
			return fmt.Errorf("download sticker %d: %w", i, err)
		}
	}

	return nil
}

// GetDownloadURL returns a Telegram t.me deep link for the given message.
// This is a best-effort URL that points to the message in the Telegram app;
// it is not a direct HTTP download link (the Telegram API does not expose those).
func (c *MCUBClient) GetDownloadURL(ctx context.Context, msgID int, chatID int64) (string, error) {
	if chatID < -1_000_000_000_000 {
		// Supergroup / channel
		channelID := -(chatID + 1_000_000_000_000)
		return fmt.Sprintf("https://t.me/c/%d/%d", channelID, msgID), nil
	}
	if chatID < 0 {
		// Legacy group — no public link available
		return fmt.Sprintf("https://t.me/c/%d/%d", -chatID, msgID), nil
	}
	// Private chat — no public link
	return "", fmt.Errorf("cannot generate public URL for private chat peer %d", chatID)
}

// ResumableDownload supports resuming interrupted file downloads.
// Populate ChatID, MsgID, and FilePath then call Start.
type ResumableDownload struct {
	// Client is the MCUBClient to use for the download.
	Client *MCUBClient

	// ChatID is the numeric peer ID of the chat that contains the message.
	ChatID int64

	// MsgID is the ID of the message whose media to download.
	MsgID int

	// FilePath is where the downloaded file will be written.
	FilePath string

	// Resume controls whether to continue an interrupted download.
	// When true a state file is created next to FilePath.
	Resume bool

	// StateFile is the explicit path for the resume state JSON file.
	// Defaults to FilePath + ".download.state.json" when empty.
	StateFile string
}

// Start begins (or resumes) the download described by r.
func (r *ResumableDownload) Start(ctx context.Context) error {
	if r.Client == nil {
		return fmt.Errorf("ResumableDownload.Client must not be nil")
	}
	if r.FilePath == "" {
		return fmt.Errorf("ResumableDownload.FilePath must not be empty")
	}

	// Resolve the media location from the message.
	mediaParams := DownloadMediaParams{
		ChatID:    r.ChatID,
		MessageID: r.MsgID,
		FilePath:  r.FilePath,
		Thumb:     -1,
	}

	if !r.Resume {
		_, err := r.Client.DownloadMedia(ctx, mediaParams)
		return err
	}

	// Resumable path: fetch the message to get the file location, then
	// delegate to DownloadFile with a StateStore.
	stateFile := r.StateFile
	if stateFile == "" {
		stateFile = r.FilePath + ".download.state.json"
	}

	stateDir := filepath.Dir(stateFile)
	store, err := session.NewStateStore(stateDir)
	if err != nil {
		return fmt.Errorf("create state store: %w", err)
	}

	// Fetch message to obtain the file location.
	peer, err := r.Client.resolvePeer(ctx, r.ChatID)
	if err != nil {
		return fmt.Errorf("resolve peer: %w", err)
	}

	histResult, err := r.Client.client.API().MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{
		Peer:     peer,
		OffsetID: r.MsgID + 1,
		Limit:    1,
		MaxID:    r.MsgID,
	})
	if err != nil {
		return fmt.Errorf("get message: %w", err)
	}

	msgs := extractMessages(histResult)
	if len(msgs) == 0 || msgs[0].ID != r.MsgID {
		return fmt.Errorf("message %d not found", r.MsgID)
	}

	msg := msgs[0]
	var location tg.InputFileLocationClass

	switch media := msg.Media.(type) {
	case *tg.MessageMediaPhoto:
		photo, ok := media.Photo.(*tg.Photo)
		if !ok {
			return fmt.Errorf("photo is empty")
		}
		thumbType, _ := pickPhotoThumb(photo.Sizes, -1)
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
		location = &tg.InputDocumentFileLocation{
			ID:            doc.ID,
			AccessHash:    doc.AccessHash,
			FileReference: doc.FileReference,
			ThumbSize:     "",
		}
	default:
		return fmt.Errorf("message %d has no resumable media", r.MsgID)
	}

	return r.Client.DownloadFile(ctx, DownloadParams{
		Location:   location,
		DestPath:   r.FilePath,
		Resume:     true,
		ResumeKey:  fmt.Sprintf("dl_%d_%d", r.ChatID, r.MsgID),
		StateStore: store,
	})
}
