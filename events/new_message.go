package events

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gotd/td/tg"

	"github.com/nulls-brawl-site/telegram-mcub-go/types"
)

// NewMessage is fired when a new message arrives.
type NewMessage struct {
	// Raw is the underlying tg.Message.
	Raw *tg.Message

	// Entities holds the parsed formatting entities.
	Entities []tg.MessageEntityClass

	// PeerID is the resolved numeric peer ID of the chat.
	PeerID int64

	// SenderID is the user ID of the sender (0 for anonymous channel posts).
	SenderID int64

	// Date is the message timestamp.
	Date time.Time

	// IsPrivate is true when the message is in a private conversation.
	IsPrivate bool

	// IsGroup is true when the message is in a group chat.
	IsGroup bool

	// IsChannel is true when the message is in a channel.
	IsChannel bool

	// IsForumTopic is true when the message belongs to a forum topic.
	IsForumTopic bool

	// ForumTopicID is the topic thread ID (only when IsForumTopic is true).
	ForumTopicID int

	// ReplyToMsgID is the ID of the message being replied to.
	ReplyToMsgID int

	// IsOutgoing is true when the message was sent by the local user.
	IsOutgoing bool

	// IsReply is true when this message replies to another.
	IsReply bool

	// IsForwarded is true when this message was forwarded.
	IsForwarded bool

	// IsBot is true when the message was sent via an inline bot (via_bot_id != 0).
	IsBot bool

	// PatternMatch holds regex group matches when a PatternFilter was applied.
	PatternMatch []string

	// client is a back-reference to the owning client for action methods.
	// Set via SetClient; typed as interface{} to avoid import cycles.
	client interface{}
}

// EventType implements Event.
func (e *NewMessage) EventType() string { return "NewMessage" }

// SetClient sets the back-reference client used by action methods.
func (e *NewMessage) SetClient(client interface{}) {
	e.client = client
}

// Text returns the plain-text body of the message.
func (e *NewMessage) Text() string {
	if e.Raw == nil {
		return ""
	}
	return e.Raw.Message
}

// RawText returns the plain message text (same as Text; no markup parsing).
func (e *NewMessage) RawText() string {
	return e.Text()
}

// Args returns the text after the first word (command arguments).
// Returns an empty string when there are no arguments.
func (e *NewMessage) Args() string {
	text := e.Text()
	parts := strings.SplitN(text, " ", 2)
	if len(parts) < 2 {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

// ArgsList returns the command arguments split into individual tokens.
// Returns nil when there are no arguments.
func (e *NewMessage) ArgsList() []string {
	args := e.Args()
	if args == "" {
		return nil
	}
	return strings.Fields(args)
}

// File returns an MCUBFile wrapping the media in this message, or nil.
func (e *NewMessage) File() *types.MCUBFile {
	if e.Raw == nil {
		return nil
	}
	return types.NewFile(e.Raw)
}

// Photo returns the photo media, or nil.
func (e *NewMessage) Photo() *tg.Photo {
	if e.Raw == nil || e.Raw.Media == nil {
		return nil
	}
	mmp, ok := e.Raw.Media.(*tg.MessageMediaPhoto)
	if !ok {
		return nil
	}
	photo, ok := mmp.Photo.(*tg.Photo)
	if !ok {
		return nil
	}
	return photo
}

// Document returns the document media, or nil.
func (e *NewMessage) Document() *tg.Document {
	if e.Raw == nil || e.Raw.Media == nil {
		return nil
	}
	mmd, ok := e.Raw.Media.(*tg.MessageMediaDocument)
	if !ok {
		return nil
	}
	doc, ok := mmd.Document.(*tg.Document)
	if !ok {
		return nil
	}
	return doc
}

// ForwardedFrom returns an MCUBForward wrapping the forward header, or nil.
func (e *NewMessage) ForwardedFrom() *types.MCUBForward {
	if e.Raw == nil {
		return nil
	}
	fwd, ok := e.Raw.GetFwdFrom()
	if !ok {
		return nil
	}
	return types.NewForward(&fwd)
}

// ViaBot returns the via-bot user ID string if the message was sent via an inline bot, else "".
func (e *NewMessage) ViaBot() string {
	if e.Raw == nil {
		return ""
	}
	_, ok := e.Raw.GetViaBotID()
	if !ok {
		return ""
	}
	return "" // Username resolution requires the client; return blank.
}

// Buttons returns the parsed inline keyboard buttons, or nil.
func (e *NewMessage) Buttons() [][]*types.MessageButton {
	if e.Raw == nil || e.Raw.ReplyMarkup == nil {
		return nil
	}
	return types.ParseMarkup(e.Raw.ReplyMarkup)
}

// Message returns a rich MCUBMessage wrapper for this event's message.
func (e *NewMessage) Message() *types.MCUBMessage {
	if e.Raw == nil {
		return nil
	}
	return types.NewMessage(e.Raw, e.client, e.PeerID)
}

// ---------------------------------------------------------------------------
// Action methods — delegate to the MCUBMessage wrapper
// ---------------------------------------------------------------------------

// Reply sends a reply to this message and returns the sent message.
func (e *NewMessage) Reply(ctx context.Context, text string) (*tg.Message, error) {
	if e.Raw == nil {
		return nil, fmt.Errorf("nil raw message")
	}
	type replyClient interface {
		SendReply(ctx context.Context, chatID int64, text string, replyToMsgID int) (*tg.Message, error)
	}
	c, ok := e.client.(replyClient)
	if !ok {
		return nil, fmt.Errorf("client does not implement SendReply")
	}
	return c.SendReply(ctx, e.PeerID, text, e.Raw.ID)
}

// Respond sends a message to the same chat without replying.
func (e *NewMessage) Respond(ctx context.Context, text string) (*tg.Message, error) {
	if e.Raw == nil {
		return nil, fmt.Errorf("nil raw message")
	}
	type sendClient interface {
		SendText(ctx context.Context, chatID int64, text string) (*tg.Message, error)
	}
	c, ok := e.client.(sendClient)
	if !ok {
		return nil, fmt.Errorf("client does not implement SendText")
	}
	return c.SendText(ctx, e.PeerID, text)
}

// Edit edits this message text.
func (e *NewMessage) Edit(ctx context.Context, text string) (*tg.Message, error) {
	if e.Raw == nil {
		return nil, fmt.Errorf("nil raw message")
	}
	type editClient interface {
		EditMessage(ctx context.Context, chatID int64, msgID int, text string) (*tg.Message, error)
	}
	c, ok := e.client.(editClient)
	if !ok {
		return nil, fmt.Errorf("client does not implement EditMessage")
	}
	return c.EditMessage(ctx, e.PeerID, e.Raw.ID, text)
}

// Delete deletes this message.
func (e *NewMessage) Delete(ctx context.Context) error {
	if e.Raw == nil {
		return fmt.Errorf("nil raw message")
	}
	type deleteClient interface {
		DeleteMessages(ctx context.Context, chatID int64, msgIDs []int, revoke bool) error
	}
	c, ok := e.client.(deleteClient)
	if !ok {
		return fmt.Errorf("client does not implement DeleteMessages")
	}
	return c.DeleteMessages(ctx, e.PeerID, []int{e.Raw.ID}, true)
}

// GetReplyMessage fetches the message this one is replying to, or nil.
func (e *NewMessage) GetReplyMessage(ctx context.Context) (*types.MCUBMessage, error) {
	m := e.Message()
	if m == nil {
		return nil, nil
	}
	return m.GetReplyMessage(ctx)
}

// Download downloads the message media to the given file path.
func (e *NewMessage) Download(ctx context.Context, path string) error {
	if e.Raw == nil {
		return fmt.Errorf("nil raw message")
	}
	type downloadClient interface {
		DownloadMedia(ctx context.Context, msg *tg.Message, filePath string) error
	}
	c, ok := e.client.(downloadClient)
	if !ok {
		return fmt.Errorf("client does not implement DownloadMedia")
	}
	return c.DownloadMedia(ctx, e.Raw, path)
}

// React sends an emoji reaction to this message.
func (e *NewMessage) React(ctx context.Context, emoji string) error {
	if e.Raw == nil {
		return fmt.Errorf("nil raw message")
	}
	type reactClient interface {
		SendReaction(ctx context.Context, chatID int64, msgID int, emoji string) error
	}
	c, ok := e.client.(reactClient)
	if !ok {
		return fmt.Errorf("client does not implement SendReaction")
	}
	return c.SendReaction(ctx, e.PeerID, e.Raw.ID, emoji)
}

// Pin pins this message.
func (e *NewMessage) Pin(ctx context.Context) error {
	if e.Raw == nil {
		return fmt.Errorf("nil raw message")
	}
	type pinClient interface {
		PinMessage(ctx context.Context, chatID int64, msgID int, notify bool) error
	}
	c, ok := e.client.(pinClient)
	if !ok {
		return fmt.Errorf("client does not implement PinMessage")
	}
	return c.PinMessage(ctx, e.PeerID, e.Raw.ID, false)
}

// NewMessageFromUpdate constructs a NewMessage event from a tg.UpdateNewMessage or similar.
func NewMessageFromUpdate(ctx context.Context, u tg.UpdateClass) (*NewMessage, bool) {
	_ = ctx
	var raw *tg.Message
	switch upd := u.(type) {
	case *tg.UpdateNewMessage:
		m, ok := upd.Message.(*tg.Message)
		if !ok {
			return nil, false
		}
		raw = m
	case *tg.UpdateNewChannelMessage:
		m, ok := upd.Message.(*tg.Message)
		if !ok {
			return nil, false
		}
		raw = m
	default:
		return nil, false
	}

	ev := &NewMessage{
		Raw:      raw,
		Entities: raw.Entities,
		Date:     time.Unix(int64(raw.Date), 0),
	}

	// Resolve peer type.
	if raw.PeerID != nil {
		switch p := raw.PeerID.(type) {
		case *tg.PeerUser:
			ev.PeerID = int64(p.UserID)
			ev.IsPrivate = true
		case *tg.PeerChat:
			ev.PeerID = -int64(p.ChatID)
			ev.IsGroup = true
		case *tg.PeerChannel:
			ev.PeerID = -1000000000000 - int64(p.ChannelID)
			ev.IsChannel = true
		}
	}

	// Resolve sender.
	if raw.FromID != nil {
		switch f := raw.FromID.(type) {
		case *tg.PeerUser:
			ev.SenderID = int64(f.UserID)
		}
	}

	// Forum topic metadata.
	if raw.ReplyTo != nil {
		if rt, ok := raw.ReplyTo.(*tg.MessageReplyHeader); ok {
			ev.ReplyToMsgID = rt.ReplyToMsgID
			ev.IsReply = true
			if rt.ForumTopic {
				ev.IsForumTopic = true
				ev.ForumTopicID, _ = rt.GetReplyToTopID()
			}
		}
	}

	ev.IsOutgoing = raw.Out
	_, ev.IsForwarded = raw.GetFwdFrom()
	_, isViaBot := raw.GetViaBotID()
	ev.IsBot = isViaBot

	return ev, true
}

// NewMessageFilter returns a Filter that accepts only NewMessage events.
func NewMessageFilter() Filter {
	return func(e Event) bool {
		_, ok := e.(*NewMessage)
		return ok
	}
}

// PrivateMessageFilter returns a Filter that accepts NewMessage events from private chats.
func PrivateMessageFilter() Filter {
	return func(e Event) bool {
		nm, ok := e.(*NewMessage)
		return ok && nm.IsPrivate
	}
}

// GroupMessageFilter returns a Filter that accepts NewMessage events from groups.
func GroupMessageFilter() Filter {
	return func(e Event) bool {
		nm, ok := e.(*NewMessage)
		return ok && nm.IsGroup
	}
}

// PatternFilter returns a Filter that only accepts NewMessage events whose text
// satisfies the given predicate.
func PatternFilter(match func(text string) bool) Filter {
	return func(e Event) bool {
		nm, ok := e.(*NewMessage)
		if !ok {
			return false
		}
		return match(nm.Text())
	}
}
