package events

import (
	"context"

	"github.com/gotd/td/tg"
)

// MessageRead fires whenever messages are marked as read.
// Mirrors Telethon's MessageRead event.
type MessageRead struct {
	// Raw is the underlying TG update.
	Raw tg.UpdateClass

	// ChatID is the signed peer ID of the conversation.
	ChatID int64

	// MaxID is the highest message ID that was read; all messages with
	// ID ≤ MaxID in this chat have been read.
	MaxID int

	// Outbox is true when your *outgoing* messages were read by the other
	// party.  False means you read someone else's messages (inbox).
	Outbox bool

	// UserID is the other user's ID in a private conversation (may be 0
	// for group/channel reads).
	UserID int64

	// Contents is true when the contents of a media message were consumed
	// (e.g. a voice note was played) rather than the message merely scrolled past.
	Contents bool

	// MessageIDs holds the specific message IDs when Contents is true.
	MessageIDs []int
}

// EventType implements Event.
func (e *MessageRead) EventType() string { return "MessageRead" }

// IsInbox reports whether the local user read someone else's messages.
func (e *MessageRead) IsInbox() bool { return !e.Outbox }

// IsRead reports whether the given message ID has been read.
func (e *MessageRead) IsRead(msgID int) bool {
	if e.MaxID > 0 {
		return msgID <= e.MaxID
	}
	for _, id := range e.MessageIDs {
		if id == msgID {
			return true
		}
	}
	return false
}

// MessageReadFilter returns a Filter that passes only MessageRead events.
func MessageReadFilter() Filter {
	return func(e Event) bool {
		_, ok := e.(*MessageRead)
		return ok
	}
}

// InboxReadFilter passes MessageRead events for incoming messages (you read others).
func InboxReadFilter() Filter {
	return func(e Event) bool {
		mr, ok := e.(*MessageRead)
		return ok && !mr.Outbox
	}
}

// OutboxReadFilter passes MessageRead events for outgoing messages (others read yours).
func OutboxReadFilter() Filter {
	return func(e Event) bool {
		mr, ok := e.(*MessageRead)
		return ok && mr.Outbox
	}
}

// MessageReadFromUpdate attempts to build a MessageRead from a raw TG update.
func MessageReadFromUpdate(ctx context.Context, u tg.UpdateClass) (*MessageRead, bool) {
	_ = ctx
	switch upd := u.(type) {
	case *tg.UpdateReadHistoryInbox:
		ev := &MessageRead{
			Raw:   u,
			MaxID: upd.MaxID,
			Outbox: false,
		}
		resolvePeerToChat(upd.Peer, ev)
		return ev, true

	case *tg.UpdateReadHistoryOutbox:
		ev := &MessageRead{
			Raw:    u,
			MaxID:  upd.MaxID,
			Outbox: true,
		}
		resolvePeerToChat(upd.Peer, ev)
		return ev, true

	case *tg.UpdateReadChannelInbox:
		return &MessageRead{
			Raw:    u,
			ChatID: -1000000000000 - upd.ChannelID,
			MaxID:  upd.MaxID,
			Outbox: false,
		}, true

	case *tg.UpdateReadChannelOutbox:
		return &MessageRead{
			Raw:    u,
			ChatID: -1000000000000 - upd.ChannelID,
			MaxID:  upd.MaxID,
			Outbox: true,
		}, true

	case *tg.UpdateReadMessagesContents:
		return &MessageRead{
			Raw:        u,
			Contents:   true,
			MessageIDs: upd.Messages,
		}, true

	case *tg.UpdateChannelReadMessagesContents:
		return &MessageRead{
			Raw:        u,
			ChatID:     -1000000000000 - upd.ChannelID,
			Contents:   true,
			MessageIDs: upd.Messages,
		}, true
	}
	return nil, false
}

// resolvePeerToChat fills ev.ChatID and ev.UserID from a peer.
func resolvePeerToChat(peer tg.PeerClass, ev *MessageRead) {
	if peer == nil {
		return
	}
	switch p := peer.(type) {
	case *tg.PeerUser:
		ev.ChatID = int64(p.UserID)
		ev.UserID = int64(p.UserID)
	case *tg.PeerChat:
		ev.ChatID = -int64(p.ChatID)
	case *tg.PeerChannel:
		ev.ChatID = -1000000000000 - int64(p.ChannelID)
	}
}
