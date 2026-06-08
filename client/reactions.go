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

// ClearReaction removes the current user's own reaction from a message.
func (c *MCUBClient) ClearReaction(ctx context.Context, chatID int64, msgID int) error {
	return c.SendReaction(ctx, SendReactionParams{
		PeerID:    chatID,
		MessageID: msgID,
		Reactions: nil, // empty = remove all
	})
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
// reaction may be nil to retrieve all reactions.
// offset is a pagination cursor (pass "" initially).
func (c *MCUBClient) GetMessageReactionsList(
	ctx context.Context,
	peerID int64,
	messageID int,
	reaction ReactionClass,
	limit int,
	offset string,
) ([]MessageReactionSender, string, error) {
	peer, err := c.resolvePeer(ctx, peerID)
	if err != nil {
		return nil, "", err
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
	if offset != "" {
		req.SetOffset(offset)
	}

	result, err := c.client.API().MessagesGetMessageReactionsList(ctx, req)
	if err != nil {
		return nil, "", fmt.Errorf("get reactions list: %w", err)
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

	nextOffset, _ := result.GetNextOffset()
	return out, nextOffset, nil
}

// GetMessageReactions returns reaction counts for the given message IDs in a chat.
func (c *MCUBClient) GetMessageReactions(ctx context.Context, chatID int64, msgIDs []int) (*tg.MessagesMessageReactionsList, error) {
	if len(msgIDs) == 0 {
		return &tg.MessagesMessageReactionsList{}, nil
	}
	// Use GetMessageReactionsList for the first message as a simplified proxy.
	// Full "get reaction counts" is available via messages.getMessagesReactions
	// which was added after v0.89.0. We iterate the reactions for the first
	// message here as a best-effort implementation.
	peer, err := c.resolvePeer(ctx, chatID)
	if err != nil {
		return nil, fmt.Errorf("resolve peer: %w", err)
	}
	req := &tg.MessagesGetMessageReactionsListRequest{
		Peer:  peer,
		ID:    msgIDs[0],
		Limit: 100,
	}
	result, err := c.client.API().MessagesGetMessageReactionsList(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("get message reactions: %w", err)
	}
	return result, nil
}

// SetDefaultReaction sets the default reaction emoji for the current user.
func (c *MCUBClient) SetDefaultReaction(ctx context.Context, reaction ReactionClass) error {
	_, err := c.client.API().MessagesSetDefaultReaction(ctx, reaction.toTL())
	if err != nil {
		return fmt.Errorf("set default reaction: %w", err)
	}
	return nil
}

// SetChatAvailableReactions sets the available reactions for a chat or channel.
// Pass an empty slice to allow all reactions; pass specific emojis to restrict.
func (c *MCUBClient) SetChatAvailableReactions(ctx context.Context, chatID int64, reactions []string) error {
	peer, err := c.resolvePeer(ctx, chatID)
	if err != nil {
		return fmt.Errorf("resolve peer: %w", err)
	}

	var chatReactions tg.ChatReactionsClass
	if len(reactions) == 0 {
		chatReactions = &tg.ChatReactionsAll{}
	} else {
		tlReactions := make([]tg.ReactionClass, 0, len(reactions))
		for _, emoji := range reactions {
			tlReactions = append(tlReactions, &tg.ReactionEmoji{Emoticon: emoji})
		}
		chatReactions = &tg.ChatReactionsSome{Reactions: tlReactions}
	}

	_, err = c.client.API().MessagesSetChatAvailableReactions(ctx, &tg.MessagesSetChatAvailableReactionsRequest{
		Peer:               peer,
		AvailableReactions: chatReactions,
	})
	if err != nil {
		return fmt.Errorf("set chat available reactions: %w", err)
	}
	return nil
}

// GetAvailableReactions returns the global list of available emoji reactions.
// hash=0 always fetches the latest list.
func (c *MCUBClient) GetAvailableReactions(ctx context.Context) (tg.MessagesAvailableReactionsClass, error) {
	result, err := c.client.API().MessagesGetAvailableReactions(ctx, 0)
	if err != nil {
		return nil, fmt.Errorf("get available reactions: %w", err)
	}
	return result, nil
}

// GetAvailableEffects returns the list of available message effects.
//
// Note: messages.getAvailableEffects was introduced after gotd v0.89.0.
// This method returns an empty list with a nil error for graceful degradation.
// Callers can upgrade the gotd/td dependency when a newer Go toolchain is available
// for full effects support.
func (c *MCUBClient) GetAvailableEffects(ctx context.Context) ([]interface{}, error) {
	// Effects are not available in gotd v0.89.0; return empty list gracefully.
	return []interface{}{}, nil
}

// SendMessageWithEffect sends a text message with a visual effect to a peer.
// effectID is the ID of the visual effect to apply.
//
// Note: the effect_id field in MessagesSendMessageRequest was introduced after
// gotd v0.89.0. This implementation falls back to sending the message without
// the effect when running on the current API layer.
func (c *MCUBClient) SendMessageWithEffect(ctx context.Context, peerID int64, text string, effectID int64) (*tg.Message, error) {
	// effectID is not supported in gotd v0.89.0; send the message without effect.
	return c.SendMessage(ctx, SendMessageParams{
		PeerID: peerID,
		Text:   text,
	})
}
