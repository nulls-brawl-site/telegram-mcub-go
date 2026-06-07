package types

import (
	"time"

	"github.com/gotd/td/tg"
)

// ParseMode controls how message text is parsed.
type ParseMode int

const (
	// ParseModeNone sends the text as-is with no formatting.
	ParseModeNone ParseMode = iota
	// ParseModeHTML interprets the text as HTML (Telegram's HTML subset + MCUB extensions).
	ParseModeHTML
	// ParseModeMarkdown interprets the text as Markdown V2.
	ParseModeMarkdown
)

// SendMessageOptions contains options for sending a message.
type SendMessageOptions struct {
	// ParseMode controls text formatting.
	ParseMode ParseMode

	// Buttons is the optional keyboard markup.
	Buttons ButtonGrid

	// ReplyToMsgID is the ID of the message to reply to.
	ReplyToMsgID int

	// Silent suppresses notifications for the recipient.
	Silent bool

	// ScheduleDate schedules the message for future delivery (Unix timestamp).
	ScheduleDate int

	// ClearDraft clears the chat draft after sending.
	ClearDraft bool

	// NoWebpage disables link preview.
	NoWebpage bool

	// ForumTopicID is the topic ID when sending to a forum supergroup topic.
	ForumTopicID int

	// ProtectContent prevents forwarding/saving.
	ProtectContent bool
}

// SendFileOptions contains options for sending a file.
type SendFileOptions struct {
	// Caption is the optional file caption.
	Caption string

	// ParseMode controls caption formatting.
	ParseMode ParseMode

	// Buttons is the optional keyboard markup.
	Buttons ButtonGrid

	// ReplyToMsgID is the ID of the message to reply to.
	ReplyToMsgID int

	// Silent suppresses notifications.
	Silent bool

	// ForumTopicID is the topic ID when sending to a forum topic.
	ForumTopicID int

	// ProtectContent prevents forwarding/saving.
	ProtectContent bool

	// FileName overrides the upload file name.
	FileName string

	// MimeType overrides the detected MIME type.
	MimeType string

	// AsDocument forces the file to be sent as a document (not media).
	AsDocument bool

	// Thumb is an optional thumbnail for video/audio files.
	Thumb string

	// Duration is the duration in seconds (for audio/video).
	Duration int

	// Width/Height for images or videos.
	Width  int
	Height int
}

// Message is an extended Telegram message with MCUB helper fields.
type Message struct {
	// Raw is the underlying gotd/td message.
	Raw *tg.Message

	// PeerID is the resolved peer ID.
	PeerID int64

	// FromID is the sender's user ID (0 for channel posts).
	FromID int64

	// Date is the parsed message timestamp.
	Date time.Time

	// Text is the plain-text message body.
	Text string

	// Entities are the parsed formatting entities.
	Entities []tg.MessageEntityClass

	// IsForumTopic indicates the message belongs to a forum topic.
	IsForumTopic bool

	// ForumTopicID is the topic ID (only set when IsForumTopic is true).
	ForumTopicID int

	// ReplyToMsgID is the replied-to message ID.
	ReplyToMsgID int
}

// ---------------------------------------------------------------------------
// MCUBMessage — rich wrapper ported from Telethon's tl/custom/message.py
// ---------------------------------------------------------------------------

// MCUBMessage wraps *tg.Message with rich helper methods that mirror the
// Telethon custom Message class.
type MCUBMessage struct {
	// Raw is the underlying gotd/td message.
	Raw *tg.Message

	// Client is a back-reference to the owning client; typed as interface{}
	// to avoid an import cycle.  Callers may type-assert to *client.MCUBClient.
	Client interface{}

	// ChatID is the signed peer ID of the conversation.
	ChatID int64

	// SenderID is the user ID of the sender (0 for anonymous posts).
	SenderID int64
}

// ID returns the message ID.
func (m *MCUBMessage) ID() int {
	if m.Raw == nil {
		return 0
	}
	return m.Raw.ID
}

// Text returns the plain-text message body.
func (m *MCUBMessage) Text() string {
	if m.Raw == nil {
		return ""
	}
	return m.Raw.Message
}

// Date returns the UTC timestamp of the message.
func (m *MCUBMessage) Date() time.Time {
	if m.Raw == nil {
		return time.Time{}
	}
	return time.Unix(int64(m.Raw.Date), 0)
}

// IsOutgoing reports whether the message was sent by the local user.
func (m *MCUBMessage) IsOutgoing() bool {
	if m.Raw == nil {
		return false
	}
	return m.Raw.Out
}

// IsReply reports whether the message is a reply to another message.
func (m *MCUBMessage) IsReply() bool {
	if m.Raw == nil {
		return false
	}
	return m.Raw.ReplyTo != nil
}

// ReplyToID returns the ID of the message being replied to (0 if not a reply).
func (m *MCUBMessage) ReplyToID() int {
	if m.Raw == nil || m.Raw.ReplyTo == nil {
		return 0
	}
	if rt, ok := m.Raw.ReplyTo.(*tg.MessageReplyHeader); ok {
		return rt.ReplyToMsgID
	}
	return 0
}

// HasMedia reports whether the message carries any media.
func (m *MCUBMessage) HasMedia() bool {
	return m.Raw != nil && m.Raw.Media != nil
}

// IsPhoto reports whether the message media is a photo.
func (m *MCUBMessage) IsPhoto() bool {
	if !m.HasMedia() {
		return false
	}
	_, ok := m.Raw.Media.(*tg.MessageMediaPhoto)
	return ok
}

// IsDocument reports whether the message media is a document.
func (m *MCUBMessage) IsDocument() bool {
	if !m.HasMedia() {
		return false
	}
	_, ok := m.Raw.Media.(*tg.MessageMediaDocument)
	return ok
}

// document returns the tg.Document if the media is a document, else nil.
func (m *MCUBMessage) document() *tg.Document {
	if !m.IsDocument() {
		return nil
	}
	mmd, ok := m.Raw.Media.(*tg.MessageMediaDocument)
	if !ok {
		return nil
	}
	doc, ok := mmd.Document.(*tg.Document)
	if !ok {
		return nil
	}
	return doc
}

// hasDocumentAttr checks whether the document has an attribute of the given type.
func (m *MCUBMessage) hasDocumentAttr(check func(tg.DocumentAttributeClass) bool) bool {
	doc := m.document()
	if doc == nil {
		return false
	}
	for _, attr := range doc.Attributes {
		if check(attr) {
			return true
		}
	}
	return false
}

// IsVideo reports whether the message is a video.
func (m *MCUBMessage) IsVideo() bool {
	return m.hasDocumentAttr(func(a tg.DocumentAttributeClass) bool {
		if va, ok := a.(*tg.DocumentAttributeVideo); ok {
			return !va.RoundMessage
		}
		return false
	})
}

// IsAudio reports whether the message is an audio file (not a voice note).
func (m *MCUBMessage) IsAudio() bool {
	return m.hasDocumentAttr(func(a tg.DocumentAttributeClass) bool {
		if aa, ok := a.(*tg.DocumentAttributeAudio); ok {
			return !aa.Voice
		}
		return false
	})
}

// IsVoice reports whether the message is a voice note.
func (m *MCUBMessage) IsVoice() bool {
	return m.hasDocumentAttr(func(a tg.DocumentAttributeClass) bool {
		if aa, ok := a.(*tg.DocumentAttributeAudio); ok {
			return aa.Voice
		}
		return false
	})
}

// IsSticker reports whether the message is a sticker.
func (m *MCUBMessage) IsSticker() bool {
	return m.hasDocumentAttr(func(a tg.DocumentAttributeClass) bool {
		_, ok := a.(*tg.DocumentAttributeSticker)
		return ok
	})
}

// IsGIF reports whether the message is an animated GIF/mp4.
func (m *MCUBMessage) IsGIF() bool {
	return m.hasDocumentAttr(func(a tg.DocumentAttributeClass) bool {
		_, ok := a.(*tg.DocumentAttributeAnimated)
		return ok
	})
}

// FileName returns the file name of the document, if any.
func (m *MCUBMessage) FileName() string {
	doc := m.document()
	if doc == nil {
		return ""
	}
	for _, attr := range doc.Attributes {
		if fa, ok := attr.(*tg.DocumentAttributeFilename); ok {
			return fa.FileName
		}
	}
	return ""
}

// FileSize returns the file size in bytes (0 if no document).
func (m *MCUBMessage) FileSize() int64 {
	doc := m.document()
	if doc == nil {
		return 0
	}
	return doc.Size
}

// MimeType returns the MIME type of the document, or an empty string.
func (m *MCUBMessage) MimeType() string {
	doc := m.document()
	if doc == nil {
		return ""
	}
	return doc.MimeType
}

// Caption returns the message text when it accompanies media (same as Text).
func (m *MCUBMessage) Caption() string {
	return m.Text()
}

// IsForwarded reports whether the message was forwarded from another chat.
func (m *MCUBMessage) IsForwarded() bool {
	if m.Raw == nil {
		return false
	}
	_, ok := m.Raw.GetFwdFrom()
	return ok
}

// ForwardedFrom returns the sender ID of the original message, or 0.
func (m *MCUBMessage) ForwardedFrom() int64 {
	if m.Raw == nil {
		return 0
	}
	fwd, ok := m.Raw.GetFwdFrom()
	if !ok {
		return 0
	}
	fromID, hasFrom := fwd.GetFromID()
	if !hasFrom {
		return 0
	}
	switch p := fromID.(type) {
	case *tg.PeerUser:
		return int64(p.UserID)
	case *tg.PeerChannel:
		return int64(p.ChannelID)
	}
	return 0
}

// Via returns the username of the inline bot the message was sent via, or "".
func (m *MCUBMessage) Via() string {
	if m.Raw == nil {
		return ""
	}
	// ViaBotsID is the bot's user ID; without a name resolver we return the
	// numeric ID as a string representation stub — callers can resolve it.
	_, ok := m.Raw.GetViaBotID()
	if !ok {
		return ""
	}
	return "" // Name resolution requires the client; return blank here.
}

// GroupedID returns the album group ID (non-zero when message belongs to an album).
func (m *MCUBMessage) GroupedID() int64 {
	if m.Raw == nil {
		return 0
	}
	id, ok := m.Raw.GetGroupedID()
	if !ok {
		return 0
	}
	return id
}

// Buttons parses the reply markup and returns the grid of MessageButtons.
// Returns nil when there are no buttons.
func (m *MCUBMessage) Buttons() [][]*MessageButton {
	if m.Raw == nil || m.Raw.ReplyMarkup == nil {
		return nil
	}
	return ParseMarkup(m.Raw.ReplyMarkup)
}

// GetButton returns the button at the given (row, col) position, or nil.
func (m *MCUBMessage) GetButton(row, col int) *MessageButton {
	grid := m.Buttons()
	if row < 0 || row >= len(grid) {
		return nil
	}
	r := grid[row]
	if col < 0 || col >= len(r) {
		return nil
	}
	return r[col]
}

// NewMCUBMessage wraps a raw tg.Message into a MCUBMessage, resolving peer IDs.
func NewMCUBMessage(raw *tg.Message, client interface{}) *MCUBMessage {
	m := &MCUBMessage{Raw: raw, Client: client}
	if raw == nil {
		return m
	}
	if raw.PeerID != nil {
		switch p := raw.PeerID.(type) {
		case *tg.PeerUser:
			m.ChatID = int64(p.UserID)
		case *tg.PeerChat:
			m.ChatID = -int64(p.ChatID)
		case *tg.PeerChannel:
			m.ChatID = -1000000000000 - int64(p.ChannelID)
		}
	}
	if raw.FromID != nil {
		switch f := raw.FromID.(type) {
		case *tg.PeerUser:
			m.SenderID = int64(f.UserID)
		case *tg.PeerChannel:
			m.SenderID = -1000000000000 - int64(f.ChannelID)
		}
	}
	return m
}

// ---------------------------------------------------------------------------

// FromTLMessage converts a tg.Message to the extended Message type.
func FromTLMessage(m *tg.Message) *Message {
	msg := &Message{
		Raw:      m,
		Text:     m.Message,
		Entities: m.Entities,
		Date:     time.Unix(int64(m.Date), 0),
	}

	// Resolve peer ID.
	if m.PeerID != nil {
		switch p := m.PeerID.(type) {
		case *tg.PeerUser:
			msg.PeerID = int64(p.UserID)
		case *tg.PeerChat:
			msg.PeerID = -int64(p.ChatID)
		case *tg.PeerChannel:
			msg.PeerID = -1000000000000 - int64(p.ChannelID)
		}
	}

	// Resolve sender.
	if m.FromID != nil {
		switch f := m.FromID.(type) {
		case *tg.PeerUser:
			msg.FromID = int64(f.UserID)
		case *tg.PeerChannel:
			msg.FromID = int64(f.ChannelID)
		}
	}

	// Forum topic fields.
	if m.ReplyTo != nil {
		if rt, ok := m.ReplyTo.(*tg.MessageReplyHeader); ok {
			if rt.ForumTopic {
				msg.IsForumTopic = true
				msg.ForumTopicID, _ = rt.GetReplyToTopID()
			}
			msg.ReplyToMsgID = rt.ReplyToMsgID
		}
	}

	return msg
}
