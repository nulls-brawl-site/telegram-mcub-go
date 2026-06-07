package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand"
	"mime"
	"os"
	"path/filepath"
	"sync"

	"github.com/gotd/td/tg"
	"github.com/nulls-brawl-site/telegram-mcub-go/session"
	"github.com/nulls-brawl-site/telegram-mcub-go/types"
)

// UploadParams holds parameters for uploading a file.
type UploadParams struct {
	// Path is the local path of the file to upload.
	Path string

	// Reader is used instead of Path when set. FileName must be provided.
	Reader io.Reader

	// FileName overrides the upload file name.
	FileName string

	// PartSize is the upload chunk size in bytes (default 512 KB).
	PartSize int

	// Resume enables resumable uploads.
	Resume bool

	// ResumeKey identifies the upload for resumption. Defaults to Path.
	ResumeKey string

	// StateStore persists upload progress.
	StateStore *session.StateStore

	// ProgressFunc is called after each part with (bytesDone, totalBytes).
	ProgressFunc func(bytesDone, totalBytes int64)
}

// uploadedFile holds the result of a completed upload.
type uploadedFile struct {
	InputFile tg.InputFileClass
	FileSize  int64
	FileName  string
}

// uploadFile uploads a file to Telegram and returns an InputFile reference.
func (c *MCUBClient) uploadFile(ctx context.Context, params UploadParams) (*uploadedFile, error) {
	partSize := params.PartSize
	if partSize <= 0 {
		partSize = 512 * 1024
	}

	var (
		r        io.Reader
		fileSize int64
		fileName string
	)

	if params.Reader != nil {
		r = params.Reader
		fileName = params.FileName
	} else {
		f, err := os.Open(params.Path)
		if err != nil {
			return nil, fmt.Errorf("open file: %w", err)
		}
		defer f.Close()

		info, err := f.Stat()
		if err != nil {
			return nil, fmt.Errorf("stat file: %w", err)
		}
		fileSize = info.Size()
		fileName = params.FileName
		if fileName == "" {
			fileName = filepath.Base(params.Path)
		}
		r = f
	}

	resumeKey := params.ResumeKey
	if resumeKey == "" {
		resumeKey = params.Path
	}

	fileID := rand.Int63()
	var partIndex int
	var startPart int

	// Resume: if previous state exists, skip already-uploaded parts.
	if params.Resume && params.StateStore != nil {
		state, err := params.StateStore.Load(resumeKey, "upload")
		if err == nil && state != nil && !state.Completed {
			startPart = state.PartsDone
			fileID = state.FileID
			if seeker, ok := r.(io.Seeker); ok {
				_, _ = seeker.Seek(int64(startPart)*int64(partSize), io.SeekStart)
			}
			partIndex = startPart
		}
	}

	buf := make([]byte, partSize)
	var totalUploaded int64 = int64(startPart) * int64(partSize)
	isBig := fileSize > 10*1024*1024

	for {
		n, err := io.ReadFull(r, buf)
		if n == 0 && err == io.EOF {
			break
		}
		if err != nil && err != io.ErrUnexpectedEOF {
			return nil, fmt.Errorf("read part %d: %w", partIndex, err)
		}

		chunk := buf[:n]

		if isBig {
			_, uploadErr := c.client.API().UploadSaveBigFilePart(ctx, &tg.UploadSaveBigFilePartRequest{
				FileID:         fileID,
				FilePart:       partIndex,
				FileTotalParts: -1,
				Bytes:          chunk,
			})
			if uploadErr != nil {
				return nil, fmt.Errorf("upload big part %d: %w", partIndex, uploadErr)
			}
		} else {
			_, uploadErr := c.client.API().UploadSaveFilePart(ctx, &tg.UploadSaveFilePartRequest{
				FileID:   fileID,
				FilePart: partIndex,
				Bytes:    chunk,
			})
			if uploadErr != nil {
				return nil, fmt.Errorf("upload part %d: %w", partIndex, uploadErr)
			}
		}

		totalUploaded += int64(n)
		partIndex++

		if params.ProgressFunc != nil {
			params.ProgressFunc(totalUploaded, fileSize)
		}

		if params.StateStore != nil {
			_ = params.StateStore.Save(&session.ResumeState{
				ID:        resumeKey,
				Kind:      "upload",
				BytesDone: totalUploaded,
				PartsDone: partIndex,
				FileID:    fileID,
				DestPath:  params.Path,
			})
		}

		if err == io.ErrUnexpectedEOF {
			break
		}
	}

	var inputFile tg.InputFileClass
	if isBig {
		inputFile = &tg.InputFileBig{
			ID:    fileID,
			Parts: partIndex,
			Name:  fileName,
		}
	} else {
		inputFile = &tg.InputFile{
			ID:    fileID,
			Parts: partIndex,
			Name:  fileName,
		}
	}

	if params.StateStore != nil {
		_ = params.StateStore.Save(&session.ResumeState{
			ID:        resumeKey,
			Kind:      "upload",
			BytesDone: totalUploaded,
			PartsDone: partIndex,
			Completed: true,
			DestPath:  params.Path,
		})
	}

	return &uploadedFile{
		InputFile: inputFile,
		FileSize:  fileSize,
		FileName:  fileName,
	}, nil
}

// SendFileParams holds parameters for sending a file to a chat.
type SendFileParams struct {
	// PeerID is the numeric peer ID.
	PeerID int64

	// Upload holds the file upload parameters.
	Upload UploadParams

	// Options contains send options.
	Options types.SendFileOptions
}

// SendFile uploads a local file and sends it to the given peer.
func (c *MCUBClient) SendFile(ctx context.Context, params SendFileParams) (*tg.Message, error) {
	uploaded, err := c.uploadFile(ctx, params.Upload)
	if err != nil {
		return nil, fmt.Errorf("upload file: %w", err)
	}

	peer, err := c.resolvePeer(ctx, params.PeerID)
	if err != nil {
		return nil, err
	}

	mimeType := params.Options.MimeType
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	media := &tg.InputMediaUploadedDocument{
		File:     uploaded.InputFile,
		MimeType: mimeType,
		Attributes: []tg.DocumentAttributeClass{
			&tg.DocumentAttributeFilename{FileName: uploaded.FileName},
		},
	}

	req := &tg.MessagesSendMediaRequest{
		Peer:     peer,
		Media:    media,
		Message:  params.Options.Caption,
		RandomID: rand.Int63(),
		Silent:   params.Options.Silent,
	}

	if params.Options.ReplyToMsgID != 0 {
		req.ReplyTo = &tg.InputReplyToMessage{
			ReplyToMsgID: params.Options.ReplyToMsgID,
		}
	}

	if params.Options.ForumTopicID != 0 {
		req.ReplyTo = &tg.InputReplyToMessage{
			ReplyToMsgID: params.Options.ForumTopicID,
		}
	}

	if params.Options.Buttons != nil {
		req.ReplyMarkup = params.Options.Buttons.ToTLMarkup()
	}

	result, err := c.client.API().MessagesSendMedia(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("send media: %w", err)
	}

	return extractMessageFromUpdates(result), nil
}

// SendFileToTopicParams holds parameters for sending a file to a forum topic.
type SendFileToTopicParams struct {
	// ChannelID is the supergroup's channel ID.
	ChannelID int64

	// TopicID is the forum topic thread ID.
	TopicID int

	// Upload contains the upload parameters.
	Upload UploadParams

	// Options contains send options.
	Options types.SendFileOptions
}

// SendFileToTopic uploads a file and sends it to a specific forum topic.
func (c *MCUBClient) SendFileToTopic(ctx context.Context, params SendFileToTopicParams) (*tg.Message, error) {
	return c.SendFile(ctx, SendFileParams{
		PeerID: -1000000000000 - params.ChannelID,
		Upload: params.Upload,
		Options: types.SendFileOptions{
			Caption:      params.Options.Caption,
			ParseMode:    params.Options.ParseMode,
			Buttons:      params.Options.Buttons,
			ForumTopicID: params.TopicID,
			Silent:       params.Options.Silent,
			MimeType:     params.Options.MimeType,
		},
	})
}

// UploadFile uploads a local file to Telegram and returns an InputFile handle
// that can later be used with SendFileAdvanced or other send methods.
func (c *MCUBClient) UploadFile(ctx context.Context, filePath string) (tg.InputFileClass, error) {
	res, err := c.uploadFile(ctx, UploadParams{Path: filePath})
	if err != nil {
		return nil, err
	}
	return res.InputFile, nil
}

// UploadBytes uploads an in-memory byte slice as a file to Telegram and returns
// an InputFile handle. fileName sets the display name of the uploaded file.
func (c *MCUBClient) UploadBytes(ctx context.Context, data []byte, fileName string) (tg.InputFileClass, error) {
	res, err := c.uploadFile(ctx, UploadParams{
		Reader:   bytes.NewReader(data),
		FileName: fileName,
	})
	if err != nil {
		return nil, err
	}
	return res.InputFile, nil
}

// SendFileAdvancedParams holds the extended parameters for SendFileAdvanced.
type SendFileAdvancedParams struct {
	// PeerID is the numeric peer ID of the recipient.
	PeerID int64

	// FilePath is the local path of the file to upload and send.
	FilePath string

	// Caption is the message caption (text shown below the media).
	Caption string

	// ParseMode is the text formatting mode: "html", "md", or "" (plain).
	ParseMode string

	// AsDocument forces the file to be sent as a generic document.
	AsDocument bool

	// AsPhoto forces the file to be sent as a photo.
	AsPhoto bool

	// ForceDocument is an alias for AsDocument.
	ForceDocument bool

	// Silent suppresses notifications for this message.
	Silent bool

	// ReplyToID is the ID of the message to reply to (0 = no reply).
	ReplyToID int

	// ScheduleDate is a Unix timestamp to schedule the message (0 = send now).
	ScheduleDate int

	// TTL is the self-destruct timer in seconds (0 = no TTL).
	TTL int

	// ProgressFn is called with (bytesSent, totalBytes) during upload.
	ProgressFn func(sent, total int64)

	// Spoiler marks the media as a spoiler.
	Spoiler bool

	// Buttons is the inline keyboard markup to attach to the message.
	Buttons interface{ ToTLMarkup() *tg.ReplyInlineMarkup }
}

// SendFileAdvanced uploads a file and sends it with extended options.
// It chooses the media type (photo vs document) based on the params.
func (c *MCUBClient) SendFileAdvanced(ctx context.Context, params SendFileAdvancedParams) (*tg.Message, error) {
	peer, err := c.resolvePeer(ctx, params.PeerID)
	if err != nil {
		return nil, fmt.Errorf("resolve peer: %w", err)
	}

	uploaded, err := c.uploadFile(ctx, UploadParams{
		Path:         params.FilePath,
		ProgressFunc: params.ProgressFn,
	})
	if err != nil {
		return nil, fmt.Errorf("upload file: %w", err)
	}

	mimeType := mime.TypeByExtension(filepath.Ext(params.FilePath))
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	var media tg.InputMediaClass
	usePhoto := params.AsPhoto && !params.AsDocument && !params.ForceDocument && isImageMIME(mimeType)

	if usePhoto {
		photo := &tg.InputMediaUploadedPhoto{
			File:       uploaded.InputFile,
			Spoiler:    params.Spoiler,
		}
		if params.TTL > 0 {
			photo.SetTTLSeconds(params.TTL)
		}
		media = photo
	} else {
		doc := &tg.InputMediaUploadedDocument{
			File:     uploaded.InputFile,
			MimeType: mimeType,
			Spoiler:  params.Spoiler,
			Attributes: []tg.DocumentAttributeClass{
				&tg.DocumentAttributeFilename{FileName: uploaded.FileName},
			},
		}
		if params.TTL > 0 {
			doc.SetTTLSeconds(params.TTL)
		}
		media = doc
	}

	req := &tg.MessagesSendMediaRequest{
		Peer:     peer,
		Media:    media,
		Message:  params.Caption,
		RandomID: rand.Int63(),
		Silent:   params.Silent,
	}

	if params.ReplyToID != 0 {
		req.ReplyTo = &tg.InputReplyToMessage{ReplyToMsgID: params.ReplyToID}
	}
	if params.ScheduleDate != 0 {
		req.SetScheduleDate(params.ScheduleDate)
	}
	if params.Buttons != nil {
		req.ReplyMarkup = params.Buttons.ToTLMarkup()
	}

	result, err := c.client.API().MessagesSendMedia(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("send media: %w", err)
	}
	return extractMessageFromUpdates(result), nil
}

// isImageMIME reports whether the MIME type represents an image.
func isImageMIME(m string) bool {
	return len(m) >= 6 && m[:6] == "image/"
}

// SendPhoto sends a local file as a photo (thumbnail-rendered media).
func (c *MCUBClient) SendPhoto(ctx context.Context, peerID int64, filePath, caption string) (*tg.Message, error) {
	return c.SendFileAdvanced(ctx, SendFileAdvancedParams{
		PeerID:   peerID,
		FilePath: filePath,
		Caption:  caption,
		AsPhoto:  true,
	})
}

// SendDocument sends a local file as a generic document.
func (c *MCUBClient) SendDocument(ctx context.Context, peerID int64, filePath, caption string) (*tg.Message, error) {
	return c.SendFileAdvanced(ctx, SendFileAdvancedParams{
		PeerID:     peerID,
		FilePath:   filePath,
		Caption:    caption,
		AsDocument: true,
	})
}

// SendAudio sends a local audio file. The MIME type is inferred from the
// file extension; ogg/opus files are recognised as voice messages by clients.
func (c *MCUBClient) SendAudio(ctx context.Context, peerID int64, filePath, caption string) (*tg.Message, error) {
	peer, err := c.resolvePeer(ctx, peerID)
	if err != nil {
		return nil, fmt.Errorf("resolve peer: %w", err)
	}

	uploaded, err := c.uploadFile(ctx, UploadParams{Path: filePath})
	if err != nil {
		return nil, fmt.Errorf("upload audio: %w", err)
	}

	mimeType := mime.TypeByExtension(filepath.Ext(filePath))
	if mimeType == "" {
		mimeType = "audio/mpeg"
	}

	media := &tg.InputMediaUploadedDocument{
		File:     uploaded.InputFile,
		MimeType: mimeType,
		Attributes: []tg.DocumentAttributeClass{
			&tg.DocumentAttributeAudio{},
			&tg.DocumentAttributeFilename{FileName: uploaded.FileName},
		},
	}

	result, err := c.client.API().MessagesSendMedia(ctx, &tg.MessagesSendMediaRequest{
		Peer:     peer,
		Media:    media,
		Message:  caption,
		RandomID: rand.Int63(),
	})
	if err != nil {
		return nil, fmt.Errorf("send audio: %w", err)
	}
	return extractMessageFromUpdates(result), nil
}

// SendVideo sends a local video file to a chat.
func (c *MCUBClient) SendVideo(ctx context.Context, peerID int64, filePath, caption string) (*tg.Message, error) {
	peer, err := c.resolvePeer(ctx, peerID)
	if err != nil {
		return nil, fmt.Errorf("resolve peer: %w", err)
	}

	uploaded, err := c.uploadFile(ctx, UploadParams{Path: filePath})
	if err != nil {
		return nil, fmt.Errorf("upload video: %w", err)
	}

	mimeType := mime.TypeByExtension(filepath.Ext(filePath))
	if mimeType == "" {
		mimeType = "video/mp4"
	}

	media := &tg.InputMediaUploadedDocument{
		File:     uploaded.InputFile,
		MimeType: mimeType,
		Attributes: []tg.DocumentAttributeClass{
			&tg.DocumentAttributeVideo{},
			&tg.DocumentAttributeFilename{FileName: uploaded.FileName},
		},
	}

	result, err := c.client.API().MessagesSendMedia(ctx, &tg.MessagesSendMediaRequest{
		Peer:     peer,
		Media:    media,
		Message:  caption,
		RandomID: rand.Int63(),
	})
	if err != nil {
		return nil, fmt.Errorf("send video: %w", err)
	}
	return extractMessageFromUpdates(result), nil
}

// UploadMultiple uploads multiple files in parallel and returns their InputFile
// handles in the same order as the input paths.
// Errors from individual uploads are accumulated; any single failure returns
// the first encountered error after all goroutines have finished.
func (c *MCUBClient) UploadMultiple(ctx context.Context, paths []string) ([]tg.InputFileClass, error) {
	results := make([]tg.InputFileClass, len(paths))
	errs := make([]error, len(paths))

	var wg sync.WaitGroup
	for i, p := range paths {
		wg.Add(1)
		go func(idx int, path string) {
			defer wg.Done()
			f, err := c.UploadFile(ctx, path)
			results[idx] = f
			errs[idx] = err
		}(i, p)
	}
	wg.Wait()

	for _, e := range errs {
		if e != nil {
			return results, e
		}
	}
	return results, nil
}

// NewStateStore is a convenience alias that creates a session.StateStore in dir.
func NewStateStore(dir string) (*session.StateStore, error) {
	return session.NewStateStore(dir)
}
