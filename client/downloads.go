package client

import (
	"context"
	"fmt"
	"io"
	"os"

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
