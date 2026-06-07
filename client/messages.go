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
