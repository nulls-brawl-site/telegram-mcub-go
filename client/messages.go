package client

import (
	"context"
	"fmt"
	"math/rand"
	"path/filepath"

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
	if peerID == 0 {
		return &tg.InputPeerSelf{}, nil
	}
	// Positive → user
	if peerID > 0 {
		return &tg.InputPeerUser{UserID: peerID, AccessHash: 0}, nil
	}
	// Packed channel/supergroup peer ID: -(channel_id + 1_000_000_000_000)
	if peerID < -999999999 {
		chanID := channelIDFromPeerID(peerID)
		return &tg.InputPeerChannel{ChannelID: chanID, AccessHash: 0}, nil
	}
	// Basic group
	return &tg.InputPeerChat{ChatID: -peerID}, nil
}

// resolveInputUser converts a user ID to tg.InputUserClass.
// Pass 0 to get InputUserSelf.
func (c *MCUBClient) resolveInputUser(ctx context.Context, userID int64) (tg.InputUserClass, error) {
	_ = ctx
	if userID == 0 {
		return &tg.InputUserSelf{}, nil
	}
	return &tg.InputUser{UserID: userID, AccessHash: 0}, nil
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

// --- Additional message methods (ported from Telethon-MCUB) ---

// SearchParams holds parameters for searching messages.
type SearchParams struct {
	// PeerID is the peer to search within. Use 0 for global search.
	PeerID int64
	// Query is the search text.
	Query string
	// Limit is the maximum number of results to return.
	Limit int
	// OffsetID is the message ID to start from.
	OffsetID int
	// FromID restricts results to messages sent by this user.
	FromID int64
	// Filter restricts by media type: "photo", "video", "document", "url", "audio", "voice", "music".
	Filter string
	// MinDate is the minimum message date (Unix timestamp).
	MinDate int
	// MaxDate is the maximum message date (Unix timestamp).
	MaxDate int
}

// HistoryParams holds parameters for fetching message history.
type HistoryParams struct {
	// Limit is the maximum number of messages to return.
	Limit int
	// OffsetID is the message ID to start from.
	OffsetID int
	// OffsetDate restricts to messages before this date (Unix timestamp).
	OffsetDate int
	// MaxID returns only messages with ID less than this value.
	MaxID int
	// MinID returns only messages with ID greater than this value.
	MinID int
	// AddOffset is an additional negative offset.
	AddOffset int
	// Hash is used for caching; pass 0 to disable.
	Hash int64
	// Reverse returns messages in ascending order (oldest first).
	Reverse bool
}

// searchFilterFromString converts a filter name to a tg.MessagesFilterClass.
func searchFilterFromString(filter string) tg.MessagesFilterClass {
	switch filter {
	case "photo":
		return &tg.InputMessagesFilterPhotos{}
	case "video":
		return &tg.InputMessagesFilterVideo{}
	case "document":
		return &tg.InputMessagesFilterDocument{}
	case "url":
		return &tg.InputMessagesFilterURL{}
	case "audio", "music":
		return &tg.InputMessagesFilterMusic{}
	case "voice":
		return &tg.InputMessagesFilterVoice{}
	default:
		return &tg.InputMessagesFilterEmpty{}
	}
}

// SearchMessages searches messages within a peer or globally.
// Set params.PeerID to 0 to perform a global search.
func (c *MCUBClient) SearchMessages(ctx context.Context, params SearchParams) ([]*tg.Message, error) {
	limit := params.Limit
	if limit <= 0 {
		limit = 100
	}

	if params.PeerID == 0 {
		// Global search.
		result, err := c.api.MessagesSearchGlobal(ctx, &tg.MessagesSearchGlobalRequest{
			Q:        params.Query,
			Filter:   searchFilterFromString(params.Filter),
			MinDate:  params.MinDate,
			MaxDate:  params.MaxDate,
			OffsetID: params.OffsetID,
			Limit:    limit,
		})
		if err != nil {
			return nil, fmt.Errorf("search global: %w", err)
		}
		return extractMessages(result), nil
	}

	peer, err := c.resolvePeer(ctx, params.PeerID)
	if err != nil {
		return nil, fmt.Errorf("resolve peer: %w", err)
	}

	req := &tg.MessagesSearchRequest{
		Peer:     peer,
		Q:        params.Query,
		Filter:   searchFilterFromString(params.Filter),
		MinDate:  params.MinDate,
		MaxDate:  params.MaxDate,
		OffsetID: params.OffsetID,
		Limit:    limit,
	}

	if params.FromID != 0 {
		req.SetFromID(&tg.InputPeerUser{UserID: params.FromID})
	}

	result, err := c.api.MessagesSearch(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("search messages: %w", err)
	}
	return extractMessages(result), nil
}

// GetHistory returns the message history for a peer.
func (c *MCUBClient) GetHistory(ctx context.Context, peerID int64, params HistoryParams) ([]*tg.Message, error) {
	peer, err := c.resolvePeer(ctx, peerID)
	if err != nil {
		return nil, fmt.Errorf("resolve peer: %w", err)
	}

	limit := params.Limit
	if limit <= 0 {
		limit = 100
	}

	result, err := c.api.MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{
		Peer:       peer,
		OffsetID:   params.OffsetID,
		OffsetDate: params.OffsetDate,
		AddOffset:  params.AddOffset,
		Limit:      limit,
		MaxID:      params.MaxID,
		MinID:      params.MinID,
		Hash:       params.Hash,
	})
	if err != nil {
		return nil, fmt.Errorf("get history: %w", err)
	}

	msgs := extractMessages(result)

	if params.Reverse {
		for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
			msgs[i], msgs[j] = msgs[j], msgs[i]
		}
	}

	return msgs, nil
}

// DeleteMessages deletes messages by their IDs in the given peer.
// For channels/supergroups the peer is required; for other chats revoke controls
// whether to delete for everyone.
func (c *MCUBClient) DeleteMessages(ctx context.Context, peerID int64, ids []int, revoke bool) error {
	if len(ids) == 0 {
		return nil
	}

	if peerID < -999999999 {
		chanID := channelIDFromPeerID(peerID)
		_, err := c.api.ChannelsDeleteMessages(ctx, &tg.ChannelsDeleteMessagesRequest{
			Channel: &tg.InputChannel{ChannelID: chanID},
			ID:      ids,
		})
		return err
	}

	_, err := c.api.MessagesDeleteMessages(ctx, &tg.MessagesDeleteMessagesRequest{
		Revoke: revoke,
		ID:     ids,
	})
	return err
}

// MarkAsRead marks messages up to maxID as read in the given peer.
// Pass maxID = 0 to mark all messages as read.
func (c *MCUBClient) MarkAsRead(ctx context.Context, peerID int64, maxID int) error {
	if peerID < -999999999 {
		chanID := channelIDFromPeerID(peerID)
		_, err := c.api.ChannelsReadHistory(ctx, &tg.ChannelsReadHistoryRequest{
			Channel: &tg.InputChannel{ChannelID: chanID},
			MaxID:   maxID,
		})
		return err
	}

	peer, err := c.resolvePeer(ctx, peerID)
	if err != nil {
		return fmt.Errorf("resolve peer: %w", err)
	}
	_, err = c.api.MessagesReadHistory(ctx, &tg.MessagesReadHistoryRequest{
		Peer:  peer,
		MaxID: maxID,
	})
	return err
}

// SendVoice uploads and sends a voice message to the given peer.
func (c *MCUBClient) SendVoice(ctx context.Context, peerID int64, filePath string, caption string) (*tg.Message, error) {
	uploaded, err := c.uploadFile(ctx, UploadParams{Path: filePath})
	if err != nil {
		return nil, fmt.Errorf("upload voice: %w", err)
	}

	peer, err := c.resolvePeer(ctx, peerID)
	if err != nil {
		return nil, fmt.Errorf("resolve peer: %w", err)
	}

	media := &tg.InputMediaUploadedDocument{
		File:     uploaded.InputFile,
		MimeType: "audio/ogg",
		Attributes: []tg.DocumentAttributeClass{
			&tg.DocumentAttributeAudio{
				Voice:    true,
				Duration: 0,
			},
		},
	}

	result, err := c.api.MessagesSendMedia(ctx, &tg.MessagesSendMediaRequest{
		Peer:     peer,
		Media:    media,
		Message:  caption,
		RandomID: rand.Int63(),
	})
	if err != nil {
		return nil, fmt.Errorf("send voice: %w", err)
	}
	return extractMessageFromUpdates(result), nil
}

// SendSticker uploads and sends a sticker file to the given peer.
func (c *MCUBClient) SendSticker(ctx context.Context, peerID int64, filePath string) (*tg.Message, error) {
	uploaded, err := c.uploadFile(ctx, UploadParams{Path: filePath})
	if err != nil {
		return nil, fmt.Errorf("upload sticker: %w", err)
	}

	peer, err := c.resolvePeer(ctx, peerID)
	if err != nil {
		return nil, fmt.Errorf("resolve peer: %w", err)
	}

	ext := filepath.Ext(filePath)
	mime := "image/webp"
	if ext == ".tgs" {
		mime = "application/x-tgsticker"
	} else if ext == ".webm" {
		mime = "video/webm"
	}

	media := &tg.InputMediaUploadedDocument{
		File:     uploaded.InputFile,
		MimeType: mime,
		Attributes: []tg.DocumentAttributeClass{
			&tg.DocumentAttributeSticker{
				Alt:      "",
				Stickerset: &tg.InputStickerSetEmpty{},
			},
		},
	}

	result, err := c.api.MessagesSendMedia(ctx, &tg.MessagesSendMediaRequest{
		Peer:     peer,
		Media:    media,
		RandomID: rand.Int63(),
	})
	if err != nil {
		return nil, fmt.Errorf("send sticker: %w", err)
	}
	return extractMessageFromUpdates(result), nil
}

// SendDice sends a dice message with the given emoji (e.g. "🎲", "🎯", "🏀").
func (c *MCUBClient) SendDice(ctx context.Context, peerID int64, emoji string) (*tg.Message, error) {
	peer, err := c.resolvePeer(ctx, peerID)
	if err != nil {
		return nil, fmt.Errorf("resolve peer: %w", err)
	}

	result, err := c.api.MessagesSendMedia(ctx, &tg.MessagesSendMediaRequest{
		Peer: peer,
		Media: &tg.InputMediaDice{
			Emoticon: emoji,
		},
		RandomID: rand.Int63(),
	})
	if err != nil {
		return nil, fmt.Errorf("send dice: %w", err)
	}
	return extractMessageFromUpdates(result), nil
}

// GetScheduledMessages returns all scheduled messages for the given peer.
func (c *MCUBClient) GetScheduledMessages(ctx context.Context, peerID int64) ([]*tg.Message, error) {
	peer, err := c.resolvePeer(ctx, peerID)
	if err != nil {
		return nil, fmt.Errorf("resolve peer: %w", err)
	}

	result, err := c.api.MessagesGetScheduledHistory(ctx, &tg.MessagesGetScheduledHistoryRequest{
		Peer: peer,
		Hash: 0,
	})
	if err != nil {
		return nil, fmt.Errorf("get scheduled messages: %w", err)
	}
	return extractMessages(result), nil
}

// SendScheduledMessage sends a previously scheduled message immediately.
func (c *MCUBClient) SendScheduledMessage(ctx context.Context, peerID int64, msgID int) error {
	peer, err := c.resolvePeer(ctx, peerID)
	if err != nil {
		return fmt.Errorf("resolve peer: %w", err)
	}

	_, err = c.api.MessagesSendScheduledMessages(ctx, &tg.MessagesSendScheduledMessagesRequest{
		Peer: peer,
		ID:   []int{msgID},
	})
	if err != nil {
		return fmt.Errorf("send scheduled message: %w", err)
	}
	return nil
}

// DeleteScheduledMessages deletes scheduled messages by their IDs.
func (c *MCUBClient) DeleteScheduledMessages(ctx context.Context, peerID int64, ids []int) error {
	if len(ids) == 0 {
		return nil
	}

	peer, err := c.resolvePeer(ctx, peerID)
	if err != nil {
		return fmt.Errorf("resolve peer: %w", err)
	}

	_, err = c.api.MessagesDeleteScheduledMessages(ctx, &tg.MessagesDeleteScheduledMessagesRequest{
		Peer: peer,
		ID:   ids,
	})
	if err != nil {
		return fmt.Errorf("delete scheduled messages: %w", err)
	}
	return nil
}

// TranslateMessage translates a message to the target language.
// Returns the translated text. toLang is a two-letter ISO 639-1 code (e.g. "en", "ru").
func (c *MCUBClient) TranslateMessage(ctx context.Context, peerID int64, msgID int, toLang string) (string, error) {
	peer, err := c.resolvePeer(ctx, peerID)
	if err != nil {
		return "", fmt.Errorf("resolve peer: %w", err)
	}

	req := &tg.MessagesTranslateTextRequest{
		ToLang: toLang,
	}
	req.SetPeer(peer)
	req.SetID([]int{msgID})

	result, err := c.api.MessagesTranslateText(ctx, req)
	if err != nil {
		return "", fmt.Errorf("translate message: %w", err)
	}

	if result == nil || len(result.Result) == 0 {
		return "", nil
	}
	return result.Result[0].Text, nil
}

// GetMessageLink returns the t.me link for a message in a channel or supergroup.
func (c *MCUBClient) GetMessageLink(ctx context.Context, peerID int64, msgID int) (string, error) {
	channel, err := c.resolveInputChannel(ctx, peerID)
	if err != nil {
		return "", fmt.Errorf("resolve channel: %w", err)
	}

	result, err := c.api.ChannelsExportMessageLink(ctx, &tg.ChannelsExportMessageLinkRequest{
		Channel: channel,
		ID:      msgID,
	})
	if err != nil {
		return "", fmt.Errorf("get message link: %w", err)
	}
	return result.Link, nil
}

// --- Additional methods (ported from Telethon-MCUB) ---

// GetPinnedMessages returns all pinned messages in a chat/channel.
// It uses the InputMessagesFilterPinned filter to retrieve only pinned messages.
func (c *MCUBClient) GetPinnedMessages(ctx context.Context, peerID int64) ([]*tg.Message, error) {
	peer, err := c.resolvePeer(ctx, peerID)
	if err != nil {
		return nil, fmt.Errorf("resolve peer: %w", err)
	}

	result, err := c.api.MessagesSearch(ctx, &tg.MessagesSearchRequest{
		Peer:   peer,
		Filter: &tg.InputMessagesFilterPinned{},
		Limit:  100,
	})
	if err != nil {
		return nil, fmt.Errorf("get pinned messages: %w", err)
	}
	return extractMessages(result), nil
}

// ClearPinnedMessages unpins all pinned messages in a chat.
// For channels it uses channels.updatePinnedMessage; for regular chats it
// iterates the pinned list and unpins each message.
func (c *MCUBClient) ClearPinnedMessages(ctx context.Context, peerID int64) error {
	msgs, err := c.GetPinnedMessages(ctx, peerID)
	if err != nil {
		return fmt.Errorf("clear pinned: fetch: %w", err)
	}

	peer, err := c.resolvePeer(ctx, peerID)
	if err != nil {
		return fmt.Errorf("clear pinned: resolve peer: %w", err)
	}
	for _, msg := range msgs {
		_, unpinErr := c.api.MessagesUpdatePinnedMessage(ctx, &tg.MessagesUpdatePinnedMessageRequest{
			Peer:  peer,
			ID:    msg.ID,
			Unpin: true,
		})
		if unpinErr != nil {
			return fmt.Errorf("unpin message %d: %w", msg.ID, unpinErr)
		}
	}
	return nil
}

// SearchGlobal performs a global search across all chats and returns up to limit messages.
func (c *MCUBClient) SearchGlobal(ctx context.Context, query string, limit int) ([]*tg.Message, error) {
	if limit <= 0 {
		limit = 20
	}

	result, err := c.api.MessagesSearchGlobal(ctx, &tg.MessagesSearchGlobalRequest{
		Q:          query,
		Filter:     &tg.InputMessagesFilterEmpty{},
		OffsetPeer: &tg.InputPeerEmpty{},
		Limit:      limit,
	})
	if err != nil {
		return nil, fmt.Errorf("search global %q: %w", query, err)
	}
	return extractMessages(result), nil
}

// SendPoll sends a poll to peerID.
// If quiz is true the poll is a quiz poll (single correct answer).
// Returns the sent message.
func (c *MCUBClient) SendPoll(ctx context.Context, peerID int64, question string, answers []string, quiz bool) (*tg.Message, error) {
	peer, err := c.resolvePeer(ctx, peerID)
	if err != nil {
		return nil, fmt.Errorf("resolve peer: %w", err)
	}

	pollAnswers := make([]tg.PollAnswer, len(answers))
	for i, a := range answers {
		pollAnswers[i] = tg.PollAnswer{
			Text:   a,
			Option: []byte{byte(i)},
		}
	}

	media := &tg.InputMediaPoll{
		Poll: tg.Poll{
			Question: question,
			Answers:  pollAnswers,
			Quiz:     quiz,
		},
	}

	result, err := c.api.MessagesSendMedia(ctx, &tg.MessagesSendMediaRequest{
		Peer:     peer,
		Media:    media,
		RandomID: rand.Int63(),
	})
	if err != nil {
		return nil, fmt.Errorf("send poll: %w", err)
	}
	return extractMessageFromUpdates(result), nil
}

// StopPoll closes (stops accepting votes for) a poll identified by msgID in peerID.
// Returns the updated message.
func (c *MCUBClient) StopPoll(ctx context.Context, peerID int64, msgID int) (*tg.Message, error) {
	peer, err := c.resolvePeer(ctx, peerID)
	if err != nil {
		return nil, fmt.Errorf("resolve peer: %w", err)
	}

	result, err := c.api.MessagesEditMessage(ctx, &tg.MessagesEditMessageRequest{
		Peer:  peer,
		ID:    msgID,
		Media: &tg.InputMediaPoll{Poll: tg.Poll{Closed: true}},
	})
	if err != nil {
		return nil, fmt.Errorf("stop poll message %d: %w", msgID, err)
	}
	return extractMessageFromUpdates(result), nil
}

// GetVotes returns the vote information for a specific poll answer option.
// option is the raw option bytes (as returned by PollAnswer.Option).
// limit controls how many votes to return per call (0 = server default).
func (c *MCUBClient) GetVotes(ctx context.Context, peerID int64, msgID int, option []byte, limit int) ([]tg.MessagePeerVoteClass, error) {
	peer, err := c.resolvePeer(ctx, peerID)
	if err != nil {
		return nil, fmt.Errorf("resolve peer: %w", err)
	}

	if limit <= 0 {
		limit = 50
	}

	req := &tg.MessagesGetPollVotesRequest{
		Peer:  peer,
		ID:    msgID,
		Limit: limit,
	}
	if len(option) > 0 {
		req.SetOption(option)
	}

	result, err := c.api.MessagesGetPollVotes(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("get poll votes: %w", err)
	}
	return result.Votes, nil
}

// EditMessageCaption edits the caption of a media message (document, photo, etc.).
func (c *MCUBClient) EditMessageCaption(ctx context.Context, peerID int64, msgID int, caption string) (*tg.Message, error) {
	peer, err := c.resolvePeer(ctx, peerID)
	if err != nil {
		return nil, fmt.Errorf("resolve peer: %w", err)
	}

	result, err := c.api.MessagesEditMessage(ctx, &tg.MessagesEditMessageRequest{
		Peer:    peer,
		ID:      msgID,
		Message: caption,
	})
	if err != nil {
		return nil, fmt.Errorf("edit caption message %d: %w", msgID, err)
	}
	return extractMessageFromUpdates(result), nil
}

// CopyMessage forwards a message to toPeerID without the "Forwarded from" header
// by re-sending the message content rather than forwarding it.
func (c *MCUBClient) CopyMessage(ctx context.Context, fromPeerID int64, msgID int, toPeerID int64) (*tg.Message, error) {
	msgs, err := c.GetMessages(ctx, fromPeerID, []int{msgID})
	if err != nil {
		return nil, fmt.Errorf("copy message: fetch source: %w", err)
	}
	if len(msgs) == 0 {
		return nil, fmt.Errorf("copy message: message %d not found in peer %d", msgID, fromPeerID)
	}

	src := msgs[0]
	toPeer, err := c.resolvePeer(ctx, toPeerID)
	if err != nil {
		return nil, fmt.Errorf("copy message: resolve target peer: %w", err)
	}

	req := &tg.MessagesSendMessageRequest{
		Peer:     toPeer,
		Message:  src.Message,
		RandomID: rand.Int63(),
	}

	// Carry the media if present.
	if src.Media != nil {
		mediaReq := &tg.MessagesSendMediaRequest{
			Peer:     toPeer,
			Message:  src.Message,
			RandomID: rand.Int63(),
		}
		switch m := src.Media.(type) {
		case *tg.MessageMediaPhoto:
			if photo, ok := m.Photo.(*tg.Photo); ok {
				mediaReq.Media = &tg.InputMediaPhoto{
					ID: &tg.InputPhoto{
						ID:            photo.ID,
						AccessHash:    photo.AccessHash,
						FileReference: photo.FileReference,
					},
				}
				result, sendErr := c.api.MessagesSendMedia(ctx, mediaReq)
				if sendErr != nil {
					return nil, fmt.Errorf("copy message: send media: %w", sendErr)
				}
				return extractMessageFromUpdates(result), nil
			}
		case *tg.MessageMediaDocument:
			if doc, ok := m.Document.(*tg.Document); ok {
				mediaReq.Media = &tg.InputMediaDocument{
					ID: &tg.InputDocument{
						ID:            doc.ID,
						AccessHash:    doc.AccessHash,
						FileReference: doc.FileReference,
					},
				}
				result, sendErr := c.api.MessagesSendMedia(ctx, mediaReq)
				if sendErr != nil {
					return nil, fmt.Errorf("copy message: send media: %w", sendErr)
				}
				return extractMessageFromUpdates(result), nil
			}
		}
	}

	// Text-only message.
	result, err := c.api.MessagesSendMessage(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("copy message: send: %w", err)
	}
	return extractMessageFromUpdates(result), nil
}

// GetMessageByLink resolves a t.me/... link to a message.
// Supports public channel links (t.me/username/123).
// For private links (t.me/c/channel_id/msg_id) the channel ID must already be
// accessible by the account.
func (c *MCUBClient) GetMessageByLink(ctx context.Context, link string) (*tg.Message, error) {
	// Strip common prefixes.
	for _, pfx := range []string{"https://t.me/", "http://t.me/", "t.me/"} {
		if len(link) > len(pfx) && link[:len(pfx)] == pfx {
			link = link[len(pfx):]
			break
		}
	}

	// Private link format: c/<channel_id>/<msg_id>
	if len(link) > 2 && link[:2] == "c/" {
		var chanID, msgID int64
		if _, err := fmt.Sscanf(link[2:], "%d/%d", &chanID, &msgID); err == nil {
			// Reconstruct packed peer ID.
			packedPeerID := -(chanID + 1000000000000)
			msgs, err := c.GetMessages(ctx, packedPeerID, []int{int(msgID)})
			if err != nil {
				return nil, fmt.Errorf("get message by private link %q: %w", link, err)
			}
			if len(msgs) == 0 {
				return nil, fmt.Errorf("get message by private link %q: not found", link)
			}
			return msgs[0], nil
		}
	}

	// Public link format: <username>/<msg_id>
	var username string
	var msgID int
	if _, err := fmt.Sscanf(link, "%s", &username); err != nil {
		return nil, fmt.Errorf("get message by link %q: cannot parse", link)
	}
	// Split on /
	for i, ch := range link {
		if ch == '/' {
			username = link[:i]
			if _, err := fmt.Sscanf(link[i+1:], "%d", &msgID); err != nil {
				return nil, fmt.Errorf("get message by link %q: cannot parse message ID", link)
			}
			break
		}
	}
	if msgID == 0 {
		return nil, fmt.Errorf("get message by link %q: no message ID found", link)
	}

	// Resolve the username to a peer.
	resolved, err := c.api.ContactsResolveUsername(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("get message by link %q: resolve username: %w", link, err)
	}

	var peer tg.InputPeerClass
	switch p := resolved.Peer.(type) {
	case *tg.PeerChannel:
		peer = &tg.InputPeerChannel{ChannelID: p.ChannelID}
	case *tg.PeerUser:
		peer = &tg.InputPeerUser{UserID: p.UserID}
	case *tg.PeerChat:
		peer = &tg.InputPeerChat{ChatID: p.ChatID}
	default:
		return nil, fmt.Errorf("get message by link %q: unknown peer type %T", link, resolved.Peer)
	}

	_ = peer
	// Fetch the message by ID relative to the resolved peer.
	// We pack a temporary peer ID for GetMessages.
	var peerID int64
	switch p := resolved.Peer.(type) {
	case *tg.PeerChannel:
		peerID = -(p.ChannelID + 1000000000000)
	case *tg.PeerUser:
		peerID = p.UserID
	case *tg.PeerChat:
		peerID = -p.ChatID
	}

	msgs, err := c.GetMessages(ctx, peerID, []int{msgID})
	if err != nil {
		return nil, fmt.Errorf("get message by link %q: %w", link, err)
	}
	if len(msgs) == 0 {
		return nil, fmt.Errorf("get message by link %q: message not found", link)
	}
	return msgs[0], nil
}

// SendInlineResult sends an inline bot query result to a chat.
// queryID is the query ID from a bot.getInlineQueryResults call.
// resultID is the identifier of the chosen result.
func (c *MCUBClient) SendInlineResult(ctx context.Context, chatID int64, queryID int64, resultID string) (*tg.Message, error) {
	peer, err := c.resolvePeer(ctx, chatID)
	if err != nil {
		return nil, fmt.Errorf("resolve peer: %w", err)
	}

	result, err := c.api.MessagesSendInlineBotResult(ctx, &tg.MessagesSendInlineBotResultRequest{
		Peer:     peer,
		QueryID:  queryID,
		ID:       resultID,
		RandomID: rand.Int63(),
	})
	if err != nil {
		return nil, fmt.Errorf("send inline result: %w", err)
	}
	return extractMessageFromUpdates(result), nil
}
