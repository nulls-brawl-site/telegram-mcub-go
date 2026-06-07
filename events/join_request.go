package events

import (
	"context"
	"time"

	"github.com/gotd/td/tg"
)

// JoinRequest is fired when a user requests to join a group/channel
// that requires admin approval.
type JoinRequest struct {
	// ChatID is the chat the user wants to join.
	ChatID int64

	// UserID is the requesting user's ID.
	UserID int64

	// Date is when the request was submitted.
	Date time.Time

	// About is the optional message the user sent with the request.
	About string

	// InviteLink is the invite link that was used (may be nil).
	InviteLink *tg.ChatInviteExported

	// Approved is true if the client has already approved the request.
	Approved bool
}

// EventType implements Event.
func (e *JoinRequest) EventType() string { return "JoinRequest" }

// JoinRequestFromUpdate constructs a JoinRequest event from
// tg.UpdatePendingJoinRequests (MCUB-specific handling).
func JoinRequestFromUpdate(ctx context.Context, u tg.UpdateClass) (*JoinRequest, bool) {
	_ = ctx
	upd, ok := u.(*tg.UpdateBotChatInviteRequester)
	if !ok {
		return nil, false
	}

	ev := &JoinRequest{
		UserID: int64(upd.UserID),
		Date:   time.Unix(int64(upd.Date), 0),
		About:  upd.About,
	}

	// Resolve chat ID from the peer.
	switch p := upd.Peer.(type) {
	case *tg.PeerChat:
		ev.ChatID = -int64(p.ChatID)
	case *tg.PeerChannel:
		ev.ChatID = -1000000000000 - int64(p.ChannelID)
	}

	if upd.Invite != nil {
		if link, ok := upd.Invite.(*tg.ChatInviteExported); ok {
			ev.InviteLink = link
		}
	}

	return ev, true
}

// JoinRequestFilter returns a Filter that accepts only JoinRequest events.
func JoinRequestFilter() Filter {
	return func(e Event) bool {
		_, ok := e.(*JoinRequest)
		return ok
	}
}

// JoinRequestForChatFilter returns a Filter that accepts JoinRequest events
// only for the given chat ID.
func JoinRequestForChatFilter(chatID int64) Filter {
	return func(e Event) bool {
		jr, ok := e.(*JoinRequest)
		return ok && jr.ChatID == chatID
	}
}
