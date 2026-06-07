package events

import (
	"fmt"

	"github.com/gotd/td/tg"
)

// Raw wraps any Telegram update that was not matched by a more specific event
// builder.  Equivalent to Telethon's Raw event.
type Raw struct {
	// Update is the original gotd/td update object.
	Update tg.UpdateClass
}

// EventType implements Event.
func (e *Raw) EventType() string { return "Raw" }

// TypeName returns the Go type name of the underlying update.
func (e *Raw) TypeName() string {
	if e.Update == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%T", e.Update)
}

// RawFilter returns a Filter that passes Raw events whose underlying update
// matches the given Go type name string (e.g. "*tg.UpdateNewMessage").
// Pass an empty string to match all Raw events.
func RawFilter(updateType string) Filter {
	return func(e Event) bool {
		r, ok := e.(*Raw)
		if !ok {
			return false
		}
		if updateType == "" {
			return true
		}
		return r.TypeName() == updateType
	}
}
