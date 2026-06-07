package client

import (
	"context"
	"fmt"
	"math/rand"

	"github.com/gotd/td/tg"
	"github.com/nulls-brawl-site/telegram-mcub-go/types"
)

// SendMessageParams holds all parameters for sending a message.
type SendMessageParams struct {
	// PeerID is the numeric peer ID to send to.
	// Positive = user, negative starting with -100 = channel.
	PeerID int64

	// Text is the message body.
	Text string

	// Options contains optional send parameters.
	Options types.SendMessageOptions
}

// SendMessage sends a text message to the given peer.
func (c *MCUBClient) SendMessage(ctx context.Context, params SendMessageParams) (*tg.Message, error) {
	peer, err := c.resolvePeer(ctx, params.PeerID)
	if err != nil {
		return nil, fmt.Errorf("resolve peer: %w", err)
	}

	req := &tg.MessagesSendMessageRequest{
		Peer:       peer,
		Message:    params.Text,
		RandomID:   rand.Int63(),
		Silent:     params.Options.Silent,
		ClearDraft: params.Options.ClearDraft,
		NoWebpage:  params.Options.NoWebpage,
	}

	if params.Options.ReplyToMsgID != 0 {
		req.ReplyTo = &tg.InputReplyToMessage{
			ReplyToMsgID: params.Options.ReplyToMsgID,
		}
	}

	// Forum topic: reply-to thread root.
	if params.Options.ForumTopicID != 0 {
		req.ReplyTo = &tg.InputReplyToMessage{
			ReplyToMsgID: params.Options.ForumTopicID,
		}
	}

	if params.Options.ScheduleDate != 0 {
		req.SetScheduleDate(params.Options.ScheduleDate)
	}

	if params.Options.Buttons != nil {
		req.ReplyMarkup = params.Options.Buttons.ToTLMarkup()
	}

	result, err := c.client.API().MessagesSendMessage(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("send message: %w", err)
	}

	return extractMessageFromUpdates(result), nil
}

// EditMessageParams holds all parameters for editing a message.
type EditMessageParams struct {
	// PeerID is the numeric peer ID.
	PeerID int64

	// MessageID is the ID of the message to edit.
	MessageID int

	// Text is the new message body.
	Text string

	// Options contains optional parameters.
	Options types.SendMessageOptions
}

// EditMessage edits an existing message.
func (c *MCUBClient) EditMessage(ctx context.Context, params EditMessageParams) (*tg.Message, error) {
	peer, err := c.resolvePeer(ctx, params.PeerID)
	if err != nil {
		return nil, fmt.Errorf("resolve peer: %w", err)
	}

	req := &tg.MessagesEditMessageRequest{
		Peer:      peer,
		ID:        params.MessageID,
		Message:   params.Text,
		NoWebpage: params.Options.NoWebpage,
	}

	if params.Options.Buttons != nil {
		req.ReplyMarkup = params.Options.Buttons.ToTLMarkup()
	}

	result, err := c.client.API().MessagesEditMessage(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("edit message: %w", err)
	}

	return extractMessageFromUpdates(result), nil
}

// DeleteMessageParams holds parameters for deleting messages.
type DeleteMessageParams struct {
	// PeerID is the numeric peer ID (required for channel messages).
	PeerID int64

	// MessageIDs are the IDs of the messages to delete.
	MessageIDs []int

	// Revoke deletes the message for all participants, not just the sender.
	Revoke bool
}

// DeleteMessage deletes one or more messages.
func (c *MCUBClient) DeleteMessage(ctx context.Context, params DeleteMessageParams) error {
	if len(params.MessageIDs) == 0 {
		return nil
	}

	if params.PeerID < -999999999 {
		// Channel message.
		chanID := channelIDFromPeerID(params.PeerID)
		_, err := c.client.API().ChannelsDeleteMessages(ctx, &tg.ChannelsDeleteMessagesRequest{
			Channel: &tg.InputChannel{ChannelID: chanID},
			ID:      params.MessageIDs,
		})
		return err
	}

	_, err := c.client.API().MessagesDeleteMessages(ctx, &tg.MessagesDeleteMessagesRequest{
		Revoke: params.Revoke,
		ID:     params.MessageIDs,
	})
	return err
}

// GetMessages fetches messages by their IDs from a given peer.
func (c *MCUBClient) GetMessages(ctx context.Context, peerID int64, msgIDs []int) ([]*tg.Message, error) {
	_, err := c.resolvePeer(ctx, peerID)
	if err != nil {
		return nil, err
	}

	ids := make([]tg.InputMessageClass, 0, len(msgIDs))
	for _, id := range msgIDs {
		ids = append(ids, &tg.InputMessageID{ID: id})
	}

	result, err := c.client.API().MessagesGetMessages(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("get messages: %w", err)
	}

	return extractMessages(result), nil
}

// ForwardMessage forwards a message from one chat to another.
func (c *MCUBClient) ForwardMessage(ctx context.Context, fromPeerID, toPeerID int64, msgID int) error {
	fromPeer, err := c.resolvePeer(ctx, fromPeerID)
	if err != nil {
		return err
	}
	toPeer, err := c.resolvePeer(ctx, toPeerID)
	if err != nil {
		return err
	}

	_, err = c.client.API().MessagesForwardMessages(ctx, &tg.MessagesForwardMessagesRequest{
		FromPeer: fromPeer,
		ToPeer:   toPeer,
		ID:       []int{msgID},
		RandomID: []int64{rand.Int63()},
	})
	return err
}

// --- helpers -----------------------------------------------------------------

// resolvePeer converts a numeric peer ID to a tg.InputPeerClass.
func (c *MCUBClient) resolvePeer(ctx context.Context, peerID int64) (tg.InputPeerClass, error) {
	_ = ctx
	if peerID > 0 {
		return &tg.InputPeerUser{UserID: peerID}, nil
	}
	if peerID < -999999999 {
		chanID := channelIDFromPeerID(peerID)
		return &tg.InputPeerChannel{ChannelID: chanID}, nil
	}
	return &tg.InputPeerChat{ChatID: -peerID}, nil
}

// channelIDFromPeerID extracts the raw channel ID from a packed peer ID.
func channelIDFromPeerID(peerID int64) int64 {
	return -peerID - 1000000000000
}

// extractMessageFromUpdates pulls the first *tg.Message out of an Updates object.
func extractMessageFromUpdates(upd tg.UpdatesClass) *tg.Message {
	switch u := upd.(type) {
	case *tg.Updates:
		for _, update := range u.Updates {
			switch v := update.(type) {
			case *tg.UpdateNewMessage:
				if m, ok := v.Message.(*tg.Message); ok {
					return m
				}
			case *tg.UpdateNewChannelMessage:
				if m, ok := v.Message.(*tg.Message); ok {
					return m
				}
			case *tg.UpdateEditMessage:
				if m, ok := v.Message.(*tg.Message); ok {
					return m
				}
			case *tg.UpdateEditChannelMessage:
				if m, ok := v.Message.(*tg.Message); ok {
					return m
				}
			}
		}
	case *tg.UpdateShortMessage:
		return &tg.Message{
			ID:      u.ID,
			Message: u.Message,
			Date:    u.Date,
		}
	}
	return nil
}

// extractMessages pulls *tg.Message values from a MessagesMessages result.
func extractMessages(result tg.MessagesMessagesClass) []*tg.Message {
	var msgs []tg.MessageClass
	switch r := result.(type) {
	case *tg.MessagesMessages:
		msgs = r.Messages
	case *tg.MessagesMessagesSlice:
		msgs = r.Messages
	case *tg.MessagesChannelMessages:
		msgs = r.Messages
	}
	out := make([]*tg.Message, 0, len(msgs))
	for _, m := range msgs {
		if msg, ok := m.(*tg.Message); ok {
			out = append(out, msg)
		}
	}
	return out
}
