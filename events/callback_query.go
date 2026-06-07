package events

import (
	"context"
	"fmt"

	"github.com/gotd/td/tg"
)

// CallbackQueryAnswerer is the interface used by CallbackQuery to send answers.
// Implement it with a real API call; inject via CallbackQuery.Answerer.
type CallbackQueryAnswerer interface {
	MessagesSetBotCallbackAnswer(ctx context.Context, req *tg.MessagesSetBotCallbackAnswerRequest) (bool, error)
}

// CallbackQuery is fired when a user clicks an inline keyboard button.
type CallbackQuery struct {
	// Raw is the underlying Telegram update.
	Raw *tg.UpdateBotCallbackQuery

	// Data is the callback data bytes attached to the button.
	Data []byte

	// ChatID is the numeric peer ID of the chat containing the message.
	ChatID int64

	// UserID is the ID of the user who clicked the button.
	UserID int64

	// MsgID is the message ID that contains the inline keyboard.
	MsgID int

	// GameShortName is set when the button is a game button.
	GameShortName string

	// Answerer is used internally to send callback answers.
	// It is set by the dispatcher when the event is created.
	Answerer CallbackQueryAnswerer
}

// EventType implements Event.
func (e *CallbackQuery) EventType() string { return "CallbackQuery" }

// Answer responds to the callback query with an optional popup text.
// If alert is true, Telegram shows a modal alert instead of a notification.
func (e *CallbackQuery) Answer(ctx context.Context, text string, alert bool) error {
	if e.Answerer == nil {
		return fmt.Errorf("CallbackQuery.Answerer is not set")
	}
	_, err := e.Answerer.MessagesSetBotCallbackAnswer(ctx, &tg.MessagesSetBotCallbackAnswerRequest{
		QueryID:   e.Raw.QueryID,
		Message:   text,
		Alert:     alert,
		CacheTime: 0,
	})
	return err
}

// CallbackQueryFromUpdate constructs a CallbackQuery event from a tg.UpdateBotCallbackQuery.
func CallbackQueryFromUpdate(ctx context.Context, u tg.UpdateClass) (*CallbackQuery, bool) {
	_ = ctx
	raw, ok := u.(*tg.UpdateBotCallbackQuery)
	if !ok {
		return nil, false
	}

	ev := &CallbackQuery{
		Raw:    raw,
		Data:   raw.Data,
		UserID: raw.UserID,
		MsgID:  raw.MsgID,
	}
	ev.GameShortName, _ = raw.GetGameShortName()

	// Resolve ChatID from the peer.
	switch p := raw.Peer.(type) {
	case *tg.PeerUser:
		ev.ChatID = int64(p.UserID)
	case *tg.PeerChat:
		ev.ChatID = -int64(p.ChatID)
	case *tg.PeerChannel:
		ev.ChatID = -1000000000000 - int64(p.ChannelID)
	}

	return ev, true
}

// CallbackQueryFilter returns a Filter that accepts only CallbackQuery events.
func CallbackQueryFilter() Filter {
	return func(e Event) bool {
		_, ok := e.(*CallbackQuery)
		return ok
	}
}
