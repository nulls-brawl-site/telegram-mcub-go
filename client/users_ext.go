package client

import (
	"context"
	"fmt"
	"strings"

	"github.com/gotd/td/tg"
)

// GetNearestDC returns the ID of the nearest Telegram data centre.
// Mirrors Telethon's get_nearest_dc() call.
func (c *MCUBClient) GetNearestDC(ctx context.Context) (int, error) {
	result, err := c.api.HelpGetNearestDC(ctx)
	if err != nil {
		return 0, fmt.Errorf("get nearest DC: %w", err)
	}
	return result.NearestDC, nil
}

// GetCDNConfig returns the CDN server configuration from Telegram.
func (c *MCUBClient) GetCDNConfig(ctx context.Context) (*tg.CDNConfig, error) {
	result, err := c.api.HelpGetCDNConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("get CDN config: %w", err)
	}
	return result, nil
}

// CheckUsername checks whether a username is available.
// Returns true when the username is free to use.
func (c *MCUBClient) CheckUsername(ctx context.Context, username string) (bool, error) {
	// Strip leading '@' if present.
	username = strings.TrimPrefix(username, "@")
	available, err := c.api.AccountCheckUsername(ctx, username)
	if err != nil {
		return false, fmt.Errorf("check username %q: %w", username, err)
	}
	return available, nil
}

// UpdateStatusOnline marks the current user as online.
func (c *MCUBClient) UpdateStatusOnline(ctx context.Context) error {
	_, err := c.api.AccountUpdateStatus(ctx, false) // offline = false → online
	if err != nil {
		return fmt.Errorf("update status online: %w", err)
	}
	return nil
}

// UpdateStatusOffline marks the current user as offline.
func (c *MCUBClient) UpdateStatusOffline(ctx context.Context) error {
	_, err := c.api.AccountUpdateStatus(ctx, true) // offline = true
	if err != nil {
		return fmt.Errorf("update status offline: %w", err)
	}
	return nil
}

// GetNotifyExceptions returns the list of peer-specific notification exceptions.
// Each element is a tg.PeerNotifySettings (or similar) interface value.
func (c *MCUBClient) GetNotifyExceptions(ctx context.Context) ([]interface{}, error) {
	result, err := c.api.AccountGetNotifyExceptions(ctx, &tg.AccountGetNotifyExceptionsRequest{})
	if err != nil {
		return nil, fmt.Errorf("get notify exceptions: %w", err)
	}
	// result is tg.UpdatesClass
	var out []interface{}
	switch u := result.(type) {
	case *tg.Updates:
		for _, upd := range u.Updates {
			out = append(out, upd)
		}
	case *tg.UpdatesCombined:
		for _, upd := range u.Updates {
			out = append(out, upd)
		}
	default:
		out = append(out, result)
	}
	return out, nil
}

// SetTyping sends a typing / chat action to a chat.
// action can be: "typing", "cancel", "record_audio", "upload_photo",
// "upload_document", "geo", "choose_sticker", "record_video", "upload_video".
func (c *MCUBClient) SetTyping(ctx context.Context, peerID int64, action string) error {
	peer, err := c.resolvePeer(ctx, peerID)
	if err != nil {
		return fmt.Errorf("resolve peer: %w", err)
	}

	var tlAction tg.SendMessageActionClass
	switch action {
	case "typing":
		tlAction = &tg.SendMessageTypingAction{}
	case "cancel":
		tlAction = &tg.SendMessageCancelAction{}
	case "record_audio":
		tlAction = &tg.SendMessageRecordAudioAction{}
	case "upload_photo":
		tlAction = &tg.SendMessageUploadPhotoAction{}
	case "upload_document":
		tlAction = &tg.SendMessageUploadDocumentAction{}
	case "geo":
		tlAction = &tg.SendMessageGeoLocationAction{}
	case "choose_sticker":
		tlAction = &tg.SendMessageChooseStickerAction{}
	case "record_video":
		tlAction = &tg.SendMessageRecordVideoAction{}
	case "upload_video":
		tlAction = &tg.SendMessageUploadVideoAction{}
	default:
		tlAction = &tg.SendMessageTypingAction{}
	}

	_, err = c.api.MessagesSetTyping(ctx, &tg.MessagesSetTypingRequest{
		Peer:   peer,
		Action: tlAction,
	})
	if err != nil {
		return fmt.Errorf("set typing: %w", err)
	}
	return nil
}

// GetOnlineStatus returns the tg.UserStatusClass for a user.
// Callers may type-assert to *tg.UserStatusOnline, *tg.UserStatusOffline, etc.
func (c *MCUBClient) GetOnlineStatus(ctx context.Context, userID int64) (tg.UserStatusClass, error) {
	user, err := c.GetUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	// user.Status is tg.UserStatusClass; may be nil for privacy-restricted users.
	return user.Status, nil
}

// GetBotInfoByID returns the BotInfo for a bot identified by botID.
func (c *MCUBClient) GetBotInfoByID(ctx context.Context, botID int64) (*tg.BotInfo, error) {
	full, err := c.GetFullUser(ctx, botID)
	if err != nil {
		return nil, fmt.Errorf("get bot info: %w", err)
	}
	// BotInfo is an embedded struct (not an interface pointer); check via Zero().
	info, ok := full.FullUser.GetBotInfo()
	if !ok {
		return nil, fmt.Errorf("user %d is not a bot or has no BotInfo", botID)
	}
	return &info, nil
}
