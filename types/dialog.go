package types

import (
	"time"

	"github.com/gotd/td/tg"
)

// MCUBDialog wraps a tg.Dialog with resolved entity and top-message data.
// Ported from Telethon's tl/custom/dialog.py.
type MCUBDialog struct {
	// Raw is the underlying tg.Dialog.
	Raw *tg.Dialog

	// EntityRaw holds the resolved entity (*tg.User, *tg.Chat, or *tg.Channel).
	// Use the typed helpers (AsUser, AsChat, AsChannel) to access it.
	EntityRaw interface{}

	// TopMessage is the most recent message in this dialog, or nil.
	TopMessage *tg.Message

	// Draft holds the current draft, or nil.
	Draft *MCUBDraft

	// ID is the signed, unique peer ID (matches bot-API chat IDs).
	ID int64

	// Name is the display name (title for groups/channels, full name for users).
	Name string

	// Username is the @username, or "" if none.
	Username string

	// Chat type flags.
	IsUser      bool
	IsGroup     bool
	IsChannel   bool
	IsMegagroup bool

	// Read state.
	UnreadCount    int
	UnreadMentions int

	// Dialog state.
	Pinned   bool
	Archived bool
	Muted    bool
	FolderID int
}

// Title is an alias for Name, matching Telethon's property name.
func (d *MCUBDialog) Title() string { return d.Name }

// Entity returns the resolved entity (same as EntityRaw).
func (d *MCUBDialog) Entity() interface{} { return d.EntityRaw }

// AsUser returns the entity cast to *tg.User, or nil if it isn't one.
func (d *MCUBDialog) AsUser() *tg.User {
	u, _ := d.EntityRaw.(*tg.User)
	return u
}

// AsChat returns the entity cast to *tg.Chat, or nil if it isn't one.
func (d *MCUBDialog) AsChat() *tg.Chat {
	c, _ := d.EntityRaw.(*tg.Chat)
	return c
}

// AsChannel returns the entity cast to *tg.Channel, or nil if it isn't one.
func (d *MCUBDialog) AsChannel() *tg.Channel {
	ch, _ := d.EntityRaw.(*tg.Channel)
	return ch
}

// LastMessageDate returns the date of the top message, or zero time if absent.
func (d *MCUBDialog) LastMessageDate() time.Time {
	if d.TopMessage == nil {
		return time.Time{}
	}
	return time.Unix(int64(d.TopMessage.Date), 0)
}

// NewMCUBDialog builds a MCUBDialog from a raw tg.Dialog plus resolved data.
//
//   - entity: *tg.User, *tg.Chat, or *tg.Channel
//   - topMsg: the resolved top message (may be nil)
//   - draft: the resolved draft (may be nil)
//   - muted: whether notifications are silenced
func NewMCUBDialog(raw *tg.Dialog, entity interface{}, topMsg *tg.Message, draft *MCUBDraft, muted bool) *MCUBDialog {
	d := &MCUBDialog{
		Raw:        raw,
		EntityRaw:  entity,
		TopMessage: topMsg,
		Draft:      draft,
		Muted:      muted,
	}

	if raw != nil {
		d.Pinned = raw.Pinned
		d.UnreadCount = raw.UnreadCount
		d.UnreadMentions = raw.UnreadMentionsCount
		if folderID, ok := raw.GetFolderID(); ok {
			d.FolderID = folderID
			d.Archived = true
		}
	}

	switch e := entity.(type) {
	case *tg.User:
		d.IsUser = true
		d.ID = int64(e.ID)
		d.Name = userDisplayName(e)
		if uname, ok := e.GetUsername(); ok {
			d.Username = uname
		}

	case *tg.Chat:
		d.IsGroup = true
		d.ID = -int64(e.ID)
		d.Name = e.Title

	case *tg.ChatForbidden:
		d.IsGroup = true
		d.ID = -int64(e.ID)
		d.Name = e.Title

	case *tg.Channel:
		d.IsChannel = true
		d.IsMegagroup = e.Megagroup
		d.IsGroup = e.Megagroup
		d.ID = -1000000000000 - int64(e.ID)
		d.Name = e.Title
		if uname, ok := e.GetUsername(); ok {
			d.Username = uname
		}

	case *tg.ChannelForbidden:
		d.IsChannel = true
		d.ID = -1000000000000 - int64(e.ID)
		d.Name = e.Title
	}

	return d
}

// userDisplayName returns "FirstName LastName" for a User.
func userDisplayName(u *tg.User) string {
	first, _ := u.GetFirstName()
	last, _ := u.GetLastName()
	if last == "" {
		return first
	}
	if first == "" {
		return last
	}
	return first + " " + last
}
