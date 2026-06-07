package events

import (
	"github.com/gotd/td/tg"
)

// Album groups multiple media messages that were sent together as an album.
// Telegram sends each photo/video in an album as a separate message sharing
// the same GroupedID; this event collects them.
type Album struct {
	// Messages are the individual messages that make up the album.
	Messages []*tg.Message

	// GroupID is the shared grouped_id that links these messages.
	GroupID int64

	// ChatID is the numeric peer ID of the chat the album was sent to.
	ChatID int64
}

// EventType implements Event.
func (e *Album) EventType() string { return "Album" }

// AlbumFilter returns a Filter that accepts only Album events.
func AlbumFilter() Filter {
	return func(e Event) bool {
		_, ok := e.(*Album)
		return ok
	}
}
