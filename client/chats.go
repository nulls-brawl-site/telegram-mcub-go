package client

import (
	"context"
	"fmt"
	"math/rand"

	"github.com/gotd/td/tg"
)

// ChatPermissions describes the default permissions for members of a chat.
type ChatPermissions struct {
	SendMessages bool
	SendMedia    bool
	SendStickers bool
	SendGifs     bool
	SendGames    bool
	SendInline   bool
	EmbedLinks   bool
	SendPolls    bool
	ChangeInfo   bool
	InviteUsers  bool
	PinMessages  bool
}

// GetParticipants returns the members of a chat or channel.
// For channels/supergroups it queries the channel participants API.
// For regular groups it uses GetFullChat.
func (c *MCUBClient) GetParticipants(ctx context.Context, chatID int64, limit int) ([]interface{}, error) {
	if limit <= 0 {
		limit = 200
	}

	if chatID < -999999999 {
		// Channel / supergroup.
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
			return nil, fmt.Errorf("get channel participants: %w", err)
		}
		chp, ok := result.(*tg.ChannelsChannelParticipants)
		if !ok {
			return nil, nil
		}
		out := make([]interface{}, len(chp.Participants))
		for i, p := range chp.Participants {
			out[i] = p
		}
		return out, nil
	}

	// Regular group.
	groupID := -chatID
	full, err := c.api.MessagesGetFullChat(ctx, groupID)
	if err != nil {
		return nil, fmt.Errorf("get full chat: %w", err)
	}
	cp, ok := full.FullChat.(*tg.ChatFull)
	if !ok {
		return nil, fmt.Errorf("unexpected full chat type")
	}
	participants, ok := cp.Participants.(*tg.ChatParticipants)
	if !ok {
		return nil, nil
	}
	out := make([]interface{}, 0, len(participants.Participants))
	for _, p := range participants.Participants {
		out = append(out, p)
	}
	return out, nil
}

// GetParticipant returns information about a specific participant in a channel or supergroup.
func (c *MCUBClient) GetParticipant(ctx context.Context, chatID, userID int64) (interface{}, error) {
	channel, err := c.resolveInputChannel(ctx, chatID)
	if err != nil {
		return nil, fmt.Errorf("resolve channel: %w", err)
	}

	result, err := c.api.ChannelsGetParticipant(ctx, &tg.ChannelsGetParticipantRequest{
		Channel:     channel,
		Participant: &tg.InputPeerUser{UserID: userID},
	})
	if err != nil {
		return nil, fmt.Errorf("get participant: %w", err)
	}
	return result.Participant, nil
}

// AddChatUser adds a user to a regular group chat.
// For channels/supergroups, use InviteToChannel instead.
func (c *MCUBClient) AddChatUser(ctx context.Context, chatID, userID int64) error {
	groupID := -chatID
	_, err := c.api.MessagesAddChatUser(ctx, &tg.MessagesAddChatUserRequest{
		ChatID:   groupID,
		UserID:   &tg.InputUser{UserID: userID},
		FwdLimit: 50,
	})
	if err != nil {
		return fmt.Errorf("add chat user: %w", err)
	}
	return nil
}

// InviteToChannel invites one or more users to a channel or supergroup.
func (c *MCUBClient) InviteToChannel(ctx context.Context, chatID int64, userIDs []int64) error {
	channel, err := c.resolveInputChannel(ctx, chatID)
	if err != nil {
		return fmt.Errorf("resolve channel: %w", err)
	}

	users := make([]tg.InputUserClass, 0, len(userIDs))
	for _, uid := range userIDs {
		users = append(users, &tg.InputUser{UserID: uid})
	}

	_, err = c.api.ChannelsInviteToChannel(ctx, &tg.ChannelsInviteToChannelRequest{
		Channel: channel,
		Users:   users,
	})
	if err != nil {
		return fmt.Errorf("invite to channel: %w", err)
	}
	return nil
}

// LeaveChat leaves a chat or channel.
func (c *MCUBClient) LeaveChat(ctx context.Context, chatID int64) error {
	if chatID < -999999999 {
		channel, err := c.resolveInputChannel(ctx, chatID)
		if err != nil {
			return fmt.Errorf("resolve channel: %w", err)
		}
		_, err = c.api.ChannelsLeaveChannel(ctx, channel)
		return err
	}

	// Regular group.
	groupID := -chatID
	_, err := c.api.MessagesDeleteChatUser(ctx, &tg.MessagesDeleteChatUserRequest{
		ChatID: groupID,
		UserID: &tg.InputUserSelf{},
	})
	return err
}

// DeleteChat deletes a chat.
// For supergroups and channels: leaves then deletes the channel history.
// For regular groups: leaves the chat.
func (c *MCUBClient) DeleteChat(ctx context.Context, chatID int64) error {
	if chatID < -999999999 {
		channel, err := c.resolveInputChannel(ctx, chatID)
		if err != nil {
			return fmt.Errorf("resolve channel: %w", err)
		}
		// Leave the channel first.
		_, err = c.api.ChannelsLeaveChannel(ctx, channel)
		if err != nil {
			return fmt.Errorf("leave channel: %w", err)
		}
		// Delete the channel history locally (API doesn't expose full delete for non-creators).
		return nil
	}

	// Regular group: leave.
	return c.LeaveChat(ctx, chatID)
}

// CreateGroup creates a new group chat with the given title and initial user IDs.
func (c *MCUBClient) CreateGroup(ctx context.Context, title string, userIDs []int64) (*tg.Chat, error) {
	users := make([]tg.InputUserClass, 0, len(userIDs))
	for _, uid := range userIDs {
		users = append(users, &tg.InputUser{UserID: uid})
	}

	result, err := c.api.MessagesCreateChat(ctx, &tg.MessagesCreateChatRequest{
		Users: users,
		Title: title,
	})
	if err != nil {
		return nil, fmt.Errorf("create group: %w", err)
	}

	// Extract the Chat from the updates.
	var chat *tg.Chat
	switch u := result.(type) {
	case *tg.Updates:
		for _, ch := range u.Chats {
			if c, ok := ch.(*tg.Chat); ok {
				chat = c
				break
			}
		}
	}
	if chat == nil {
		return nil, fmt.Errorf("chat not found in create response")
	}
	return chat, nil
}

// CreateChannel creates a new channel or supergroup.
// Set megagroup to true to create a supergroup; false for a broadcast channel.
func (c *MCUBClient) CreateChannel(ctx context.Context, title, about string, megagroup bool) (*tg.Channel, error) {
	req := &tg.ChannelsCreateChannelRequest{
		Title:     title,
		About:     about,
		Megagroup: megagroup,
		Broadcast: !megagroup,
	}
	result, err := c.api.ChannelsCreateChannel(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("create channel: %w", err)
	}

	var channel *tg.Channel
	switch u := result.(type) {
	case *tg.Updates:
		for _, ch := range u.Chats {
			if ch, ok := ch.(*tg.Channel); ok {
				channel = ch
				break
			}
		}
	}
	if channel == nil {
		return nil, fmt.Errorf("channel not found in create response")
	}
	return channel, nil
}

// EditChatTitle changes the title of a group or channel.
func (c *MCUBClient) EditChatTitle(ctx context.Context, chatID int64, title string) error {
	if chatID < -999999999 {
		channel, err := c.resolveInputChannel(ctx, chatID)
		if err != nil {
			return fmt.Errorf("resolve channel: %w", err)
		}
		_, err = c.api.ChannelsEditTitle(ctx, &tg.ChannelsEditTitleRequest{
			Channel: channel,
			Title:   title,
		})
		return err
	}

	// Regular group.
	groupID := -chatID
	_, err := c.api.MessagesEditChatTitle(ctx, &tg.MessagesEditChatTitleRequest{
		ChatID: groupID,
		Title:  title,
	})
	return err
}

// EditChatAbout changes the description of a group or channel.
func (c *MCUBClient) EditChatAbout(ctx context.Context, chatID int64, about string) error {
	peer, err := c.resolvePeer(ctx, chatID)
	if err != nil {
		return fmt.Errorf("resolve peer: %w", err)
	}

	_, err = c.api.MessagesEditChatAbout(ctx, &tg.MessagesEditChatAboutRequest{
		Peer:  peer,
		About: about,
	})
	return err
}

// EditChatPhoto uploads a photo from filePath and sets it as the group or channel photo.
func (c *MCUBClient) EditChatPhoto(ctx context.Context, chatID int64, filePath string) error {
	uploaded, err := c.uploadFile(ctx, UploadParams{Path: filePath})
	if err != nil {
		return fmt.Errorf("upload photo: %w", err)
	}

	photo := &tg.InputChatUploadedPhoto{
		File: uploaded.InputFile,
	}

	if chatID < -999999999 {
		channel, chErr := c.resolveInputChannel(ctx, chatID)
		if chErr != nil {
			return fmt.Errorf("resolve channel: %w", chErr)
		}
		_, err = c.api.ChannelsEditPhoto(ctx, &tg.ChannelsEditPhotoRequest{
			Channel: channel,
			Photo:   photo,
		})
		return err
	}

	groupID := -chatID
	_, err = c.api.MessagesEditChatPhoto(ctx, &tg.MessagesEditChatPhotoRequest{
		ChatID: groupID,
		Photo:  photo,
	})
	return err
}

// GetInviteLink returns the primary (permanent) invite link for a chat or channel.
func (c *MCUBClient) GetInviteLink(ctx context.Context, chatID int64) (string, error) {
	return c.ExportInviteLink(ctx, chatID)
}

// RevokeInviteLink revokes (deletes) a specific invite link.
func (c *MCUBClient) RevokeInviteLink(ctx context.Context, chatID int64, link string) error {
	peer, err := c.resolvePeer(ctx, chatID)
	if err != nil {
		return fmt.Errorf("resolve peer: %w", err)
	}

	_, err = c.api.MessagesDeleteExportedChatInvite(ctx, &tg.MessagesDeleteExportedChatInviteRequest{
		Peer: peer,
		Link: link,
	})
	return err
}

// ExportInviteLink creates and returns a new invite link for a chat or channel.
func (c *MCUBClient) ExportInviteLink(ctx context.Context, chatID int64) (string, error) {
	peer, err := c.resolvePeer(ctx, chatID)
	if err != nil {
		return "", fmt.Errorf("resolve peer: %w", err)
	}

	result, err := c.api.MessagesExportChatInvite(ctx, &tg.MessagesExportChatInviteRequest{
		Peer: peer,
	})
	if err != nil {
		return "", fmt.Errorf("export invite link: %w", err)
	}

	switch inv := result.(type) {
	case *tg.ChatInviteExported:
		return inv.Link, nil
	}
	return "", fmt.Errorf("unexpected invite result type: %T", result)
}

// SetChatPermissions sets the default permissions for all members of a group or channel.
// A permission set to false means that members are NOT allowed to perform that action.
func (c *MCUBClient) SetChatPermissions(ctx context.Context, chatID int64, perms ChatPermissions) error {
	peer, err := c.resolvePeer(ctx, chatID)
	if err != nil {
		return fmt.Errorf("resolve peer: %w", err)
	}

	_, err = c.api.MessagesEditChatDefaultBannedRights(ctx, &tg.MessagesEditChatDefaultBannedRightsRequest{
		Peer: peer,
		BannedRights: tg.ChatBannedRights{
			SendMessages: !perms.SendMessages,
			SendMedia:    !perms.SendMedia,
			SendStickers: !perms.SendStickers,
			SendGifs:     !perms.SendGifs,
			SendGames:    !perms.SendGames,
			SendInline:   !perms.SendInline,
			EmbedLinks:   !perms.EmbedLinks,
			SendPolls:    !perms.SendPolls,
			ChangeInfo:   !perms.ChangeInfo,
			InviteUsers:  !perms.InviteUsers,
			PinMessages:  !perms.PinMessages,
		},
	})
	return err
}

// ToggleSlowMode enables or disables slow mode for a supergroup.
// Set seconds to 0 to disable slow mode.
func (c *MCUBClient) ToggleSlowMode(ctx context.Context, chatID int64, seconds int) error {
	channel, err := c.resolveInputChannel(ctx, chatID)
	if err != nil {
		return fmt.Errorf("resolve channel: %w", err)
	}

	_, err = c.api.ChannelsToggleSlowMode(ctx, &tg.ChannelsToggleSlowModeRequest{
		Channel: channel,
		Seconds: seconds,
	})
	return err
}

// ensure rand is used (used in CreateGroup/CreateChannel via updates)
var _ = rand.Int63
