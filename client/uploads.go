package client

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"

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
