package events

import (
	"context"
	"time"

	"github.com/gotd/td/tg"
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
}

// EventType implements Event.
func (e *NewMessage) EventType() string { return "NewMessage" }

// Text returns the plain-text body of the message.
func (e *NewMessage) Text() string {
	if e.Raw == nil {
		return ""
	}
	return e.Raw.Message
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
			if rt.ForumTopic {
				ev.IsForumTopic = true
				ev.ForumTopicID, _ = rt.GetReplyToTopID()
			}
		}
	}

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
