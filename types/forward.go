package types

import (
	"time"

	"github.com/gotd/td/tg"
)

// MCUBForward wraps forward-origin info from a forwarded Telegram message.
// It mirrors Telethon's tl/custom/forward.Forward class.
type MCUBForward struct {
	// Raw is the underlying gotd/td MessageFwdHeader.
	Raw *tg.MessageFwdHeader
}

// NewForward creates a new MCUBForward from a MessageFwdHeader.
// Returns nil when fwd is nil.
func NewForward(fwd *tg.MessageFwdHeader) *MCUBForward {
	if fwd == nil {
		return nil
	}
	return &MCUBForward{Raw: fwd}
}

// OriginalDate returns the UTC timestamp of the original message.
func (f *MCUBForward) OriginalDate() time.Time {
	if f.Raw == nil {
		return time.Time{}
	}
	return time.Unix(int64(f.Raw.Date), 0)
}

// FromID returns the original sender's user ID (non-zero when forwarded from a user).
func (f *MCUBForward) FromID() int64 {
	if f.Raw == nil {
		return 0
	}
	fromID, ok := f.Raw.GetFromID()
	if !ok {
		return 0
	}
	if p, ok := fromID.(*tg.PeerUser); ok {
		return int64(p.UserID)
	}
	return 0
}

// FromChatID returns the original channel/group ID (non-zero when forwarded from a channel or group).
func (f *MCUBForward) FromChatID() int64 {
	if f.Raw == nil {
		return 0
	}
	fromID, ok := f.Raw.GetFromID()
	if !ok {
		return 0
	}
	switch p := fromID.(type) {
	case *tg.PeerChannel:
		return -1000000000000 - int64(p.ChannelID)
	case *tg.PeerChat:
		return -int64(p.ChatID)
	}
	return 0
}

// FromName returns the sender name for hidden forwards (where the sender hid their identity).
func (f *MCUBForward) FromName() string {
	if f.Raw == nil {
		return ""
	}
	name, _ := f.Raw.GetFromName()
	return name
}

// PostAuthor returns the channel post author signature, if present.
func (f *MCUBForward) PostAuthor() string {
	if f.Raw == nil {
		return ""
	}
	author, _ := f.Raw.GetPostAuthor()
	return author
}

// OriginalMsgID returns the ID of the original forwarded message (0 if not available).
func (f *MCUBForward) OriginalMsgID() int {
	if f.Raw == nil {
		return 0
	}
	id, _ := f.Raw.GetChannelPost()
	return id
}

// IsChannel reports whether the forward originated from a channel.
func (f *MCUBForward) IsChannel() bool {
	if f.Raw == nil {
		return false
	}
	fromID, ok := f.Raw.GetFromID()
	if !ok {
		return false
	}
	_, ok = fromID.(*tg.PeerChannel)
	return ok
}

// IsUser reports whether the forward originated from a user.
func (f *MCUBForward) IsUser() bool {
	if f.Raw == nil {
		return false
	}
	fromID, ok := f.Raw.GetFromID()
	if !ok {
		return false
	}
	_, ok = fromID.(*tg.PeerUser)
	return ok
}

// IsHidden reports whether the original sender chose to hide their identity.
func (f *MCUBForward) IsHidden() bool {
	if f.Raw == nil {
		return false
	}
	// When the sender hid their identity, FromID is absent but FromName may be set.
	_, hasFromID := f.Raw.GetFromID()
	fromName, _ := f.Raw.GetFromName()
	return !hasFromID && fromName != ""
}

// SavedFromID returns the peer ID this message was forwarded from in Saved Messages (0 if N/A).
func (f *MCUBForward) SavedFromID() int64 {
	if f.Raw == nil {
		return 0
	}
	savedPeer, ok := f.Raw.GetSavedFromPeer()
	if !ok {
		return 0
	}
	switch p := savedPeer.(type) {
	case *tg.PeerUser:
		return int64(p.UserID)
	case *tg.PeerChat:
		return -int64(p.ChatID)
	case *tg.PeerChannel:
		return -1000000000000 - int64(p.ChannelID)
	}
	return 0
}
