package types

import (
	"time"

	"github.com/gotd/td/tg"
)

// MCUBDraft wraps a tg.DraftMessage with parsed convenience fields.
// Ported from Telethon's tl/custom/draft.py.
type MCUBDraft struct {
	// Raw is the underlying tg.DraftMessage (may be nil for empty drafts).
	Raw *tg.DraftMessage

	// ChatID is the signed peer ID of the chat this draft belongs to.
	ChatID int64

	// Text is the draft message text.
	Text string

	// ReplyTo is the message ID being replied to (0 if none).
	ReplyTo int

	// Date is the timestamp when the draft was last saved.
	Date time.Time

	// LinkPreview indicates whether a web-page preview is enabled.
	LinkPreview bool
}

// IsEmpty reports whether the draft contains no text.
func (d *MCUBDraft) IsEmpty() bool {
	return d == nil || d.Text == ""
}

// NewMCUBDraft wraps a tg.DraftMessage, resolving its fields.
// Pass chatID as the signed peer ID of the owning dialog.
func NewMCUBDraft(raw *tg.DraftMessage, chatID int64) *MCUBDraft {
	d := &MCUBDraft{ChatID: chatID}

	if raw == nil {
		return d
	}

	d.Raw = raw
	d.Text = raw.Message
	d.Date = time.Unix(int64(raw.Date), 0)
	d.LinkPreview = !raw.NoWebpage

	if replyTo, ok := raw.GetReplyTo(); ok {
		if rt, ok2 := replyTo.(*tg.InputReplyToMessage); ok2 {
			d.ReplyTo = rt.ReplyToMsgID
		}
	}

	return d
}
