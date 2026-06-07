package client

import (
	"context"
	"fmt"

	"github.com/gotd/td/tg"
)

// Dialog represents a Telegram dialog (chat, channel, or user conversation).
type Dialog struct {
	// PeerID is the numeric peer ID.
	PeerID int64

	// Name is the display name.
	Name string

	// Username is the @username (may be empty).
	Username string

	// UnreadCount is the number of unread messages.
	UnreadCount int

	// IsUser is true when the dialog is a private conversation.
	IsUser bool

	// IsGroup is true when the dialog is a group chat.
	IsGroup bool

	// IsChannel is true when the dialog is a channel or supergroup.
	IsChannel bool
}

// GetDialogsParams holds parameters for GetDialogs.
type GetDialogsParams struct {
	// Limit is the maximum number of dialogs to return (0 = server default, max 100).
	Limit int

	// OffsetDate is the Unix timestamp of the last message in the last dialog of the previous page.
	OffsetDate int

	// OffsetID is the message ID of the last message in the last dialog of the previous page.
	OffsetID int

	// OffsetPeer is the peer ID of the last dialog of the previous page.
	OffsetPeer int64

	// ExcludePinned excludes pinned dialogs from results.
	ExcludePinned bool

	// FolderID specifies a folder to list (0 = all).
	FolderID int
}

// GetDialogs returns recent dialogs (chats, channels, and users).
func (c *MCUBClient) GetDialogs(ctx context.Context, params GetDialogsParams) ([]*Dialog, error) {
	limit := params.Limit
	if limit <= 0 {
		limit = 100
	}

	offsetPeer, _ := c.resolvePeer(ctx, params.OffsetPeer)
	if offsetPeer == nil {
		offsetPeer = &tg.InputPeerEmpty{}
	}

	req := &tg.MessagesGetDialogsRequest{
		ExcludePinned: params.ExcludePinned,
		FolderID:      params.FolderID,
		OffsetDate:    params.OffsetDate,
		OffsetID:      params.OffsetID,
		OffsetPeer:    offsetPeer,
		Limit:         limit,
		Hash:          0,
	}

	result, err := c.api.MessagesGetDialogs(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("get dialogs: %w", err)
	}

	return extractDialogs(result), nil
}

// GetDialog returns a single dialog by its peer ID.
func (c *MCUBClient) GetDialog(ctx context.Context, peerID int64) (*Dialog, error) {
	peer, err := c.resolvePeer(ctx, peerID)
	if err != nil {
		return nil, fmt.Errorf("resolve peer: %w", err)
	}

	result, err := c.api.MessagesGetPeerDialogs(ctx, []tg.InputDialogPeerClass{
		&tg.InputDialogPeer{Peer: peer},
	})
	if err != nil {
		return nil, fmt.Errorf("get peer dialog: %w", err)
	}

	dialogs := extractDialogsFromFull(result)
	if len(dialogs) == 0 {
		return nil, fmt.Errorf("dialog not found for peer %d", peerID)
	}
	return dialogs[0], nil
}

// extractDialogs converts a MessagesDialogsClass into []*Dialog.
func extractDialogs(result tg.MessagesDialogsClass) []*Dialog {
	var (
		rawDialogs []tg.DialogClass
		users      []tg.UserClass
		chats      []tg.ChatClass
	)
	switch r := result.(type) {
	case *tg.MessagesDialogs:
		rawDialogs = r.Dialogs
		users = r.Users
		chats = r.Chats
	case *tg.MessagesDialogsSlice:
		rawDialogs = r.Dialogs
		users = r.Users
		chats = r.Chats
	default:
		return nil
	}

	return buildDialogs(rawDialogs, users, chats)
}

// extractDialogsFromFull converts a MessagesPeerDialogs response into []*Dialog.
func extractDialogsFromFull(result *tg.MessagesPeerDialogs) []*Dialog {
	return buildDialogs(result.Dialogs, result.Users, result.Chats)
}

// buildDialogs assembles Dialog structs from raw TL data.
func buildDialogs(rawDialogs []tg.DialogClass, users []tg.UserClass, chats []tg.ChatClass) []*Dialog {
	// Build lookup maps.
	userMap := make(map[int64]*tg.User, len(users))
	for _, u := range users {
		if user, ok := u.(*tg.User); ok {
			userMap[user.ID] = user
		}
	}
	chatMap := make(map[int64]tg.ChatClass, len(chats))
	for _, ch := range chats {
		switch v := ch.(type) {
		case *tg.Chat:
			chatMap[v.ID] = v
		case *tg.Channel:
			chatMap[v.ID] = v
		}
	}

	out := make([]*Dialog, 0, len(rawDialogs))
	for _, d := range rawDialogs {
		dlg, ok := d.(*tg.Dialog)
		if !ok {
			continue
		}
		dialog := dialogFromPeer(dlg.Peer, dlg.UnreadCount, userMap, chatMap)
		if dialog != nil {
			out = append(out, dialog)
		}
	}
	return out
}

// dialogFromPeer constructs a Dialog given a peer and supporting maps.
func dialogFromPeer(
	peer tg.PeerClass,
	unread int,
	userMap map[int64]*tg.User,
	chatMap map[int64]tg.ChatClass,
) *Dialog {
	switch p := peer.(type) {
	case *tg.PeerUser:
		d := &Dialog{
			PeerID:      int64(p.UserID),
			UnreadCount: unread,
			IsUser:      true,
		}
		if u, ok := userMap[int64(p.UserID)]; ok {
			d.Name = u.FirstName + " " + u.LastName
			d.Username, _ = u.GetUsername()
		}
		return d

	case *tg.PeerChat:
		d := &Dialog{
			PeerID:      -int64(p.ChatID),
			UnreadCount: unread,
			IsGroup:     true,
		}
		if ch, ok := chatMap[int64(p.ChatID)]; ok {
			if chat, ok := ch.(*tg.Chat); ok {
				d.Name = chat.Title
			}
		}
		return d

	case *tg.PeerChannel:
		d := &Dialog{
			PeerID:      -1000000000000 - int64(p.ChannelID),
			UnreadCount: unread,
			IsChannel:   true,
		}
		if ch, ok := chatMap[int64(p.ChannelID)]; ok {
			if channel, ok := ch.(*tg.Channel); ok {
				d.Name = channel.Title
				d.Username, _ = channel.GetUsername()
				if channel.Megagroup {
					d.IsChannel = false
					d.IsGroup = true
				}
			}
		}
		return d
	}
	return nil
}
