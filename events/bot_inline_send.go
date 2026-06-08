package events

import (
	"context"

	"github.com/gotd/td/tg"
)

// BotInlineSend fires when someone selects and sends an inline result from your bot.
// This corresponds to Telegram's updateBotInlineSend.
type BotInlineSend struct {
	// Raw is the underlying Telegram update.
	Raw *tg.UpdateBotInlineSend

	// UserID is the ID of the user who sent the inline result.
	UserID int64

	// Query is the query text that was used to obtain the result.
	Query string

	// ResultID is the unique identifier of the chosen inline result.
	ResultID string

	// MsgID is the identifier of the sent inline message, if an inline keyboard
	// was attached. Nil when no keyboard was present.
	MsgID tg.InputBotInlineMessageIDClass
}

// EventType implements Event.
func (e *BotInlineSend) EventType() string { return "BotInlineSend" }

// BotInlineSendFromUpdate constructs a BotInlineSend event from a
// tg.UpdateBotInlineSend update, returning (event, true) on success.
func BotInlineSendFromUpdate(_ context.Context, u tg.UpdateClass) (*BotInlineSend, bool) {
	raw, ok := u.(*tg.UpdateBotInlineSend)
	if !ok {
		return nil, false
	}
	ev := &BotInlineSend{
		Raw:      raw,
		UserID:   raw.UserID,
		Query:    raw.Query,
		ResultID: raw.ID,
	}
	if msgID, ok := raw.GetMsgID(); ok {
		ev.MsgID = msgID
	}
	return ev, true
}

// BotInlineSendFilter returns a Filter that accepts only BotInlineSend events.
func BotInlineSendFilter() Filter {
	return func(e Event) bool {
		_, ok := e.(*BotInlineSend)
		return ok
	}
}
