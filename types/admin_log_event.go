package types

import (
	"time"

	"github.com/gotd/td/tg"
)

// AdminLogEvent wraps a channel admin log event.
// It mirrors Telethon's tl/custom/adminlogevent.AdminLogEvent class.
type AdminLogEvent struct {
	// Raw is the underlying gotd/td ChannelAdminLogEvent.
	Raw *tg.ChannelAdminLogEvent
}

// NewAdminLogEvent creates an AdminLogEvent from a raw ChannelAdminLogEvent.
func NewAdminLogEvent(e *tg.ChannelAdminLogEvent) *AdminLogEvent {
	return &AdminLogEvent{Raw: e}
}

// ID returns the event ID.
func (e *AdminLogEvent) ID() int64 {
	if e.Raw == nil {
		return 0
	}
	return e.Raw.ID
}

// Date returns the UTC timestamp when this event occurred.
func (e *AdminLogEvent) Date() time.Time {
	if e.Raw == nil {
		return time.Time{}
	}
	return time.Unix(int64(e.Raw.Date), 0)
}

// UserID returns the ID of the user who triggered the event.
func (e *AdminLogEvent) UserID() int64 {
	if e.Raw == nil {
		return 0
	}
	return e.Raw.UserID
}

// Action returns the raw ChannelAdminLogEventActionClass.
func (e *AdminLogEvent) Action() tg.ChannelAdminLogEventActionClass {
	if e.Raw == nil {
		return nil
	}
	return e.Raw.Action
}

// ActionType returns a human-readable string describing the action type.
func (e *AdminLogEvent) ActionType() string {
	if e.Raw == nil {
		return ""
	}
	switch e.Raw.Action.(type) {
	case *tg.ChannelAdminLogEventActionEditMessage:
		return "edit_message"
	case *tg.ChannelAdminLogEventActionDeleteMessage:
		return "delete_message"
	case *tg.ChannelAdminLogEventActionParticipantToggleBan:
		return "ban_user"
	case *tg.ChannelAdminLogEventActionParticipantJoin:
		return "user_join"
	case *tg.ChannelAdminLogEventActionParticipantLeave:
		return "user_leave"
	case *tg.ChannelAdminLogEventActionParticipantInvite:
		return "user_invite"
	case *tg.ChannelAdminLogEventActionParticipantToggleAdmin:
		return "toggle_admin"
	case *tg.ChannelAdminLogEventActionChangeTitle:
		return "change_title"
	case *tg.ChannelAdminLogEventActionChangeAbout:
		return "change_about"
	case *tg.ChannelAdminLogEventActionChangeUsername:
		return "change_username"
	case *tg.ChannelAdminLogEventActionChangePhoto:
		return "change_photo"
	case *tg.ChannelAdminLogEventActionToggleInvites:
		return "toggle_invites"
	case *tg.ChannelAdminLogEventActionToggleSignatures:
		return "toggle_signatures"
	case *tg.ChannelAdminLogEventActionUpdatePinned:
		return "pin_message"
	case *tg.ChannelAdminLogEventActionDefaultBannedRights:
		return "change_default_banned_rights"
	case *tg.ChannelAdminLogEventActionStopPoll:
		return "stop_poll"
	case *tg.ChannelAdminLogEventActionChangeStickerSet:
		return "change_sticker_set"
	case *tg.ChannelAdminLogEventActionTogglePreHistoryHidden:
		return "toggle_pre_history_hidden"
	case *tg.ChannelAdminLogEventActionStartGroupCall:
		return "start_group_call"
	case *tg.ChannelAdminLogEventActionDiscardGroupCall:
		return "discard_group_call"
	case *tg.ChannelAdminLogEventActionParticipantMute:
		return "participant_mute"
	case *tg.ChannelAdminLogEventActionParticipantUnmute:
		return "participant_unmute"
	case *tg.ChannelAdminLogEventActionToggleGroupCallSetting:
		return "toggle_group_call_setting"
	case *tg.ChannelAdminLogEventActionParticipantJoinByInvite:
		return "join_by_invite"
	case *tg.ChannelAdminLogEventActionExportedInviteDelete:
		return "delete_exported_invite"
	case *tg.ChannelAdminLogEventActionExportedInviteRevoke:
		return "revoke_exported_invite"
	case *tg.ChannelAdminLogEventActionExportedInviteEdit:
		return "edit_exported_invite"
	case *tg.ChannelAdminLogEventActionParticipantVolume:
		return "participant_volume"
	case *tg.ChannelAdminLogEventActionChangeHistoryTTL:
		return "change_history_ttl"
	case *tg.ChannelAdminLogEventActionChangeLocation:
		return "change_location"
	}
	return "unknown"
}

// IsMessageEdit reports whether the event is a message edit.
func (e *AdminLogEvent) IsMessageEdit() bool {
	_, ok := e.Raw.Action.(*tg.ChannelAdminLogEventActionEditMessage)
	return ok
}

// IsMessageDelete reports whether the event is a message deletion.
func (e *AdminLogEvent) IsMessageDelete() bool {
	_, ok := e.Raw.Action.(*tg.ChannelAdminLogEventActionDeleteMessage)
	return ok
}

// IsUserBan reports whether the event is a user ban/restriction change.
func (e *AdminLogEvent) IsUserBan() bool {
	_, ok := e.Raw.Action.(*tg.ChannelAdminLogEventActionParticipantToggleBan)
	return ok
}

// IsUserKick reports whether a user was kicked (same as ban in TL terms).
func (e *AdminLogEvent) IsUserKick() bool {
	return e.IsUserBan()
}

// IsUserJoin reports whether a user joined the channel.
func (e *AdminLogEvent) IsUserJoin() bool {
	_, ok := e.Raw.Action.(*tg.ChannelAdminLogEventActionParticipantJoin)
	return ok
}

// IsUserLeave reports whether a user left the channel.
func (e *AdminLogEvent) IsUserLeave() bool {
	_, ok := e.Raw.Action.(*tg.ChannelAdminLogEventActionParticipantLeave)
	return ok
}

// IsTitleChange reports whether the channel title was changed.
func (e *AdminLogEvent) IsTitleChange() bool {
	_, ok := e.Raw.Action.(*tg.ChannelAdminLogEventActionChangeTitle)
	return ok
}

// IsPhotoChange reports whether the channel photo was changed.
func (e *AdminLogEvent) IsPhotoChange() bool {
	_, ok := e.Raw.Action.(*tg.ChannelAdminLogEventActionChangePhoto)
	return ok
}

// IsInviteLink reports whether the event concerns an invite link.
func (e *AdminLogEvent) IsInviteLink() bool {
	switch e.Raw.Action.(type) {
	case *tg.ChannelAdminLogEventActionExportedInviteDelete,
		*tg.ChannelAdminLogEventActionExportedInviteRevoke,
		*tg.ChannelAdminLogEventActionExportedInviteEdit,
		*tg.ChannelAdminLogEventActionParticipantJoinByInvite:
		return true
	}
	return false
}

// OldMessage returns the previous version of the message for edit events (nil otherwise).
func (e *AdminLogEvent) OldMessage() *tg.Message {
	if e.Raw == nil {
		return nil
	}
	if edit, ok := e.Raw.Action.(*tg.ChannelAdminLogEventActionEditMessage); ok {
		if m, ok := edit.PrevMessage.(*tg.Message); ok {
			return m
		}
	}
	return nil
}

// NewMessage returns the new version of the message for edit events, or the
// deleted message for delete events (nil otherwise).
func (e *AdminLogEvent) NewMessage() *tg.Message {
	if e.Raw == nil {
		return nil
	}
	switch a := e.Raw.Action.(type) {
	case *tg.ChannelAdminLogEventActionEditMessage:
		if m, ok := a.NewMessage.(*tg.Message); ok {
			return m
		}
	case *tg.ChannelAdminLogEventActionDeleteMessage:
		if m, ok := a.Message.(*tg.Message); ok {
			return m
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// GetAdminLogParams — parameters for fetching the admin log of a channel.
// ---------------------------------------------------------------------------

// GetAdminLogParams holds filter and pagination options for GetAdminLog.
type GetAdminLogParams struct {
	// ChannelID is the packed peer ID of the channel/supergroup.
	ChannelID int64

	// Query filters events by search term (empty = no filter).
	Query string

	// Limit is the maximum number of events to return (0 = use server default).
	Limit int

	// MinID is the minimum event ID for pagination (0 = no lower bound).
	MinID int64

	// MaxID is the maximum event ID for pagination (0 = no upper bound).
	MaxID int64

	// --- Event type filters (all false = return all event types) ---

	Join      bool
	Leave     bool
	Invite    bool
	Ban       bool
	Unban     bool
	Kick      bool
	Unkick    bool
	Promote   bool
	Demote    bool
	Info      bool
	Settings  bool
	Pinned    bool
	Edit      bool
	Delete    bool
	GroupCall bool
	Invites   bool
	Send      bool
	Forums    bool
}

// ToTLEventsFilter converts the boolean flags to a tg.ChannelAdminLogEventsFilter.
// Returns nil when no specific filters are requested (fetch all event types).
func (p *GetAdminLogParams) ToTLEventsFilter() *tg.ChannelAdminLogEventsFilter {
	hasAny := p.Join || p.Leave || p.Invite || p.Ban || p.Unban || p.Kick ||
		p.Unkick || p.Promote || p.Demote || p.Info || p.Settings || p.Pinned ||
		p.Edit || p.Delete || p.GroupCall || p.Invites || p.Send || p.Forums

	if !hasAny {
		return nil
	}

	return &tg.ChannelAdminLogEventsFilter{
		Join:      p.Join,
		Leave:     p.Leave,
		Invite:    p.Invite,
		Ban:       p.Ban,
		Unban:     p.Unban,
		Kick:      p.Kick,
		Unkick:    p.Unkick,
		Promote:   p.Promote,
		Demote:    p.Demote,
		Info:      p.Info,
		Settings:  p.Settings,
		Pinned:    p.Pinned,
		Edit:      p.Edit,
		Delete:    p.Delete,
		GroupCall: p.GroupCall,
		Invites:   p.Invites,
		Send:      p.Send,
		Forums:    p.Forums,
	}
}
