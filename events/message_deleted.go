package events

import (
	"context"

	"github.com/gotd/td/tg"
)

// MessageDeleted fires whenever one or more messages are deleted.
// Note: for non-channel chats Telegram does not include the chat ID,
// so ChatID will be 0 in those cases.
// Mirrors Telethon's MessageDeleted event.
type MessageDeleted struct {
	// Raw is the underlying TG update.
	Raw tg.UpdateClass

	// ChatID is the signed channel peer ID (non-zero only for channel deletions).
	ChatID int64

	// DeletedIDs holds the IDs of every deleted message.
	DeletedIDs []int

	// IsChannel is true when the deletion occurred in a channel/megagroup.
	IsChannel bool
}

// EventType implements Event.
func (e *MessageDeleted) EventType() string { return "MessageDeleted" }

// MessageDeletedFilter returns a Filter that passes only MessageDeleted events.
func MessageDeletedFilter() Filter {
	return func(e Event) bool {
		_, ok := e.(*MessageDeleted)
		return ok
	}
}

// MessageDeletedFromUpdate attempts to build a MessageDeleted from a raw TG update.
func MessageDeletedFromUpdate(ctx context.Context, u tg.UpdateClass) (*MessageDeleted, bool) {
	_ = ctx
	switch upd := u.(type) {
	case *tg.UpdateDeleteMessages:
		return &MessageDeleted{
			Raw:        u,
			DeletedIDs: upd.Messages,
			IsChannel:  false,
		}, true

	case *tg.UpdateDeleteChannelMessages:
		return &MessageDeleted{
			Raw:        u,
			ChatID:     -1000000000000 - upd.ChannelID,
			DeletedIDs: upd.Messages,
			IsChannel:  true,
		}, true
	}
	return nil, false
}
