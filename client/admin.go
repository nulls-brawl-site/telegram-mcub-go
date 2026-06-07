package client

import (
	"context"
	"fmt"

	"github.com/gotd/td/tg"

	"github.com/nulls-brawl-site/telegram-mcub-go/types"
)

// BanUser permanently bans a user from a channel or supergroup.
func (c *MCUBClient) BanUser(ctx context.Context, chatID, userID int64) error {
	channel, err := c.resolveInputChannel(ctx, chatID)
	if err != nil {
		return err
	}

	_, err = c.api.ChannelsEditBanned(ctx, &tg.ChannelsEditBannedRequest{
		Channel:     channel,
		Participant: &tg.InputPeerUser{UserID: userID},
		BannedRights: tg.ChatBannedRights{
			ViewMessages: true,
			UntilDate:    0, // 0 = permanent
		},
	})
	if err != nil {
		return fmt.Errorf("ban user %d in %d: %w", userID, chatID, err)
	}
	return nil
}

// KickUser removes a user from a chat or channel.
// For supergroups/channels it bans then immediately unbans.
// For regular groups it calls MessagesDeleteChatUser.
func (c *MCUBClient) KickUser(ctx context.Context, chatID, userID int64) error {
	if chatID < -999999999 {
		// Kick from channel/supergroup: ban then unban.
		if err := c.BanUser(ctx, chatID, userID); err != nil {
			return err
		}
		return c.UnbanUser(ctx, chatID, userID)
	}

	// Regular group.
	groupID := -chatID
	_, err := c.api.MessagesDeleteChatUser(ctx, &tg.MessagesDeleteChatUserRequest{
		ChatID:   groupID,
		UserID:   &tg.InputUser{UserID: userID},
		RevokeHistory: false,
	})
	if err != nil {
		return fmt.Errorf("kick user %d from chat %d: %w", userID, chatID, err)
	}
	return nil
}

// UnbanUser removes a ban from a user in a channel/supergroup.
func (c *MCUBClient) UnbanUser(ctx context.Context, chatID, userID int64) error {
	channel, err := c.resolveInputChannel(ctx, chatID)
	if err != nil {
		return err
	}

	_, err = c.api.ChannelsEditBanned(ctx, &tg.ChannelsEditBannedRequest{
		Channel:      channel,
		Participant:  &tg.InputPeerUser{UserID: userID},
		BannedRights: tg.ChatBannedRights{}, // empty = no restrictions
	})
	if err != nil {
		return fmt.Errorf("unban user %d in %d: %w", userID, chatID, err)
	}
	return nil
}

// MuteUser restricts a user from sending messages in a channel/supergroup.
// untilDate is a Unix timestamp; 0 means permanent.
func (c *MCUBClient) MuteUser(ctx context.Context, chatID, userID int64, untilDate int) error {
	channel, err := c.resolveInputChannel(ctx, chatID)
	if err != nil {
		return err
	}

	_, err = c.api.ChannelsEditBanned(ctx, &tg.ChannelsEditBannedRequest{
		Channel:     channel,
		Participant: &tg.InputPeerUser{UserID: userID},
		BannedRights: tg.ChatBannedRights{
			SendMessages: true,
			SendMedia:    true,
			SendStickers: true,
			SendGifs:     true,
			SendGames:    true,
			SendInline:   true,
			UntilDate:    untilDate,
		},
	})
	if err != nil {
		return fmt.Errorf("mute user %d in %d: %w", userID, chatID, err)
	}
	return nil
}

// ChannelParticipant holds information about a channel participant.
type ChannelParticipant struct {
	UserID int64
	IsAdmin bool
	IsCreator bool
}

// GetAdmins returns the administrators of a channel or supergroup.
func (c *MCUBClient) GetAdmins(ctx context.Context, chatID int64) ([]*ChannelParticipant, error) {
	channel, err := c.resolveInputChannel(ctx, chatID)
	if err != nil {
		return nil, err
	}

	result, err := c.api.ChannelsGetParticipants(ctx, &tg.ChannelsGetParticipantsRequest{
		Channel: channel,
		Filter:  &tg.ChannelParticipantsAdmins{},
		Offset:  0,
		Limit:   200,
		Hash:    0,
	})
	if err != nil {
		return nil, fmt.Errorf("get admins of %d: %w", chatID, err)
	}

	return extractParticipants(result), nil
}

// GetMembers returns members of a channel or supergroup.
func (c *MCUBClient) GetMembers(ctx context.Context, chatID int64, limit int) ([]interface{}, error) {
	if limit <= 0 {
		limit = 200
	}

	channel, err := c.resolveInputChannel(ctx, chatID)
	if err != nil {
		return nil, err
	}

	result, err := c.api.ChannelsGetParticipants(ctx, &tg.ChannelsGetParticipantsRequest{
		Channel: channel,
		Filter:  &tg.ChannelParticipantsRecent{},
		Offset:  0,
		Limit:   limit,
		Hash:    0,
	})
	if err != nil {
		return nil, fmt.Errorf("get members of %d: %w", chatID, err)
	}

	participants := extractParticipants(result)
	out := make([]interface{}, len(participants))
	for i, p := range participants {
		out[i] = p
	}
	return out, nil
}

// resolveInputChannel converts a packed channel peer ID to an InputChannel.
func (c *MCUBClient) resolveInputChannel(ctx context.Context, chatID int64) (*tg.InputChannel, error) {
	_ = ctx
	// Accept packed peer IDs (-(channel_id + 1_000_000_000_000)) and raw positive channel IDs.
	if chatID < -999999999 {
		return &tg.InputChannel{ChannelID: channelIDFromPeerID(chatID), AccessHash: 0}, nil
	}
	if chatID > 0 {
		// Raw channel ID supplied directly.
		return &tg.InputChannel{ChannelID: chatID, AccessHash: 0}, nil
	}
	if chatID < 0 {
		// Regular basic group — not a channel.
		return nil, fmt.Errorf("chat ID %d is a basic group, not a channel/supergroup", chatID)
	}
	return nil, fmt.Errorf("invalid channel ID %d", chatID)
}

// extractParticipants converts a ChannelsChannelParticipants response.
func extractParticipants(result tg.ChannelsChannelParticipantsClass) []*ChannelParticipant {
	chp, ok := result.(*tg.ChannelsChannelParticipants)
	if !ok {
		return nil
	}
	out := make([]*ChannelParticipant, 0, len(chp.Participants))
	for _, p := range chp.Participants {
		cp := &ChannelParticipant{}
		switch v := p.(type) {
		case *tg.ChannelParticipant:
			cp.UserID = v.UserID
		case *tg.ChannelParticipantSelf:
			cp.UserID = v.UserID
		case *tg.ChannelParticipantCreator:
			cp.UserID = v.UserID
			cp.IsCreator = true
			cp.IsAdmin = true
		case *tg.ChannelParticipantAdmin:
			cp.UserID = v.UserID
			cp.IsAdmin = true
		case *tg.ChannelParticipantBanned:
			cp.UserID = peerToUserID(v.Peer)
		case *tg.ChannelParticipantLeft:
			cp.UserID = peerToUserID(v.Peer)
		}
		out = append(out, cp)
	}
	return out
}

// peerToUserID extracts a user ID from a PeerClass (0 if not a user).
func peerToUserID(peer tg.PeerClass) int64 {
	if p, ok := peer.(*tg.PeerUser); ok {
		return int64(p.UserID)
	}
	return 0
}

// GetAdminLog fetches the admin log for a channel or supergroup.
// Filters and pagination are controlled via params.
func (c *MCUBClient) GetAdminLog(ctx context.Context, params types.GetAdminLogParams) ([]*types.AdminLogEvent, error) {
	channel, err := c.resolveInputChannel(ctx, params.ChannelID)
	if err != nil {
		return nil, err
	}

	limit := params.Limit
	if limit <= 0 {
		limit = 100
	}

	req := &tg.ChannelsGetAdminLogRequest{
		Channel: channel,
		Q:       params.Query,
		MaxID:   params.MaxID,
		MinID:   params.MinID,
		Limit:   limit,
	}

	if f := params.ToTLEventsFilter(); f != nil {
		req.SetEventsFilter(*f)
	}

	result, err := c.api.ChannelsGetAdminLog(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("get admin log for channel %d: %w", params.ChannelID, err)
	}

	events := make([]*types.AdminLogEvent, 0, len(result.Events))
	for i := range result.Events {
		events = append(events, types.NewAdminLogEvent(&result.Events[i]))
	}
	return events, nil
}
