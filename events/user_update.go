package events

import (
	"context"
	"time"

	"github.com/gotd/td/tg"
)

// UserUpdate fires when a user's online status or typing action changes.
// Mirrors Telethon's UserUpdate event.
type UserUpdate struct {
	// Raw is the underlying TG update.
	Raw tg.UpdateClass

	// UserID is the user whose state changed.
	UserID int64

	// TypingChatID is non-zero when the action occurred in a specific chat.
	TypingChatID int64

	// Status is the raw UserStatus object (non-nil when this is a status update).
	Status tg.UserStatusClass

	// Action is the raw SendMessageAction object (non-nil when typing/uploading).
	Action tg.SendMessageActionClass

	// ActionStr is the human-readable action string derived from Action.
	// One of: "typing", "cancel", "record_video", "upload_video",
	// "record_audio", "upload_audio", "upload_photo", "upload_document",
	// "geo", "contact", "playing", "record_round", "upload_round",
	// "choose_sticker", "unknown".
	ActionStr string

	// Online is true when the user just came online (Status is UserStatusOnline).
	Online bool

	// Offline is true when the user just went offline (Status is UserStatusOffline).
	Offline bool

	// LastSeen holds the timestamp when the user was last seen (Offline only).
	LastSeen time.Time

	// UntilOnline holds the time until which the user will appear online.
	UntilOnline time.Time
}

// EventType implements Event.
func (e *UserUpdate) EventType() string { return "UserUpdate" }

// IsOnline reports whether the user is currently online.
func (e *UserUpdate) IsOnline() bool { return e.Online }

// IsTyping reports whether the user is typing (ActionStr == "typing").
func (e *UserUpdate) IsTyping() bool { return e.ActionStr == "typing" }

// UserUpdateFilter returns a Filter that passes only UserUpdate events.
func UserUpdateFilter() Filter {
	return func(e Event) bool {
		_, ok := e.(*UserUpdate)
		return ok
	}
}

// UserUpdateFromUpdate attempts to build a UserUpdate from a raw TG update.
func UserUpdateFromUpdate(ctx context.Context, u tg.UpdateClass) (*UserUpdate, bool) {
	_ = ctx
	switch upd := u.(type) {
	case *tg.UpdateUserStatus:
		ev := &UserUpdate{
			Raw:    u,
			UserID: upd.UserID,
			Status: upd.Status,
		}
		switch s := upd.Status.(type) {
		case *tg.UserStatusOnline:
			ev.Online = true
			ev.UntilOnline = time.Unix(int64(s.Expires), 0)
		case *tg.UserStatusOffline:
			ev.Offline = true
			ev.LastSeen = time.Unix(int64(s.WasOnline), 0)
		}
		return ev, true

	case *tg.UpdateUserTyping:
		return &UserUpdate{
			Raw:       u,
			UserID:    upd.UserID,
			Action:    upd.Action,
			ActionStr: sendActionName(upd.Action),
		}, true

	case *tg.UpdateChatUserTyping:
		var userID int64
		if pu, ok := upd.FromID.(*tg.PeerUser); ok {
			userID = int64(pu.UserID)
		}
		return &UserUpdate{
			Raw:          u,
			UserID:       userID,
			TypingChatID: -upd.ChatID,
			Action:       upd.Action,
			ActionStr:    sendActionName(upd.Action),
		}, true

	case *tg.UpdateChannelUserTyping:
		var userID int64
		if pu, ok := upd.FromID.(*tg.PeerUser); ok {
			userID = int64(pu.UserID)
		}
		return &UserUpdate{
			Raw:          u,
			UserID:       userID,
			TypingChatID: -1000000000000 - upd.ChannelID,
			Action:       upd.Action,
			ActionStr:    sendActionName(upd.Action),
		}, true
	}
	return nil, false
}

// sendActionName maps a SendMessageActionClass to a human-readable string.
func sendActionName(a tg.SendMessageActionClass) string {
	if a == nil {
		return ""
	}
	switch a.(type) {
	case *tg.SendMessageTypingAction:
		return "typing"
	case *tg.SendMessageCancelAction:
		return "cancel"
	case *tg.SendMessageRecordVideoAction:
		return "record_video"
	case *tg.SendMessageUploadVideoAction:
		return "upload_video"
	case *tg.SendMessageRecordAudioAction:
		return "record_audio"
	case *tg.SendMessageUploadAudioAction:
		return "upload_audio"
	case *tg.SendMessageUploadPhotoAction:
		return "upload_photo"
	case *tg.SendMessageUploadDocumentAction:
		return "upload_document"
	case *tg.SendMessageGeoLocationAction:
		return "geo"
	case *tg.SendMessageChooseContactAction:
		return "contact"
	case *tg.SendMessageGamePlayAction:
		return "playing"
	case *tg.SendMessageRecordRoundAction:
		return "record_round"
	case *tg.SendMessageUploadRoundAction:
		return "upload_round"
	case *tg.SendMessageChooseStickerAction:
		return "choose_sticker"
	default:
		return "unknown"
	}
}
