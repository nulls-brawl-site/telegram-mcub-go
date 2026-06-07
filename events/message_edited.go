package events

import (
	"context"
	"time"

	"github.com/gotd/td/tg"
)

// MessageEdited is fired when an existing message is edited.
type MessageEdited struct {
	// Raw is the underlying tg.Message after the edit.
	Raw *tg.Message

	// PeerID is the resolved numeric peer ID.
	PeerID int64

	// SenderID is the user ID of the original sender.
	SenderID int64

	// Date is the original message timestamp.
	Date time.Time

	// EditDate is the timestamp of the edit.
	EditDate time.Time
}

// EventType implements Event.
func (e *MessageEdited) EventType() string { return "MessageEdited" }

// Text returns the updated plain-text message body.
func (e *MessageEdited) Text() string {
	if e.Raw == nil {
		return ""
	}
	return e.Raw.Message
}

// MessageEditedFromUpdate constructs a MessageEdited event from a tg.UpdateEditMessage
// or tg.UpdateEditChannelMessage.
func MessageEditedFromUpdate(ctx context.Context, u tg.UpdateClass) (*MessageEdited, bool) {
	_ = ctx
	var raw *tg.Message
	switch upd := u.(type) {
	case *tg.UpdateEditMessage:
		m, ok := upd.Message.(*tg.Message)
		if !ok {
			return nil, false
		}
		raw = m
	case *tg.UpdateEditChannelMessage:
		m, ok := upd.Message.(*tg.Message)
		if !ok {
			return nil, false
		}
		raw = m
	default:
		return nil, false
	}

	ev := &MessageEdited{
		Raw:      raw,
		Date:     time.Unix(int64(raw.Date), 0),
		EditDate: time.Unix(int64(raw.EditDate), 0),
	}

	if raw.PeerID != nil {
		switch p := raw.PeerID.(type) {
		case *tg.PeerUser:
			ev.PeerID = int64(p.UserID)
		case *tg.PeerChat:
			ev.PeerID = -int64(p.ChatID)
		case *tg.PeerChannel:
			ev.PeerID = -1000000000000 - int64(p.ChannelID)
		}
	}

	if raw.FromID != nil {
		if f, ok := raw.FromID.(*tg.PeerUser); ok {
			ev.SenderID = int64(f.UserID)
		}
	}

	return ev, true
}

// MessageEditedFilter returns a Filter that accepts only MessageEdited events.
func MessageEditedFilter() Filter {
	return func(e Event) bool {
		_, ok := e.(*MessageEdited)
		return ok
	}
}
