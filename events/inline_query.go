package events

import (
	"context"

	"github.com/gotd/td/tg"
)

// InlineQuery is fired when a user types "@bot <query>" in any chat.
type InlineQuery struct {
	// Raw is the underlying Telegram update.
	Raw *tg.UpdateBotInlineQuery

	// Query is the text the user typed after the bot mention.
	Query string

	// UserID is the ID of the user who typed the query.
	UserID int64

	// Offset is the pagination offset string from the previous answer.
	Offset string
}

// EventType implements Event.
func (e *InlineQuery) EventType() string { return "InlineQuery" }

// InlineQueryFromUpdate constructs an InlineQuery event from a tg.UpdateBotInlineQuery.
func InlineQueryFromUpdate(ctx context.Context, u tg.UpdateClass) (*InlineQuery, bool) {
	_ = ctx
	raw, ok := u.(*tg.UpdateBotInlineQuery)
	if !ok {
		return nil, false
	}
	return &InlineQuery{
		Raw:    raw,
		Query:  raw.Query,
		UserID: raw.UserID,
		Offset: raw.Offset,
	}, true
}

// InlineQueryFilter returns a Filter that accepts only InlineQuery events.
func InlineQueryFilter() Filter {
	return func(e Event) bool {
		_, ok := e.(*InlineQuery)
		return ok
	}
}
