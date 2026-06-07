package client

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gotd/td/tg"
)

// HistoryBatch is a page of messages returned by StreamHistoryBatches.
type HistoryBatch struct {
	// Messages is the list of messages in this batch.
	Messages []*tg.Message

	// BatchNum is the zero-based index of this batch.
	BatchNum int

	// Total is an estimate of the total message count (-1 when unknown).
	Total int
}

// ExportHistoryResult is an alias of HistoryExportResult provided for
// compatibility with the history.py port naming convention.
type ExportHistoryResult = HistoryExportResult

// ExportHistoryJSONLParams holds parameters for ExportHistoryJSONL.
type ExportHistoryJSONLParams struct {
	// PeerID is the numeric peer ID of the chat to export.
	PeerID int64

	// Output is the path of the JSONL file to write.
	Output string

	// Reverse iterates messages from oldest to newest when true.
	// When false (default) messages are fetched newest-first and written
	// in that order.
	Reverse bool

	// DownloadMedia controls whether attached media files are also downloaded.
	DownloadMedia bool

	// MediaDir is the directory where media files are saved.
	// Defaults to Output + "_media" when empty and DownloadMedia is true.
	MediaDir string

	// StateFile is the path of the resume-state JSON file.
	// Set to enable resuming an interrupted export.
	// Defaults to Output + ".state.json" when empty (only used when non-empty).
	StateFile string

	// Limit caps the total number of messages exported (0 = all).
	Limit int

	// BatchSize controls how many messages are fetched per API request.
	// Defaults to 100.
	BatchSize int
}

// exportJSONLState is the on-disk state saved between resume runs.
type exportJSONLState struct {
	LastMsgID       int   `json:"last_msg_id"`
	ExportedTotal   int   `json:"exported_total"`
	MediaTotal      int   `json:"media_total"`
	UpdatedAt       int64 `json:"updated_at"`
}

// ExportHistoryJSONL exports chat history to a JSONL file.
// Each line contains a JSON object representing one message.
// When params.StateFile is set the export can be resumed after interruption.
func (c *MCUBClient) ExportHistoryJSONL(ctx context.Context, params ExportHistoryJSONLParams) (*ExportHistoryResult, error) {
	if params.Output == "" {
		return nil, fmt.Errorf("ExportHistoryJSONLParams.Output must not be empty")
	}
	if err := os.MkdirAll(filepath.Dir(params.Output), 0o755); err != nil {
		return nil, fmt.Errorf("create output dir: %w", err)
	}

	batchSize := params.BatchSize
	if batchSize <= 0 {
		batchSize = 100
	}

	mediaDir := params.MediaDir
	if params.DownloadMedia && mediaDir == "" {
		base := params.Output
		if ext := filepath.Ext(base); ext != "" {
			base = base[:len(base)-len(ext)]
		}
		mediaDir = base + "_media"
	}
	if mediaDir != "" {
		if err := os.MkdirAll(mediaDir, 0o755); err != nil {
			return nil, fmt.Errorf("create media dir: %w", err)
		}
	}

	// Resume state.
	var (
		resumedFrom   int
		exportedTotal int
		mediaTotal    int
		minID         int
	)

	if params.StateFile != "" {
		raw, err := os.ReadFile(params.StateFile)
		if err == nil {
			var st exportJSONLState
			if jsonErr := json.Unmarshal(raw, &st); jsonErr == nil {
				resumedFrom = st.LastMsgID
				exportedTotal = st.ExportedTotal
				mediaTotal = st.MediaTotal
				minID = st.LastMsgID
			}
		}
	}

	// Open (or append to) the output file.
	fileFlags := os.O_CREATE | os.O_WRONLY
	if resumedFrom > 0 {
		fileFlags |= os.O_APPEND
	} else {
		fileFlags |= os.O_TRUNC
	}
	outFile, err := os.OpenFile(params.Output, fileFlags, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open output file: %w", err)
	}
	defer outFile.Close()
	writer := bufio.NewWriter(outFile)

	peer, err := c.resolvePeer(ctx, params.PeerID)
	if err != nil {
		return nil, fmt.Errorf("resolve peer: %w", err)
	}

	exported := 0
	media := 0
	lastMsgID := resumedFrom
	offsetID := 0
	if resumedFrom > 0 {
		offsetID = resumedFrom + 1
	}

	for {
		req := &tg.MessagesGetHistoryRequest{
			Peer:     peer,
			OffsetID: offsetID,
			Limit:    batchSize,
			MinID:    minID,
		}
		if params.Reverse {
			req.AddOffset = -batchSize
		}

		result, err := c.client.API().MessagesGetHistory(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("get history: %w", err)
		}

		batch := extractMessages(result)
		if len(batch) == 0 {
			break
		}

		// Reverse the batch when iterating oldest-first.
		if params.Reverse {
			for i, j := 0, len(batch)-1; i < j; i, j = i+1, j-1 {
				batch[i], batch[j] = batch[j], batch[i]
			}
		}

		for _, msg := range batch {
			// Serialise message to JSON.
			line, jsonErr := json.Marshal(messageToMap(msg))
			if jsonErr != nil {
				continue
			}
			writer.Write(line)
			writer.WriteByte('\n')

			lastMsgID = msg.ID
			exported++

			// Optionally download media.
			if params.DownloadMedia && mediaDir != "" && msg.Media != nil {
				destPath := filepath.Join(mediaDir, fmt.Sprintf("media_%d", msg.ID))
				if _, dlErr := c.DownloadMedia(ctx, DownloadMediaParams{
					ChatID:    params.PeerID,
					MessageID: msg.ID,
					FilePath:  destPath,
					Thumb:     -1,
				}); dlErr == nil {
					media++
				}
			}

			if params.Limit > 0 && (exported+exportedTotal) >= params.Limit {
				goto done
			}
		}

		if err := writer.Flush(); err != nil {
			return nil, fmt.Errorf("flush output: %w", err)
		}

		// Persist state.
		if params.StateFile != "" {
			saveExportJSONLState(params.StateFile, exportJSONLState{
				LastMsgID:     lastMsgID,
				ExportedTotal: exportedTotal + exported,
				MediaTotal:    mediaTotal + media,
				UpdatedAt:     time.Now().Unix(),
			})
		}

		if params.Reverse {
			offsetID = batch[len(batch)-1].ID
		} else {
			offsetID = batch[len(batch)-1].ID
		}

		if len(batch) < batchSize {
			break
		}
	}

done:
	if flushErr := writer.Flush(); flushErr != nil {
		return nil, fmt.Errorf("flush final output: %w", flushErr)
	}

	// Remove state file on successful completion.
	if params.StateFile != "" {
		_ = os.Remove(params.StateFile)
	}

	return &ExportHistoryResult{
		PeerID:        params.PeerID,
		TotalMessages: exported,
		OutputPath:    params.Output,
	}, nil
}

// saveExportJSONLState atomically writes the export state JSON file.
func saveExportJSONLState(path string, st exportJSONLState) {
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return
	}
	_ = os.Rename(tmp, path)
}

// messageToMap converts a *tg.Message to a plain map suitable for JSON serialisation.
func messageToMap(m *tg.Message) map[string]interface{} {
	out := map[string]interface{}{
		"id":      m.ID,
		"date":    m.Date,
		"message": m.Message,
		"out":     m.Out,
		"mentioned": m.Mentioned,
		"silent":  m.Silent,
	}
	if m.FromID != nil {
		out["from_id"] = fmt.Sprintf("%T:%v", m.FromID, m.FromID)
	}
	if m.PeerID != nil {
		out["peer_id"] = fmt.Sprintf("%T:%v", m.PeerID, m.PeerID)
	}
	if m.Media != nil {
		out["media_type"] = fmt.Sprintf("%T", m.Media)
	}
	if m.ReplyTo != nil {
		out["reply_to"] = fmt.Sprintf("%T", m.ReplyTo)
	}
	if _, hasFwd := m.GetFwdFrom(); hasFwd {
		out["fwd_from"] = true
	}
	return out
}

// StreamHistoryBatches returns two channels: one that emits HistoryBatch
// values (oldest-first when batchSize > 0) and one that receives a single
// non-nil error value if the iteration fails.
//
// The batch channel is closed after all messages have been sent or an error
// occurs. The caller must drain both channels to avoid goroutine leaks.
func (c *MCUBClient) StreamHistoryBatches(ctx context.Context, peerID int64, batchSize int) (<-chan HistoryBatch, <-chan error) {
	batchCh := make(chan HistoryBatch, 4)
	errCh := make(chan error, 1)

	if batchSize <= 0 {
		batchSize = 100
	}

	go func() {
		defer close(batchCh)
		defer close(errCh)

		peer, err := c.resolvePeer(ctx, peerID)
		if err != nil {
			errCh <- fmt.Errorf("resolve peer: %w", err)
			return
		}

		offsetID := 0
		batchNum := 0
		total := -1

		for {
			result, err := c.client.API().MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{
				Peer:     peer,
				OffsetID: offsetID,
				Limit:    batchSize,
			})
			if err != nil {
				errCh <- fmt.Errorf("get history (batch %d): %w", batchNum, err)
				return
			}

			// Extract total count when available.
			switch r := result.(type) {
			case *tg.MessagesMessagesSlice:
				total = r.Count
			}

			batch := extractMessages(result)
			if len(batch) == 0 {
				return
			}

			select {
			case batchCh <- HistoryBatch{Messages: batch, BatchNum: batchNum, Total: total}:
			case <-ctx.Done():
				errCh <- ctx.Err()
				return
			}

			batchNum++
			offsetID = batch[len(batch)-1].ID

			if len(batch) < batchSize {
				return
			}
		}
	}()

	return batchCh, errCh
}
