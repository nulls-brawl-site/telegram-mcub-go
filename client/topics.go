package client

import (
	"context"
	"fmt"
	"math/rand"

	"github.com/gotd/td/tg"
	"github.com/nulls-brawl-site/telegram-mcub-go/types"
)

// Topic represents a forum topic (thread) in a supergroup.
type Topic struct {
	// ID is the topic thread ID.
	ID int

	// Title is the topic name.
	Title string

	// IconEmojiID is the custom emoji ID used as the topic icon (0 if none).
	IconEmojiID int64

	// Closed indicates the topic is closed (no new messages).
	Closed bool

	// Hidden indicates the "General" topic is hidden.
	Hidden bool

	// Pinned indicates the topic is pinned in the topic list.
	Pinned bool

	// UnreadCount is the number of unread messages in the topic.
	UnreadCount int

	// TopMessageID is the ID of the latest message in the topic.
	TopMessageID int
}

// CreateTopicParams holds parameters for creating a forum topic.
type CreateTopicParams struct {
	// ChannelID is the supergroup's channel ID (positive).
	ChannelID int64

	// Title is the topic name.
	Title string

	// IconColor is the icon background color (one of Telegram's 7 palette values).
	IconColor int

	// IconEmojiID is the custom emoji ID for the topic icon (0 = default).
	IconEmojiID int64
}

// CreateTopic creates a new forum topic in a supergroup.
func (c *MCUBClient) CreateTopic(ctx context.Context, params CreateTopicParams) (*Topic, error) {
	req := &tg.ChannelsCreateForumTopicRequest{
		Channel:    &tg.InputChannel{ChannelID: params.ChannelID},
		Title:      params.Title,
		RandomID:   rand.Int63(),
	}
	if params.IconColor != 0 {
		req.IconColor = params.IconColor
		req.Flags.Set(0)
	}
	if params.IconEmojiID != 0 {
		req.IconEmojiID = params.IconEmojiID
		req.Flags.Set(3)
	}

	result, err := c.client.API().ChannelsCreateForumTopic(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("create topic: %w", err)
	}

	msg := extractMessageFromUpdates(result)
	_ = msg
	// The topic ID is the message ID of the service message that created it.
	// We return a stub; callers should use GetTopics to refresh.
	return &Topic{Title: params.Title}, nil
}

// GetTopicsParams holds parameters for retrieving forum topics.
type GetTopicsParams struct {
	// ChannelID is the supergroup's channel ID.
	ChannelID int64

	// TopicIDs filters to specific topic IDs. If empty, returns all topics.
	TopicIDs []int

	// OffsetDate is used for pagination (Unix timestamp).
	OffsetDate int

	// OffsetID is the last topic ID seen (for pagination).
	OffsetID int

	// Limit is the max number of topics to return (0 = server default).
	Limit int
}

// GetTopics fetches forum topics for a supergroup.
func (c *MCUBClient) GetTopics(ctx context.Context, params GetTopicsParams) ([]*Topic, error) {
	if len(params.TopicIDs) > 0 {
		return c.getTopicsByIDs(ctx, params.ChannelID, params.TopicIDs)
	}

	limit := params.Limit
	if limit == 0 {
		limit = 100
	}

	result, err := c.client.API().ChannelsGetForumTopics(ctx, &tg.ChannelsGetForumTopicsRequest{
		Channel:    &tg.InputChannel{ChannelID: params.ChannelID},
		OffsetDate: params.OffsetDate,
		OffsetID:   params.OffsetID,
		OffsetTopic: 0,
		Limit:      limit,
	})
	if err != nil {
		return nil, fmt.Errorf("get topics: %w", err)
	}

	return forumTopicsToTopics(result.Topics), nil
}

// getTopicsByIDs fetches specific topics by ID.
func (c *MCUBClient) getTopicsByIDs(ctx context.Context, channelID int64, ids []int) ([]*Topic, error) {
	result, err := c.client.API().ChannelsGetForumTopicsByID(ctx, &tg.ChannelsGetForumTopicsByIDRequest{
		Channel: &tg.InputChannel{ChannelID: channelID},
		Topics:  ids,
	})
	if err != nil {
		return nil, fmt.Errorf("get topics by id: %w", err)
	}
	return forumTopicsToTopics(result.Topics), nil
}

// IterTopics returns an iterator that paginates through all forum topics.
// The callback receives each batch of topics; returning false stops iteration.
func (c *MCUBClient) IterTopics(ctx context.Context, channelID int64, batchSize int, fn func([]*Topic) bool) error {
	if batchSize <= 0 {
		batchSize = 100
	}

	var (
		offsetDate  int
		offsetID    int
	)

	for {
		result, err := c.client.API().ChannelsGetForumTopics(ctx, &tg.ChannelsGetForumTopicsRequest{
			Channel:     &tg.InputChannel{ChannelID: channelID},
			OffsetDate:  offsetDate,
			OffsetID:    offsetID,
			OffsetTopic: 0,
			Limit:       batchSize,
		})
		if err != nil {
			return fmt.Errorf("iter topics: %w", err)
		}

		topics := forumTopicsToTopics(result.Topics)
		if len(topics) == 0 {
			break
		}

		if !fn(topics) {
			break
		}

		if result.Count <= offsetID+len(topics) {
			break
		}

		last := result.Topics[len(result.Topics)-1]
		if ft, ok := last.(*tg.ForumTopic); ok {
			offsetID = ft.ID
			offsetDate = ft.Date
		} else {
			break
		}
	}
	return nil
}

// SendToTopicParams holds parameters for sending a message to a forum topic.
type SendToTopicParams struct {
	// ChannelID is the supergroup's channel ID.
	ChannelID int64

	// TopicID is the forum topic thread ID.
	TopicID int

	// Text is the message body.
	Text string

	// Options contains optional send parameters.
	Options types.SendMessageOptions
}

// SendToTopic sends a text message to a specific forum topic.
func (c *MCUBClient) SendToTopic(ctx context.Context, params SendToTopicParams) (*tg.Message, error) {
	req := &tg.MessagesSendMessageRequest{
		Peer: &tg.InputPeerChannel{ChannelID: params.ChannelID},
		ReplyTo: &tg.InputReplyToMessage{
			ReplyToMsgID: params.TopicID,
		},
		Message:  params.Text,
		RandomID: rand.Int63(),
		Silent:   params.Options.Silent,
	}

	if params.Options.Buttons != nil {
		req.ReplyMarkup = params.Options.Buttons.ToTLMarkup()
	}

	result, err := c.client.API().MessagesSendMessage(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("send to topic: %w", err)
	}

	return extractMessageFromUpdates(result), nil
}

// IterTopicMessagesParams holds parameters for iterating topic messages.
type IterTopicMessagesParams struct {
	// ChannelID is the supergroup's channel ID.
	ChannelID int64

	// TopicID is the topic thread ID.
	TopicID int

	// Limit is the number of messages per page (0 = 100).
	Limit int

	// OffsetID is used for pagination.
	OffsetID int

	// Reverse iterates from oldest to newest when true.
	Reverse bool
}

// IterTopicMessages iterates messages within a forum topic, calling fn for each batch.
// Return false from fn to stop early.
func (c *MCUBClient) IterTopicMessages(
	ctx context.Context,
	params IterTopicMessagesParams,
	fn func([]*tg.Message) bool,
) error {
	limit := params.Limit
	if limit <= 0 {
		limit = 100
	}

	offsetID := params.OffsetID

	for {
		result, err := c.client.API().MessagesGetReplies(ctx, &tg.MessagesGetRepliesRequest{
			Peer:     &tg.InputPeerChannel{ChannelID: params.ChannelID},
			MsgID:    params.TopicID,
			OffsetID: offsetID,
			Limit:    limit,
		})
		if err != nil {
			return fmt.Errorf("iter topic messages: %w", err)
		}

		msgs := extractMessages(result)
		if len(msgs) == 0 {
			break
		}

		if params.Reverse {
			for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
				msgs[i], msgs[j] = msgs[j], msgs[i]
			}
		}

		if !fn(msgs) {
			break
		}

		offsetID = msgs[len(msgs)-1].ID
	}
	return nil
}

// forumTopicsToTopics converts a slice of tg.ForumTopicClass to []*Topic.
func forumTopicsToTopics(raw []tg.ForumTopicClass) []*Topic {
	out := make([]*Topic, 0, len(raw))
	for _, t := range raw {
		ft, ok := t.(*tg.ForumTopic)
		if !ok {
			continue
		}
		topic := &Topic{
			ID:           ft.ID,
			Title:        ft.Title,
			IconEmojiID:  ft.IconEmojiID,
			Closed:       ft.Closed,
			Hidden:       ft.Hidden,
			Pinned:       ft.Pinned,
			UnreadCount:  ft.UnreadCount,
			TopMessageID: ft.TopMessage,
		}
		out = append(out, topic)
	}
	return out
}
