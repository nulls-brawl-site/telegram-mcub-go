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

// ForwardParams holds parameters for ForwardMessages.
type ForwardParams struct {
	// FromPeerID is the source chat.
	FromPeerID int64

	// ToPeerID is the destination chat.
	ToPeerID int64

	// MessageIDs are the IDs of the messages to forward.
	MessageIDs []int

	// Silent suppresses notifications in the destination chat.
	Silent bool

	// DropAuthor hides the original sender's name.
	DropAuthor bool
}

// ForwardMessages forwards one or more messages from one chat to another.
func (c *MCUBClient) ForwardMessages(ctx context.Context, params ForwardParams) ([]*tg.Message, error) {
	fromPeer, err := c.resolvePeer(ctx, params.FromPeerID)
	if err != nil {
		return nil, fmt.Errorf("resolve from peer: %w", err)
	}
	toPeer, err := c.resolvePeer(ctx, params.ToPeerID)
	if err != nil {
		return nil, fmt.Errorf("resolve to peer: %w", err)
	}

	randomIDs := make([]int64, len(params.MessageIDs))
	for i := range randomIDs {
		randomIDs[i] = rand.Int63()
	}

	result, err := c.client.API().MessagesForwardMessages(ctx, &tg.MessagesForwardMessagesRequest{
		FromPeer:    fromPeer,
		ToPeer:      toPeer,
		ID:          params.MessageIDs,
		RandomID:    randomIDs,
		Silent:      params.Silent,
		DropAuthor:  params.DropAuthor,
	})
	if err != nil {
		return nil, fmt.Errorf("forward messages: %w", err)
	}

	// Collect forwarded messages from the updates.
	var msgs []*tg.Message
	switch u := result.(type) {
	case *tg.Updates:
		for _, upd := range u.Updates {
			switch v := upd.(type) {
			case *tg.UpdateNewMessage:
				if m, ok := v.Message.(*tg.Message); ok {
					msgs = append(msgs, m)
				}
			case *tg.UpdateNewChannelMessage:
				if m, ok := v.Message.(*tg.Message); ok {
					msgs = append(msgs, m)
				}
			}
		}
	}
	return msgs, nil
}

// PinMessage pins a message in a chat.
// If notify is true, participants receive a notification.
func (c *MCUBClient) PinMessage(ctx context.Context, peerID int64, msgID int, notify bool) error {
	peer, err := c.resolvePeer(ctx, peerID)
	if err != nil {
		return fmt.Errorf("resolve peer: %w", err)
	}

	_, err = c.client.API().MessagesUpdatePinnedMessage(ctx, &tg.MessagesUpdatePinnedMessageRequest{
		Peer:   peer,
		ID:     msgID,
		Silent: !notify,
		Unpin:  false,
	})
	if err != nil {
		return fmt.Errorf("pin message: %w", err)
	}
	return nil
}

// UnpinMessage unpins a specific message in a chat.
func (c *MCUBClient) UnpinMessage(ctx context.Context, peerID int64, msgID int) error {
	peer, err := c.resolvePeer(ctx, peerID)
	if err != nil {
		return fmt.Errorf("resolve peer: %w", err)
	}

	_, err = c.client.API().MessagesUpdatePinnedMessage(ctx, &tg.MessagesUpdatePinnedMessageRequest{
		Peer:  peer,
		ID:    msgID,
		Unpin: true,
	})
	if err != nil {
		return fmt.Errorf("unpin message: %w", err)
	}
	return nil
}

// UnpinAllMessages unpins all pinned messages in a chat.
func (c *MCUBClient) UnpinAllMessages(ctx context.Context, peerID int64) error {
	peer, err := c.resolvePeer(ctx, peerID)
	if err != nil {
		return fmt.Errorf("resolve peer: %w", err)
	}

	_, err = c.client.API().MessagesUnpinAllMessages(ctx, &tg.MessagesUnpinAllMessagesRequest{
		Peer: peer,
	})
	if err != nil {
		return fmt.Errorf("unpin all messages: %w", err)
	}
	return nil
}

// SendAlbum sends multiple local files as a media album (grouped messages).
func (c *MCUBClient) SendAlbum(ctx context.Context, peerID int64, files []string, caption string) ([]*tg.Message, error) {
	peer, err := c.resolvePeer(ctx, peerID)
	if err != nil {
		return nil, fmt.Errorf("resolve peer: %w", err)
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no files provided for album")
	}

	multiMedia := make([]tg.InputSingleMedia, 0, len(files))
	for i, filePath := range files {
		uploaded, err := c.uploadFile(ctx, UploadParams{Path: filePath})
		if err != nil {
			return nil, fmt.Errorf("upload file %d: %w", i, err)
		}

		media := &tg.InputMediaUploadedDocument{
			File:     uploaded.InputFile,
			MimeType: "application/octet-stream",
			Attributes: []tg.DocumentAttributeClass{
				&tg.DocumentAttributeFilename{FileName: uploaded.FileName},
			},
		}

		msg := ""
		if i == 0 {
			msg = caption
		}

		multiMedia = append(multiMedia, tg.InputSingleMedia{
			Media:    media,
			RandomID: rand.Int63(),
			Message:  msg,
		})
	}

	result, err := c.client.API().MessagesSendMultiMedia(ctx, &tg.MessagesSendMultiMediaRequest{
		Peer:       peer,
		MultiMedia: multiMedia,
	})
	if err != nil {
		return nil, fmt.Errorf("send album: %w", err)
	}

	var msgs []*tg.Message
	switch u := result.(type) {
	case *tg.Updates:
		for _, upd := range u.Updates {
			switch v := upd.(type) {
			case *tg.UpdateNewMessage:
				if m, ok := v.Message.(*tg.Message); ok {
					msgs = append(msgs, m)
				}
			case *tg.UpdateNewChannelMessage:
				if m, ok := v.Message.(*tg.Message); ok {
					msgs = append(msgs, m)
				}
			}
		}
	}
	return msgs, nil
}

// ReadHistory marks all messages up to maxID as read in the given chat.
// Pass maxID = 0 to mark all messages as read.
func (c *MCUBClient) ReadHistory(ctx context.Context, peerID int64, maxID int) error {
	peer, err := c.resolvePeer(ctx, peerID)
	if err != nil {
		return fmt.Errorf("resolve peer: %w", err)
	}

	_, err = c.client.API().MessagesReadHistory(ctx, &tg.MessagesReadHistoryRequest{
		Peer:  peer,
		MaxID: maxID,
	})
	if err != nil {
		return fmt.Errorf("read history: %w", err)
	}
	return nil
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
