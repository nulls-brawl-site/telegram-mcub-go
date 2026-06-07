package types

import "github.com/gotd/td/tg"

// ParticipantPermissions wraps chat/channel participant restrictions and admin rights.
// It mirrors Telethon's tl/custom/participantpermissions.ParticipantPermissions class.
type ParticipantPermissions struct {
	// Raw is the underlying participant (tg.ChannelParticipantClass or tg.ChatParticipantClass).
	Raw interface{}

	// isChat indicates this is a regular group (not a channel/supergroup).
	isChat bool
}

// NewParticipantPermissions creates a ParticipantPermissions from a raw participant.
// Pass isChat=true for regular groups, false for channels/supergroups.
func NewParticipantPermissions(p interface{}, isChat bool) *ParticipantPermissions {
	return &ParticipantPermissions{Raw: p, isChat: isChat}
}

// IsCreator reports whether the participant is the group/channel creator.
func (pp *ParticipantPermissions) IsCreator() bool {
	switch pp.Raw.(type) {
	case *tg.ChannelParticipantCreator, *tg.ChatParticipantCreator:
		return true
	}
	return false
}

// IsAdmin reports whether the participant is an admin (includes creator).
func (pp *ParticipantPermissions) IsAdmin() bool {
	if pp.IsCreator() {
		return true
	}
	switch pp.Raw.(type) {
	case *tg.ChannelParticipantAdmin, *tg.ChatParticipantAdmin:
		return true
	}
	return false
}

// IsBanned reports whether the participant is banned.
func (pp *ParticipantPermissions) IsBanned() bool {
	_, ok := pp.Raw.(*tg.ChannelParticipantBanned)
	return ok
}

// IsRestricted reports whether the participant has any non-zero restrictions applied.
func (pp *ParticipantPermissions) IsRestricted() bool {
	if banned, ok := pp.Raw.(*tg.ChannelParticipantBanned); ok {
		rights := banned.BannedRights
		return rights.SendMessages || rights.SendMedia || rights.SendStickers ||
			rights.SendGifs || rights.SendGames || rights.SendInline ||
			rights.EmbedLinks || rights.SendPolls || rights.ChangeInfo ||
			rights.InviteUsers || rights.PinMessages
	}
	return false
}

// bannedRights returns the ChatBannedRights for a banned participant, or nil.
func (pp *ParticipantPermissions) bannedRights() *tg.ChatBannedRights {
	if banned, ok := pp.Raw.(*tg.ChannelParticipantBanned); ok {
		return &banned.BannedRights
	}
	return nil
}

// adminRights returns the ChatAdminRights for an admin participant, or nil.
func (pp *ParticipantPermissions) adminRights() *tg.ChatAdminRights {
	switch v := pp.Raw.(type) {
	case *tg.ChannelParticipantAdmin:
		return &v.AdminRights
	case *tg.ChannelParticipantCreator:
		return &v.AdminRights
	}
	return nil
}

// CanSendMessages reports whether the participant may send messages.
func (pp *ParticipantPermissions) CanSendMessages() bool {
	if pp.IsAdmin() || pp.isChat {
		return !pp.IsBanned()
	}
	if r := pp.bannedRights(); r != nil {
		return !r.SendMessages
	}
	return true
}

// CanSendMedia reports whether the participant may send media.
func (pp *ParticipantPermissions) CanSendMedia() bool {
	if pp.IsAdmin() || pp.isChat {
		return !pp.IsBanned()
	}
	if r := pp.bannedRights(); r != nil {
		return !r.SendMedia
	}
	return true
}

// CanSendStickers reports whether the participant may send stickers.
func (pp *ParticipantPermissions) CanSendStickers() bool {
	if pp.IsAdmin() || pp.isChat {
		return !pp.IsBanned()
	}
	if r := pp.bannedRights(); r != nil {
		return !r.SendStickers
	}
	return true
}

// CanSendPolls reports whether the participant may send polls.
func (pp *ParticipantPermissions) CanSendPolls() bool {
	if pp.IsAdmin() || pp.isChat {
		return !pp.IsBanned()
	}
	if r := pp.bannedRights(); r != nil {
		return !r.SendPolls
	}
	return true
}

// CanAddUsers reports whether the participant may add new users.
func (pp *ParticipantPermissions) CanAddUsers() bool {
	if !pp.IsAdmin() {
		return false
	}
	if pp.isChat {
		return true
	}
	if r := pp.adminRights(); r != nil {
		return r.InviteUsers
	}
	return false
}

// CanPinMessages reports whether the participant may pin messages.
func (pp *ParticipantPermissions) CanPinMessages() bool {
	if !pp.IsAdmin() {
		return false
	}
	if pp.isChat {
		return true
	}
	if r := pp.adminRights(); r != nil {
		return r.PinMessages
	}
	return false
}

// CanChangeInfo reports whether the participant may change chat info.
func (pp *ParticipantPermissions) CanChangeInfo() bool {
	if !pp.IsAdmin() {
		return false
	}
	if pp.isChat {
		return true
	}
	if r := pp.adminRights(); r != nil {
		return r.ChangeInfo
	}
	return false
}

// CanDeleteMessages reports whether the participant may delete other users' messages.
func (pp *ParticipantPermissions) CanDeleteMessages() bool {
	if !pp.IsAdmin() {
		return false
	}
	if pp.isChat {
		return true
	}
	if r := pp.adminRights(); r != nil {
		return r.DeleteMessages
	}
	return false
}

// CanBanUsers reports whether the participant may ban other users.
func (pp *ParticipantPermissions) CanBanUsers() bool {
	if !pp.IsAdmin() {
		return false
	}
	if pp.isChat {
		return pp.IsCreator()
	}
	if r := pp.adminRights(); r != nil {
		return r.BanUsers
	}
	return false
}

// CanInviteUsers reports whether the participant may invite new users via links.
func (pp *ParticipantPermissions) CanInviteUsers() bool {
	if !pp.IsAdmin() {
		return false
	}
	if pp.isChat {
		return true
	}
	if r := pp.adminRights(); r != nil {
		return r.InviteUsers
	}
	return false
}

// CanManageCall reports whether the participant may manage voice/video calls.
func (pp *ParticipantPermissions) CanManageCall() bool {
	if !pp.IsAdmin() {
		return false
	}
	if pp.isChat {
		return true
	}
	if r := pp.adminRights(); r != nil {
		return r.ManageCall
	}
	return false
}

// Restrictions returns the ChatBannedRights for a banned participant, or nil.
func (pp *ParticipantPermissions) Restrictions() *tg.ChatBannedRights {
	return pp.bannedRights()
}
