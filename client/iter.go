package client

import (
	"context"
	"fmt"

	"github.com/gotd/td/tg"
)

// IterMessagesParams holds parameters for IterMessages.
type IterMessagesParams struct {
	// PeerID is the numeric peer ID of the chat to iterate.
	PeerID int64

	// Limit is the total maximum number of messages to return (0 = unlimited).
	Limit int

	// OffsetID starts iteration from this message ID.
	OffsetID int

	// MaxID returns only messages with ID <= MaxID (0 = no upper bound).
	MaxID int

	// MinID returns only messages with ID >= MinID (0 = no lower bound).
	MinID int

	// Search filters messages by text search query.
	Search string

	// FromUserID filters messages sent by this user ID (0 = all senders).
	FromUserID int64

	// Reverse iterates from oldest to newest when true.
	Reverse bool

	// BatchSize controls the number of messages fetched per API call (default 100).
	BatchSize int
}

// MessageIterator iterates over messages in a chat, fetching them in batches.
type MessageIterator struct {
	client *MCUBClient
	params IterMessagesParams

	buf      []*tg.Message
	bufPos   int
	done     bool
	total    int
	fetched  int
	offsetID int
}

// IterMessages constructs a MessageIterator for the given peer.
func (c *MCUBClient) IterMessages(ctx context.Context, params IterMessagesParams) *MessageIterator {
	_ = ctx
	batchSize := params.BatchSize
	if batchSize <= 0 {
		batchSize = 100
	}
	params.BatchSize = batchSize

	return &MessageIterator{
		client:   c,
		params:   params,
		offsetID: params.OffsetID,
	}
}

// Next advances the iterator and returns the next message.
// Returns (msg, true, nil) while messages remain, (nil, false, nil) when exhausted,
// and (nil, false, err) on error.
func (it *MessageIterator) Next(ctx context.Context) (*tg.Message, bool, error) {
	if it.done {
		return nil, false, nil
	}

	// Serve from buffer first.
	if it.bufPos < len(it.buf) {
		msg := it.buf[it.bufPos]
		it.bufPos++
		it.fetched++
		if it.params.Limit > 0 && it.fetched >= it.params.Limit {
			it.done = true
		}
		return msg, true, nil
	}

	// Check if we've already fetched enough.
	if it.params.Limit > 0 && it.fetched >= it.params.Limit {
		it.done = true
		return nil, false, nil
	}

	// Fetch the next batch.
	batch, err := it.fetchBatch(ctx)
	if err != nil {
		return nil, false, err
	}
	if len(batch) == 0 {
		it.done = true
		return nil, false, nil
	}

	it.buf = batch
	it.bufPos = 0
	return it.Next(ctx)
}

// Collect fetches up to limit messages into a slice.
// If limit <= 0, it collects all messages (respecting IterMessagesParams.Limit).
func (it *MessageIterator) Collect(ctx context.Context, limit int) ([]*tg.Message, error) {
	var out []*tg.Message
	for {
		msg, ok, err := it.Next(ctx)
		if err != nil {
			return out, err
		}
		if !ok {
			break
		}
		out = append(out, msg)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

// fetchBatch performs one API call to get the next batch of messages.
func (it *MessageIterator) fetchBatch(ctx context.Context) ([]*tg.Message, error) {
	peer, err := it.client.resolvePeer(ctx, it.params.PeerID)
	if err != nil {
		return nil, fmt.Errorf("resolve peer: %w", err)
	}

	batchSize := it.params.BatchSize
	remaining := it.params.Limit - it.fetched
	if it.params.Limit > 0 && remaining < batchSize {
		batchSize = remaining
	}

	req := &tg.MessagesGetHistoryRequest{
		Peer:     peer,
		Limit:    batchSize,
		OffsetID: it.offsetID,
		MaxID:    it.params.MaxID,
		MinID:    it.params.MinID,
		AddOffset: 0,
	}

	// For reverse iteration, use add_offset trick.
	if it.params.Reverse {
		req.AddOffset = -batchSize
	}

	var result tg.MessagesMessagesClass

	if it.params.Search != "" || it.params.FromUserID != 0 {
		searchReq := &tg.MessagesSearchRequest{
			Peer:     peer,
			Q:        it.params.Search,
			Filter:   &tg.InputMessagesFilterEmpty{},
			Limit:    batchSize,
			OffsetID: it.offsetID,
			MaxID:    it.params.MaxID,
			MinID:    it.params.MinID,
		}
		if it.params.FromUserID != 0 {
			searchReq.FromID = &tg.InputPeerUser{UserID: it.params.FromUserID}
		}
		result, err = it.client.api.MessagesSearch(ctx, searchReq)
	} else {
		result, err = it.client.api.MessagesGetHistory(ctx, req)
	}

	if err != nil {
		return nil, fmt.Errorf("fetch messages: %w", err)
	}

	msgs := extractMessages(result)
	if len(msgs) == 0 {
		return nil, nil
	}

	if it.params.Reverse {
		for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
			msgs[i], msgs[j] = msgs[j], msgs[i]
		}
		it.offsetID = msgs[len(msgs)-1].ID + 1
	} else {
		it.offsetID = msgs[len(msgs)-1].ID
	}

	return msgs, nil
}
