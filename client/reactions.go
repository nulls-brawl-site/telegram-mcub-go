package client

import (
	"context"
	"fmt"

	"github.com/gotd/td/tg"
)

// ReactionEmoji represents a standard emoji reaction.
type ReactionEmoji struct {
	Emoticon string
}

// ReactionCustomEmoji represents a custom emoji reaction.
type ReactionCustomEmoji struct {
	DocumentID int64
}

// ReactionClass is the interface implemented by ReactionEmoji and ReactionCustomEmoji.
type ReactionClass interface {
	toTL() tg.ReactionClass
}

func (r ReactionEmoji) toTL() tg.ReactionClass {
	return &tg.ReactionEmoji{Emoticon: r.Emoticon}
}

func (r ReactionCustomEmoji) toTL() tg.ReactionClass {
	return &tg.ReactionCustomEmoji{DocumentID: r.DocumentID}
}

// SendReactionParams holds parameters for sending a reaction.
type SendReactionParams struct {
	// PeerID is the numeric peer ID of the chat.
	PeerID int64

	// MessageID is the ID of the message to react to.
	MessageID int

	// Reactions is the list of reactions to send.
	// Pass an empty slice to remove all reactions.
	Reactions []ReactionClass

	// Big enables the big (animated) reaction effect.
	Big bool

	// AddToRecent adds the emoji to the user's recent reactions.
	AddToRecent bool
}

// SendReaction sends one or more reactions to a message.
func (c *MCUBClient) SendReaction(ctx context.Context, params SendReactionParams) error {
	peer, err := c.resolvePeer(ctx, params.PeerID)
	if err != nil {
		return fmt.Errorf("resolve peer: %w", err)
	}

	tlReactions := make([]tg.ReactionClass, 0, len(params.Reactions))
	for _, r := range params.Reactions {
		tlReactions = append(tlReactions, r.toTL())
	}

	req := &tg.MessagesSendReactionRequest{
		Peer:        peer,
		MsgID:       params.MessageID,
		Reaction:    tlReactions,
		Big:         params.Big,
		AddToRecent: params.AddToRecent,
	}

	_, err = c.client.API().MessagesSendReaction(ctx, req)
	if err != nil {
		return fmt.Errorf("send reaction: %w", err)
	}
	return nil
}

// MessageReactionSender holds information about a user who reacted.
type MessageReactionSender struct {
	// PeerID is the peer ID of the reactor (positive = user).
	PeerID int64

	// Reaction is the reaction they sent.
	Reaction ReactionClass

	// Date is the Unix timestamp of the reaction.
	Date int
}

// GetMessageReactionsList retrieves the list of peers who reacted to a message.
func (c *MCUBClient) GetMessageReactionsList(
	ctx context.Context,
	peerID int64,
	messageID int,
	reaction ReactionClass,
	limit int,
) ([]MessageReactionSender, error) {
	peer, err := c.resolvePeer(ctx, peerID)
	if err != nil {
		return nil, err
	}

	if limit <= 0 {
		limit = 100
	}

	req := &tg.MessagesGetMessageReactionsListRequest{
		Peer:  peer,
		ID:    messageID,
		Limit: limit,
	}
	if reaction != nil {
		req.SetReaction(reaction.toTL())
	}

	result, err := c.client.API().MessagesGetMessageReactionsList(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("get reactions list: %w", err)
	}

	out := make([]MessageReactionSender, 0, len(result.Reactions))
	for _, r := range result.Reactions {
		sender := MessageReactionSender{Date: r.Date}

		switch p := r.PeerID.(type) {
		case *tg.PeerUser:
			sender.PeerID = int64(p.UserID)
		case *tg.PeerChannel:
			sender.PeerID = -1000000000000 - int64(p.ChannelID)
		}

		switch rx := r.Reaction.(type) {
		case *tg.ReactionEmoji:
			sender.Reaction = ReactionEmoji{Emoticon: rx.Emoticon}
		case *tg.ReactionCustomEmoji:
			sender.Reaction = ReactionCustomEmoji{DocumentID: rx.DocumentID}
		}

		out = append(out, sender)
	}
	return out, nil
}

// SetDefaultReaction sets the default reaction emoji for the current user.
func (c *MCUBClient) SetDefaultReaction(ctx context.Context, reaction ReactionClass) error {
	_, err := c.client.API().MessagesSetDefaultReaction(ctx, reaction.toTL())
	if err != nil {
		return fmt.Errorf("set default reaction: %w", err)
	}
	return nil
}
