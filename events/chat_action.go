package events

import (
	"context"
	"time"

	"github.com/gotd/td/tg"
)

// ChatAction fires when users join/leave a chat, the title changes, a message
// is pinned, etc.  Mirrors Telethon's ChatAction event.
type ChatAction struct {
	// Raw is the underlying TG update.
	Raw tg.UpdateClass

	// ChatID is the signed peer ID of the chat (negative for groups/channels).
	ChatID int64

	// ActionType describes what happened.
	// One of: "user_joined", "user_left", "user_kicked", "user_invited",
	// "chat_created", "title_changed", "photo_changed", "pin_message",
	// "history_cleared", "migrate_to", "migrate_from".
	ActionType string

	// UserID is the primary user that triggered the action.
	UserID int64

	// UserIDs holds all users affected (e.g. multiple adds at once).
	UserIDs []int64

	// InviterID is the user who sent the invite link / added the user.
	InviterID int64

	// KickerID is the admin who removed the user.
	KickerID int64

	// NewTitle is the updated chat title (for "title_changed").
	NewTitle string

	// PinnedMsgID is the message that was pinned (for "pin_message").
	PinnedMsgID int

	// NewChatID is the target/source chat ID for migrations.
	NewChatID int64

	// HasNewPhoto is true when a photo was set (false means removed).
	HasNewPhoto bool

	// Date is the event timestamp.
	Date time.Time

	// IsPrivate / IsGroup / IsChannel classify the chat kind.
	IsPrivate bool
	IsGroup   bool
	IsChannel bool
}

// EventType implements Event.
func (e *ChatAction) EventType() string { return "ChatAction" }

// ChatActionFilter returns a Filter that passes only ChatAction events.
func ChatActionFilter() Filter {
	return func(e Event) bool {
		_, ok := e.(*ChatAction)
		return ok
	}
}

// IsUserJoined reports whether a user joined on their own.
func (e *ChatAction) IsUserJoined() bool { return e.ActionType == "user_joined" }

// IsUserLeft reports whether a user left on their own.
func (e *ChatAction) IsUserLeft() bool { return e.ActionType == "user_left" }

// IsUserKicked reports whether a user was removed by an admin.
func (e *ChatAction) IsUserKicked() bool { return e.ActionType == "user_kicked" }

// IsUserInvited reports whether a user was added by someone else.
func (e *ChatAction) IsUserInvited() bool { return e.ActionType == "user_invited" }

// IsTitleChanged reports whether the chat title was updated.
func (e *ChatAction) IsTitleChanged() bool { return e.ActionType == "title_changed" }

// IsPhotoChanged reports whether the chat photo was set or removed.
func (e *ChatAction) IsPhotoChanged() bool { return e.ActionType == "photo_changed" }

// IsPhotoPinned reports whether a message was pinned.
func (e *ChatAction) IsPhotoPinned() bool { return e.ActionType == "pin_message" }

// ChatActionFromUpdate attempts to build a ChatAction from a raw TG update.
// Returns (event, true) on success, (nil, false) if the update is not a chat action.
func ChatActionFromUpdate(ctx context.Context, u tg.UpdateClass) (*ChatAction, bool) {
	_ = ctx
	switch upd := u.(type) {
	case *tg.UpdateChatParticipantAdd:
		return &ChatAction{
			Raw:        u,
			ChatID:     -upd.ChatID,
			ActionType: "user_invited",
			UserID:     upd.UserID,
			UserIDs:    []int64{upd.UserID},
			InviterID:  upd.InviterID,
			IsGroup:    true,
			Date:       time.Unix(int64(upd.Date), 0),
		}, true

	case *tg.UpdateChatParticipantDelete:
		return &ChatAction{
			Raw:        u,
			ChatID:     -upd.ChatID,
			ActionType: "user_left",
			UserID:     upd.UserID,
			UserIDs:    []int64{upd.UserID},
			IsGroup:    true,
		}, true

	case *tg.UpdateChannelParticipant:
		return chatActionFromChannelParticipant(u, upd)

	case *tg.UpdateNewMessage:
		return chatActionFromServiceMsg(u, upd.Message)

	case *tg.UpdateNewChannelMessage:
		return chatActionFromServiceMsg(u, upd.Message)
	}
	return nil, false
}

// chatActionFromChannelParticipant handles join/leave events in channels/megagroups.
func chatActionFromChannelParticipant(raw tg.UpdateClass, upd *tg.UpdateChannelParticipant) (*ChatAction, bool) {
	_, hasNew := upd.GetNewParticipant()
	_, hasPrev := upd.GetPrevParticipant()

	// Only fire for pure joins or pure removals (promotions have both).
	if hasNew == hasPrev {
		return nil, false
	}

	ev := &ChatAction{
		Raw:       raw,
		ChatID:    -1000000000000 - upd.ChannelID,
		UserID:    upd.UserID,
		UserIDs:   []int64{upd.UserID},
		IsChannel: true,
		Date:      time.Unix(int64(upd.Date), 0),
	}

	actorID := upd.GetActorID()
	if hasNew {
		ev.ActionType = "user_joined"
		ev.InviterID = actorID
	} else {
		ev.ActionType = "user_kicked"
		ev.KickerID = actorID
	}
	return ev, true
}

// chatActionFromServiceMsg extracts a ChatAction from a MessageService payload.
func chatActionFromServiceMsg(raw tg.UpdateClass, msgClass tg.MessageClass) (*ChatAction, bool) {
	svc, ok := msgClass.(*tg.MessageService)
	if !ok {
		return nil, false
	}

	ev := &ChatAction{
		Raw:  raw,
		Date: time.Unix(int64(svc.Date), 0),
	}

	// Resolve chat peer.
	if svc.PeerID != nil {
		switch p := svc.PeerID.(type) {
		case *tg.PeerUser:
			ev.ChatID = int64(p.UserID)
			ev.IsPrivate = true
		case *tg.PeerChat:
			ev.ChatID = -int64(p.ChatID)
			ev.IsGroup = true
		case *tg.PeerChannel:
			ev.ChatID = -1000000000000 - int64(p.ChannelID)
			ev.IsChannel = true
		}
	}

	// Resolve actor (sender of the service message).
	if fromID, ok2 := svc.GetFromID(); ok2 {
		if pu, ok3 := fromID.(*tg.PeerUser); ok3 {
			ev.UserID = int64(pu.UserID)
		}
	}

	switch action := svc.Action.(type) {
	case *tg.MessageActionChatJoinedByLink:
		ev.ActionType = "user_joined"
		ev.InviterID = action.InviterID

	case *tg.MessageActionChatAddUser:
		ev.UserIDs = action.Users
		if len(action.Users) == 1 && action.Users[0] == ev.UserID {
			ev.ActionType = "user_joined"
		} else {
			ev.ActionType = "user_invited"
			if len(action.Users) > 0 {
				ev.UserID = action.Users[0]
			}
		}

	case *tg.MessageActionChatDeleteUser:
		if action.UserID == ev.UserID {
			ev.ActionType = "user_left"
		} else {
			ev.ActionType = "user_kicked"
			ev.KickerID = ev.UserID
		}
		ev.UserID = action.UserID
		ev.UserIDs = []int64{action.UserID}

	case *tg.MessageActionChatCreate:
		ev.ActionType = "chat_created"
		ev.NewTitle = action.Title
		ev.UserIDs = action.Users

	case *tg.MessageActionChannelCreate:
		ev.ActionType = "chat_created"
		ev.NewTitle = action.Title

	case *tg.MessageActionChatEditTitle:
		ev.ActionType = "title_changed"
		ev.NewTitle = action.Title

	case *tg.MessageActionChatEditPhoto:
		_ = action
		ev.ActionType = "photo_changed"
		ev.HasNewPhoto = true

	case *tg.MessageActionChatDeletePhoto:
		_ = action
		ev.ActionType = "photo_changed"
		ev.HasNewPhoto = false

	case *tg.MessageActionPinMessage:
		_ = action
		ev.ActionType = "pin_message"
		if replyTo, ok2 := svc.GetReplyTo(); ok2 {
			if rt, ok3 := replyTo.(*tg.MessageReplyHeader); ok3 {
				ev.PinnedMsgID = rt.ReplyToMsgID
			}
		}

	case *tg.MessageActionHistoryClear:
		_ = action
		ev.ActionType = "history_cleared"

	case *tg.MessageActionChatMigrateTo:
		ev.ActionType = "migrate_to"
		ev.NewChatID = action.ChannelID

	case *tg.MessageActionChannelMigrateFrom:
		ev.ActionType = "migrate_from"
		ev.NewChatID = action.ChatID

	default:
		return nil, false
	}

	return ev, true
}
