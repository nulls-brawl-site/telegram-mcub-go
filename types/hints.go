// Package types defines MCUB-specific Telegram types and type aliases used
// across the telegram-mcub-go library.
package types

import (
	"fmt"

	"github.com/gotd/td/tg"
)

// ---------------------------------------------------------------------------
// Hint types — mirrors of Telethon's telethon/hints.py
// ---------------------------------------------------------------------------

// EntityLike is satisfied by any value that can represent a Telegram entity:
// an integer user/chat/channel ID, a username string, or a concrete entity
// object (*tg.User, *tg.Chat, *tg.Channel, or an InputPeer variant).
type EntityLike interface{}

// MessageLike is satisfied by a message ID (int) or a concrete message object
// (*tg.Message, *tg.MessageService, *tg.MessageEmpty).
type MessageLike interface{}

// ButtonLike is satisfied by a keyboard button value.
type ButtonLike interface{}

// MarkupLike is satisfied by an inline keyboard or reply keyboard markup.
type MarkupLike interface{}

// ---------------------------------------------------------------------------
// Entity resolution helper
// ---------------------------------------------------------------------------

// ResolveEntityLike resolves an EntityLike value to its canonical int64 peer ID.
//
// The Bot API convention is used:
//   - User IDs are positive.
//   - Chat IDs are negative  (−chat_id).
//   - Channel/supergroup IDs are −1_000_000_000_000 − channel_id.
//
// For plain string values (usernames) the function returns (0, nil) — the
// caller must resolve the username via the Telegram API.
//
// Mirrors Telethon's hints.py entity coercion logic.
func ResolveEntityLike(e EntityLike) (int64, error) {
	switch v := e.(type) {
	case int:
		return int64(v), nil
	case int32:
		return int64(v), nil
	case int64:
		return v, nil
	case string:
		// Username — cannot resolve without a network call; return zero and
		// leave resolution to the caller.
		return 0, nil

	// Concrete entity types.
	case *tg.User:
		return v.ID, nil
	case *tg.Chat:
		return -v.ID, nil
	case *tg.ChatForbidden:
		return -v.ID, nil
	case *tg.Channel:
		return -1_000_000_000_000 - v.ID, nil
	case *tg.ChannelForbidden:
		return -1_000_000_000_000 - v.ID, nil

	// Peer types.
	case *tg.PeerUser:
		return v.UserID, nil
	case *tg.PeerChat:
		return -v.ChatID, nil
	case *tg.PeerChannel:
		return -1_000_000_000_000 - v.ChannelID, nil

	// InputPeer types.
	case *tg.InputPeerUser:
		return v.UserID, nil
	case *tg.InputPeerSelf:
		// Cannot resolve without knowing the local user's ID; return zero.
		return 0, nil
	case *tg.InputPeerChat:
		return -v.ChatID, nil
	case *tg.InputPeerChannel:
		return -1_000_000_000_000 - v.ChannelID, nil
	case *tg.InputPeerEmpty:
		return 0, nil
	}

	return 0, fmt.Errorf("types: cannot resolve EntityLike of type %T", e)
}
