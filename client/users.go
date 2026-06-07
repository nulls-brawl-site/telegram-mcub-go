package client

import (
	"context"
	"fmt"

	"github.com/gotd/td/tg"
)

// GetUser returns a tg.User by their numeric user ID.
func (c *MCUBClient) GetUser(ctx context.Context, userID int64) (*tg.User, error) {
	result, err := c.api.UsersGetUsers(ctx, []tg.InputUserClass{
		&tg.InputUser{UserID: userID},
	})
	if err != nil {
		return nil, fmt.Errorf("get user %d: %w", userID, err)
	}
	for _, u := range result {
		if user, ok := u.(*tg.User); ok && user.ID == userID {
			return user, nil
		}
	}
	return nil, fmt.Errorf("user %d not found", userID)
}

// GetChat returns chat or channel info by a numeric peer ID.
// The returned value is one of *tg.Chat, *tg.Channel, or *tg.User.
func (c *MCUBClient) GetChat(ctx context.Context, chatID int64) (interface{}, error) {
	if chatID > 0 {
		return c.GetUser(ctx, chatID)
	}

	if chatID < -999999999 {
		// Supergroup / channel.
		chanID := channelIDFromPeerID(chatID)
		result, err := c.api.ChannelsGetChannels(ctx, []tg.InputChannelClass{
			&tg.InputChannel{ChannelID: chanID},
		})
		if err != nil {
			return nil, fmt.Errorf("get channel %d: %w", chanID, err)
		}
		switch r := result.(type) {
		case *tg.MessagesChats:
			if len(r.Chats) > 0 {
				return r.Chats[0], nil
			}
		case *tg.MessagesChatsSlice:
			if len(r.Chats) > 0 {
				return r.Chats[0], nil
			}
		}
		return nil, fmt.Errorf("channel %d not found", chanID)
	}

	// Regular group chat.
	groupID := -chatID
	result, err := c.api.MessagesGetChats(ctx, []int64{groupID})
	if err != nil {
		return nil, fmt.Errorf("get chat %d: %w", groupID, err)
	}
	switch r := result.(type) {
	case *tg.MessagesChats:
		if len(r.Chats) > 0 {
			return r.Chats[0], nil
		}
	case *tg.MessagesChatsSlice:
		if len(r.Chats) > 0 {
			return r.Chats[0], nil
		}
	}
	return nil, fmt.Errorf("chat %d not found", groupID)
}

// GetProfilePhotos returns up to limit profile photos for the given user or chat ID.
func (c *MCUBClient) GetProfilePhotos(ctx context.Context, entityID int64, limit int) ([]*tg.Photo, error) {
	if limit <= 0 {
		limit = 100
	}

	var inputPeer tg.InputPeerClass
	if entityID > 0 {
		inputPeer = &tg.InputPeerUser{UserID: entityID}
	} else if entityID < -999999999 {
		chanID := channelIDFromPeerID(entityID)
		inputPeer = &tg.InputPeerChannel{ChannelID: chanID}
	} else {
		inputPeer = &tg.InputPeerChat{ChatID: -entityID}
	}

	result, err := c.api.PhotosGetUserPhotos(ctx, &tg.PhotosGetUserPhotosRequest{
		UserID: inputPeerToInputUser(inputPeer),
		Offset: 0,
		MaxID:  0,
		Limit:  limit,
	})
	if err != nil {
		return nil, fmt.Errorf("get profile photos: %w", err)
	}

	var photos []tg.PhotoClass
	switch r := result.(type) {
	case *tg.PhotosPhotos:
		photos = r.Photos
	case *tg.PhotosPhotosSlice:
		photos = r.Photos
	}

	out := make([]*tg.Photo, 0, len(photos))
	for _, p := range photos {
		if photo, ok := p.(*tg.Photo); ok {
			out = append(out, photo)
		}
	}
	return out, nil
}

// inputPeerToInputUser converts an InputPeerUser to InputUser; others use InputUserSelf.
func inputPeerToInputUser(peer tg.InputPeerClass) tg.InputUserClass {
	if p, ok := peer.(*tg.InputPeerUser); ok {
		return &tg.InputUser{UserID: p.UserID}
	}
	return &tg.InputUserSelf{}
}
