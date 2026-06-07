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
