package events

import (
	"regexp"
	"strings"

	"github.com/gotd/td/tg"
)

// OutgoingFilter returns a Filter that accepts only outgoing NewMessage events.
func OutgoingFilter() Filter {
	return func(e Event) bool {
		nm, ok := e.(*NewMessage)
		if !ok {
			return false
		}
		return nm.IsOutgoing
	}
}

// IncomingFilter returns a Filter that accepts only incoming NewMessage events.
func IncomingFilter() Filter {
	return func(e Event) bool {
		nm, ok := e.(*NewMessage)
		if !ok {
			return false
		}
		return !nm.IsOutgoing
	}
}

// ChatsFilter accepts events only from the given chat IDs.
func ChatsFilter(chatIDs ...int64) Filter {
	set := make(map[int64]bool, len(chatIDs))
	for _, id := range chatIDs {
		set[id] = true
	}
	return func(e Event) bool {
		switch ev := e.(type) {
		case *NewMessage:
			return set[ev.PeerID]
		case *MessageEdited:
			return set[ev.PeerID]
		default:
			return false
		}
	}
}

// FromUsersFilter accepts events only from the given user IDs.
func FromUsersFilter(userIDs ...int64) Filter {
	set := make(map[int64]bool, len(userIDs))
	for _, id := range userIDs {
		set[id] = true
	}
	return func(e Event) bool {
		switch ev := e.(type) {
		case *NewMessage:
			return set[ev.SenderID]
		default:
			return false
		}
	}
}

// PrivateFilter accepts only private NewMessage events.
func PrivateFilter() Filter {
	return func(e Event) bool {
		nm, ok := e.(*NewMessage)
		return ok && nm.IsPrivate
	}
}

// GroupFilter accepts only group NewMessage events.
func GroupFilter() Filter {
	return func(e Event) bool {
		nm, ok := e.(*NewMessage)
		return ok && nm.IsGroup
	}
}

// ChannelFilter accepts only channel NewMessage events.
func ChannelFilter() Filter {
	return func(e Event) bool {
		nm, ok := e.(*NewMessage)
		return ok && nm.IsChannel
	}
}

// RegexFilter accepts NewMessage events whose text matches the given regular expression.
// It panics if pattern is not a valid regular expression.
func RegexFilter(pattern string) Filter {
	re := regexp.MustCompile(pattern)
	return func(e Event) bool {
		nm, ok := e.(*NewMessage)
		if !ok {
			return false
		}
		return re.MatchString(nm.Text())
	}
}

// CommandFilter accepts NewMessage events whose text equals prefix+command or
// starts with prefix+command followed by a space.
func CommandFilter(prefix, command string) Filter {
	target := prefix + command
	return func(e Event) bool {
		nm, ok := e.(*NewMessage)
		if !ok {
			return false
		}
		text := nm.Text()
		return text == target || strings.HasPrefix(text, target+" ")
	}
}

// HasMediaFilter accepts NewMessage events that contain any media attachment.
func HasMediaFilter() Filter {
	return func(e Event) bool {
		nm, ok := e.(*NewMessage)
		if !ok {
			return false
		}
		return nm.Raw != nil && nm.Raw.Media != nil
	}
}

// MediaTypeFilter accepts NewMessage events whose media matches one of the
// given type strings: "photo", "document", "video", "audio", "voice",
// "sticker", "location", "contact", "poll".
func MediaTypeFilter(mediaTypes ...string) Filter {
	set := make(map[string]bool, len(mediaTypes))
	for _, t := range mediaTypes {
		set[t] = true
	}
	return func(e Event) bool {
		nm, ok := e.(*NewMessage)
		if !ok || nm.Raw == nil || nm.Raw.Media == nil {
			return false
		}
		switch nm.Raw.Media.(type) {
		case *tg.MessageMediaPhoto:
			return set["photo"]
		case *tg.MessageMediaDocument:
			return set["document"] || set["video"] || set["audio"] || set["voice"] || set["sticker"]
		case *tg.MessageMediaGeo:
			return set["location"]
		case *tg.MessageMediaContact:
			return set["contact"]
		case *tg.MessageMediaPoll:
			return set["poll"]
		}
		return false
	}
}

// AndFilter returns a Filter that passes only when ALL provided filters pass.
func AndFilter(filters ...Filter) Filter {
	return func(e Event) bool {
		for _, f := range filters {
			if !f(e) {
				return false
			}
		}
		return true
	}
}

// OrFilter returns a Filter that passes when ANY of the provided filters pass.
func OrFilter(filters ...Filter) Filter {
	return func(e Event) bool {
		for _, f := range filters {
			if f(e) {
				return true
			}
		}
		return false
	}
}

// NotFilter returns a Filter that negates the given filter.
func NotFilter(f Filter) Filter {
	return func(e Event) bool {
		return !f(e)
	}
}
