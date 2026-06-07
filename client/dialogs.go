package client

import (
	"context"
	"fmt"
	"time"

	"github.com/gotd/td/tg"

	"github.com/nulls-brawl-site/telegram-mcub-go/types"
)

// ─────────────────────────────────────────────────────────────────────────────
// Legacy low-level helpers (kept for internal use)
// ─────────────────────────────────────────────────────────────────────────────

// Dialog is a lightweight dialog representation used by the low-level helpers.
type Dialog struct {
	PeerID      int64
	Name        string
	Username    string
	UnreadCount int
	IsUser      bool
	IsGroup     bool
	IsChannel   bool
}

// GetDialogsParams holds parameters for the low-level GetDialogsRaw.
type GetDialogsParams struct {
	Limit         int
	OffsetDate    int
	OffsetID      int
	OffsetPeer    int64
	ExcludePinned bool
	FolderID      int
}

// GetDialogsRaw returns raw dialogs using low-level API parameters.
// Prefer GetDialogs or IterDialogs for most use cases.
func (c *MCUBClient) GetDialogsRaw(ctx context.Context, params GetDialogsParams) ([]*Dialog, error) {
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

// GetDialogRaw returns a single raw dialog by peer ID.
func (c *MCUBClient) GetDialogRaw(ctx context.Context, peerID int64) (*Dialog, error) {
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

func extractDialogsFromFull(result *tg.MessagesPeerDialogs) []*Dialog {
	return buildDialogs(result.Dialogs, result.Users, result.Chats)
}

func buildDialogs(rawDialogs []tg.DialogClass, users []tg.UserClass, chats []tg.ChatClass) []*Dialog {
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

func dialogFromPeer(peer tg.PeerClass, unread int, userMap map[int64]*tg.User, chatMap map[int64]tg.ChatClass) *Dialog {
	switch p := peer.(type) {
	case *tg.PeerUser:
		d := &Dialog{PeerID: int64(p.UserID), UnreadCount: unread, IsUser: true}
		if u, ok := userMap[int64(p.UserID)]; ok {
			d.Name = u.FirstName + " " + u.LastName
			d.Username, _ = u.GetUsername()
		}
		return d
	case *tg.PeerChat:
		d := &Dialog{PeerID: -int64(p.ChatID), UnreadCount: unread, IsGroup: true}
		if ch, ok := chatMap[int64(p.ChatID)]; ok {
			if chat, ok2 := ch.(*tg.Chat); ok2 {
				d.Name = chat.Title
			}
		}
		return d
	case *tg.PeerChannel:
		d := &Dialog{PeerID: -1000000000000 - int64(p.ChannelID), UnreadCount: unread, IsChannel: true}
		if ch, ok := chatMap[int64(p.ChannelID)]; ok {
			if channel, ok2 := ch.(*tg.Channel); ok2 {
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

// ─────────────────────────────────────────────────────────────────────────────
// High-level dialog API — ported from Telethon-MCUB/telethon/client/dialogs.py
// ─────────────────────────────────────────────────────────────────────────────

// IterDialogsParams configures IterDialogs.
type IterDialogsParams struct {
	// Limit is the maximum number of dialogs to return (0 = all).
	Limit int
	// OffsetDate is an optional start date; dialogs with a top message older
	// than this value will be returned.
	OffsetDate time.Time
	// OffsetID is the message ID used as a pagination cursor.
	OffsetID int
	// OffsetPeer is the peer ID used as a pagination cursor.
	OffsetPeer int64
	// IgnorePinned skips pinned dialogs.
	IgnorePinned bool
	// IgnoreMigrated skips chats that have been migrated to a channel.
	IgnoreMigrated bool
	// FolderID restricts results to a specific folder (0 = default, 1 = archived).
	FolderID int
	// Archived is a convenience alias: true ⇒ FolderID=1, false ⇒ FolderID=0.
	// Ignored when FolderID is explicitly set to a non-zero value.
	Archived bool
}

// IterDialogs returns a channel that yields resolved MCUBDialog values and a
// separate error channel. The caller must drain both channels to avoid goroutine
// leaks. The dialog channel is closed when iteration is complete or an error
// occurs; the error channel carries at most one value.
//
// Ported from Telethon's _DialogsIter / iter_dialogs.
func (c *MCUBClient) IterDialogs(ctx context.Context, params IterDialogsParams) (<-chan *types.MCUBDialog, <-chan error) {
	out := make(chan *types.MCUBDialog, 64)
	errc := make(chan error, 1)

	// Resolve archived shortcut.
	if params.FolderID == 0 && params.Archived {
		params.FolderID = 1
	}

	go func() {
		defer close(out)
		defer close(errc)

		var (
			offsetDate int
			offsetID   int
			offsetPeer tg.InputPeerClass = &tg.InputPeerEmpty{}
			fetched    int
		)

		if !params.OffsetDate.IsZero() {
			offsetDate = int(params.OffsetDate.Unix())
		}
		offsetID = params.OffsetID

		if params.OffsetPeer != 0 {
			if p, _ := c.resolvePeer(ctx, params.OffsetPeer); p != nil {
				offsetPeer = p
			}
		}

		seen := make(map[int64]struct{})

		for {
			limit := 100
			if params.Limit > 0 {
				remaining := params.Limit - fetched
				if remaining <= 0 {
					return
				}
				if remaining < limit {
					limit = remaining
				}
			}

			req := &tg.MessagesGetDialogsRequest{
				ExcludePinned: params.IgnorePinned,
				FolderID:      params.FolderID,
				OffsetDate:    offsetDate,
				OffsetID:      offsetID,
				OffsetPeer:    offsetPeer,
				Limit:         limit,
				Hash:          0,
			}

			raw, err := c.api.MessagesGetDialogs(ctx, req)
			if err != nil {
				errc <- fmt.Errorf("iter dialogs: %w", err)
				return
			}

			var (
				rawDialogs []tg.DialogClass
				rawUsers   []tg.UserClass
				rawChats   []tg.ChatClass
				rawMsgs    []tg.MessageClass
				total      int
			)
			switch r := raw.(type) {
			case *tg.MessagesDialogs:
				rawDialogs = r.Dialogs
				rawUsers = r.Users
				rawChats = r.Chats
				rawMsgs = r.Messages
				total = len(r.Dialogs)
			case *tg.MessagesDialogsSlice:
				rawDialogs = r.Dialogs
				rawUsers = r.Users
				rawChats = r.Chats
				rawMsgs = r.Messages
				total = r.Count
			default:
				return
			}

			_ = total

			// Build entity maps.
			userMap := make(map[int64]*tg.User, len(rawUsers))
			for _, u := range rawUsers {
				if user, ok := u.(*tg.User); ok {
					userMap[user.ID] = user
				}
			}
			chatMap := make(map[int64]tg.ChatClass, len(rawChats))
			for _, ch := range rawChats {
				switch v := ch.(type) {
				case *tg.Chat:
					chatMap[v.ID] = v
				case *tg.Channel:
					chatMap[v.ID] = v
				case *tg.ChatForbidden:
					chatMap[v.ID] = v
				case *tg.ChannelForbidden:
					chatMap[v.ID] = v
				}
			}

			// Index top messages by (channel_id, msg_id) or (0, msg_id).
			type msgKey struct{ chanID, msgID int64 }
			msgMap := make(map[msgKey]*tg.Message, len(rawMsgs))
			for _, m := range rawMsgs {
				if msg, ok := m.(*tg.Message); ok {
					key := msgKey{msgID: int64(msg.ID)}
					if ch, ok2 := msg.PeerID.(*tg.PeerChannel); ok2 {
						key.chanID = int64(ch.ChannelID)
					}
					msgMap[key] = msg
				}
			}

			var lastMsg *tg.Message
			var lastOffsetPeer tg.InputPeerClass

			for _, d := range rawDialogs {
				dlg, ok := d.(*tg.Dialog)
				if !ok {
					continue
				}

				// Determine peer ID for dedup.
				var peerID int64
				switch p := dlg.Peer.(type) {
				case *tg.PeerUser:
					peerID = int64(p.UserID)
				case *tg.PeerChat:
					peerID = -int64(p.ChatID)
				case *tg.PeerChannel:
					peerID = -1000000000000 - int64(p.ChannelID)
				}

				if _, dup := seen[peerID]; dup {
					continue
				}
				seen[peerID] = struct{}{}

				// Resolve entity.
				var entity interface{}
				switch p := dlg.Peer.(type) {
				case *tg.PeerUser:
					if u, ok2 := userMap[int64(p.UserID)]; ok2 {
						entity = u
					}
				case *tg.PeerChat:
					if ch, ok2 := chatMap[int64(p.ChatID)]; ok2 {
						entity = ch
					}
				case *tg.PeerChannel:
					if ch, ok2 := chatMap[int64(p.ChannelID)]; ok2 {
						entity = ch
					}
				}
				if entity == nil {
					continue
				}

				// Skip migrated chats if requested.
				if params.IgnoreMigrated {
					if ch, ok2 := entity.(*tg.Chat); ok2 {
						if _, hasMigrated := ch.GetMigratedTo(); hasMigrated {
							continue
						}
					}
				}

				// Resolve top message.
				var topMsg *tg.Message
				{
					key := msgKey{msgID: int64(dlg.TopMessage)}
					if ch, ok2 := dlg.Peer.(*tg.PeerChannel); ok2 {
						key.chanID = int64(ch.ChannelID)
					}
					topMsg = msgMap[key]
				}

				// Apply offset date filter (Telegram may ignore it).
				if !params.OffsetDate.IsZero() && topMsg != nil {
					msgTime := time.Unix(int64(topMsg.Date), 0)
					if msgTime.After(params.OffsetDate) {
						continue
					}
				}

				// Build MCUBDialog (no draft/mute info at this level).
				mcubDlg := types.NewMCUBDialog(dlg, entity, topMsg, nil, false)

				select {
				case <-ctx.Done():
					errc <- ctx.Err()
					return
				case out <- mcubDlg:
				}

				fetched++
				lastMsg = topMsg
				lastOffsetPeer = inputPeerFromDialog(dlg.Peer, userMap, chatMap)

				if params.Limit > 0 && fetched >= params.Limit {
					return
				}
			}

			// Determine whether to fetch more pages.
			if len(rawDialogs) < limit {
				// Reached the end.
				return
			}
			if _, isSlice := raw.(*tg.MessagesDialogsSlice); !isSlice {
				// Got a full (non-paginated) result.
				return
			}

			// Advance cursor for next page.
			if lastMsg != nil {
				offsetID = lastMsg.ID
				offsetDate = lastMsg.Date
			}
			if lastOffsetPeer != nil {
				offsetPeer = lastOffsetPeer
			}
		}
	}()

	return out, errc
}

// inputPeerFromDialog builds an InputPeer from a Dialog peer reference.
func inputPeerFromDialog(peer tg.PeerClass, userMap map[int64]*tg.User, chatMap map[int64]tg.ChatClass) tg.InputPeerClass {
	switch p := peer.(type) {
	case *tg.PeerUser:
		if u, ok := userMap[int64(p.UserID)]; ok {
			hash, _ := u.GetAccessHash()
			return &tg.InputPeerUser{UserID: u.ID, AccessHash: hash}
		}
	case *tg.PeerChat:
		return &tg.InputPeerChat{ChatID: p.ChatID}
	case *tg.PeerChannel:
		if ch, ok := chatMap[int64(p.ChannelID)]; ok {
			if channel, ok2 := ch.(*tg.Channel); ok2 {
				hash, _ := channel.GetAccessHash()
				return &tg.InputPeerChannel{ChannelID: channel.ID, AccessHash: hash}
			}
		}
	}
	return &tg.InputPeerEmpty{}
}

// GetDialogs returns a slice of resolved dialogs up to limit (0 = all).
// It is a convenience wrapper around IterDialogs.
func (c *MCUBClient) GetDialogs(ctx context.Context, limit int) ([]*types.MCUBDialog, error) {
	ch, errc := c.IterDialogs(ctx, IterDialogsParams{Limit: limit})
	var result []*types.MCUBDialog
	for d := range ch {
		result = append(result, d)
	}
	if err := <-errc; err != nil {
		return result, err
	}
	return result, nil
}

// GetDialogByID returns the dialog for the given peer ID, or an error if not found.
func (c *MCUBClient) GetDialogByID(ctx context.Context, peerID int64) (*types.MCUBDialog, error) {
	peer, err := c.resolvePeer(ctx, peerID)
	if err != nil {
		return nil, fmt.Errorf("resolve peer %d: %w", peerID, err)
	}

	result, err := c.api.MessagesGetPeerDialogs(ctx, []tg.InputDialogPeerClass{
		&tg.InputDialogPeer{Peer: peer},
	})
	if err != nil {
		return nil, fmt.Errorf("get peer dialog: %w", err)
	}

	userMap := make(map[int64]*tg.User, len(result.Users))
	for _, u := range result.Users {
		if user, ok := u.(*tg.User); ok {
			userMap[user.ID] = user
		}
	}
	chatMap := make(map[int64]tg.ChatClass, len(result.Chats))
	for _, ch := range result.Chats {
		switch v := ch.(type) {
		case *tg.Chat:
			chatMap[v.ID] = v
		case *tg.Channel:
			chatMap[v.ID] = v
		case *tg.ChatForbidden:
			chatMap[v.ID] = v
		case *tg.ChannelForbidden:
			chatMap[v.ID] = v
		}
	}

	msgMap := make(map[int]*tg.Message, len(result.Messages))
	for _, m := range result.Messages {
		if msg, ok := m.(*tg.Message); ok {
			msgMap[msg.ID] = msg
		}
	}

	for _, d := range result.Dialogs {
		dlg, ok := d.(*tg.Dialog)
		if !ok {
			continue
		}
		var entity interface{}
		switch p := dlg.Peer.(type) {
		case *tg.PeerUser:
			entity = userMap[int64(p.UserID)]
		case *tg.PeerChat:
			entity = chatMap[int64(p.ChatID)]
		case *tg.PeerChannel:
			entity = chatMap[int64(p.ChannelID)]
		}
		topMsg := msgMap[dlg.TopMessage]
		return types.NewMCUBDialog(dlg, entity, topMsg, nil, false), nil
	}
	return nil, fmt.Errorf("dialog not found for peer %d", peerID)
}

// PinDialog pins or unpins the dialog with the given peer ID.
// Ported from Telethon's Dialog.pin() / messages.toggleDialogPin.
func (c *MCUBClient) PinDialog(ctx context.Context, peerID int64, pin bool) error {
	peer, err := c.resolvePeer(ctx, peerID)
	if err != nil {
		return fmt.Errorf("resolve peer %d: %w", peerID, err)
	}
	req := &tg.MessagesToggleDialogPinRequest{
		Peer: &tg.InputDialogPeer{Peer: peer},
	}
	req.SetPinned(pin)
	_, err = c.api.MessagesToggleDialogPin(ctx, req)
	if err != nil {
		return fmt.Errorf("toggle pin: %w", err)
	}
	return nil
}

// ArchiveDialog moves the dialog into folder 1 (archived) or out of it (folder 0).
// Ported from Telethon's Dialog.archive() / folders.editPeerFolders.
func (c *MCUBClient) ArchiveDialog(ctx context.Context, peerID int64, archive bool) error {
	peer, err := c.resolvePeer(ctx, peerID)
	if err != nil {
		return fmt.Errorf("resolve peer %d: %w", peerID, err)
	}
	folderID := 0
	if archive {
		folderID = 1
	}
	_, err = c.api.FoldersEditPeerFolders(ctx, []tg.InputFolderPeer{
		{Peer: peer, FolderID: folderID},
	})
	if err != nil {
		return fmt.Errorf("archive dialog: %w", err)
	}
	return nil
}

// DeleteDialog deletes a dialog: leaves a channel/supergroup/chat or deletes a
// private conversation.  When revokeHistory is true the history is deleted for
// all participants (only effective for private chats).
// Ported from Telethon's delete_dialog().
func (c *MCUBClient) DeleteDialog(ctx context.Context, peerID int64, revokeHistory bool) error {
	peer, err := c.resolvePeer(ctx, peerID)
	if err != nil {
		return fmt.Errorf("resolve peer %d: %w", peerID, err)
	}

	switch p := peer.(type) {
	case *tg.InputPeerChannel:
		_, err = c.api.ChannelsLeaveChannel(ctx, &tg.InputChannel{
			ChannelID:  p.ChannelID,
			AccessHash: p.AccessHash,
		})
	case *tg.InputPeerChat:
		_, err = c.api.MessagesDeleteChatUser(ctx, &tg.MessagesDeleteChatUserRequest{
			ChatID:        p.ChatID,
			UserID:        &tg.InputUserSelf{},
			RevokeHistory: revokeHistory,
		})
	default:
		// Private conversation: delete history.
		_, err = c.api.MessagesDeleteHistory(ctx, &tg.MessagesDeleteHistoryRequest{
			Peer:  peer,
			MaxID: 0,
			Revoke: revokeHistory,
		})
	}
	return err
}

// MarkDialogAsRead marks all messages in the dialog as read.
// Ported from Telethon's Dialog.mark_as_read() / messages.readHistory.
func (c *MCUBClient) MarkDialogAsRead(ctx context.Context, peerID int64) error {
	peer, err := c.resolvePeer(ctx, peerID)
	if err != nil {
		return fmt.Errorf("resolve peer %d: %w", peerID, err)
	}

	switch p := peer.(type) {
	case *tg.InputPeerChannel:
		_, err = c.api.ChannelsReadHistory(ctx, &tg.ChannelsReadHistoryRequest{
			Channel: &tg.InputChannel{
				ChannelID:  p.ChannelID,
				AccessHash: p.AccessHash,
			},
			MaxID: 0,
		})
	default:
		_, err = c.api.MessagesReadHistory(ctx, &tg.MessagesReadHistoryRequest{
			Peer:  peer,
			MaxID: 0,
		})
	}
	return err
}

// MuteDialog mutes or unmutes a dialog's notifications.
// Ported from Telethon's Dialog.mute() / account.updateNotifySettings.
func (c *MCUBClient) MuteDialog(ctx context.Context, peerID int64, mute bool) error {
	peer, err := c.resolvePeer(ctx, peerID)
	if err != nil {
		return fmt.Errorf("resolve peer %d: %w", peerID, err)
	}

	var muteUntil int
	if mute {
		muteUntil = int(time.Now().Add(10 * 365 * 24 * time.Hour).Unix())
	}

	_, err = c.api.AccountUpdateNotifySettings(ctx, &tg.AccountUpdateNotifySettingsRequest{
		Peer: &tg.InputNotifyPeer{Peer: peer},
		Settings: tg.InputPeerNotifySettings{
			MuteUntil: muteUntil,
		},
	})
	if err != nil {
		return fmt.Errorf("mute dialog: %w", err)
	}
	return nil
}

// GetDrafts returns all dialogs that currently have a non-empty draft.
// Ported from Telethon's get_drafts() / messages.getAllDrafts.
func (c *MCUBClient) GetDrafts(ctx context.Context) ([]*types.MCUBDraft, error) {
	updates, err := c.api.MessagesGetAllDrafts(ctx)
	if err != nil {
		return nil, fmt.Errorf("get all drafts: %w", err)
	}

	var drafts []*types.MCUBDraft

	switch u := updates.(type) {
	case *tg.Updates:
		for _, upd := range u.Updates {
			if ud, ok := upd.(*tg.UpdateDraftMessage); ok {
				if draft, ok2 := ud.Draft.(*tg.DraftMessage); ok2 {
					peerID := peerToID(ud.Peer)
					drafts = append(drafts, types.NewMCUBDraft(draft, peerID))
				}
			}
		}
	}
	return drafts, nil
}

// SetDraft saves a message draft for a dialog.
// Pass replyTo=0 to set a draft without a reply context.
// Ported from Telethon's Draft.set() / messages.saveDraft.
func (c *MCUBClient) SetDraft(ctx context.Context, peerID int64, text string, replyTo int) error {
	peer, err := c.resolvePeer(ctx, peerID)
	if err != nil {
		return fmt.Errorf("resolve peer %d: %w", peerID, err)
	}

	req := &tg.MessagesSaveDraftRequest{
		Peer:    peer,
		Message: text,
	}
	if replyTo != 0 {
		req.SetReplyTo(&tg.InputReplyToMessage{ReplyToMsgID: replyTo})
	}
	_, err = c.api.MessagesSaveDraft(ctx, req)
	if err != nil {
		return fmt.Errorf("save draft: %w", err)
	}
	return nil
}

// ClearDraft clears (deletes) the draft for a dialog.
// Ported from Telethon's Draft.delete() / messages.saveDraft with empty text.
func (c *MCUBClient) ClearDraft(ctx context.Context, peerID int64) error {
	return c.SetDraft(ctx, peerID, "", 0)
}

// GetPinnedDialogs returns the pinned dialogs for the given folder.
// Use folderID=0 for the main dialog list, folderID=1 for the archive.
// Ported from Telethon's messages.getPinnedDialogs.
func (c *MCUBClient) GetPinnedDialogs(ctx context.Context, folderID int) ([]*types.MCUBDialog, error) {
	result, err := c.api.MessagesGetPinnedDialogs(ctx, folderID)
	if err != nil {
		return nil, fmt.Errorf("get pinned dialogs: %w", err)
	}

	userMap := make(map[int64]*tg.User, len(result.Users))
	for _, u := range result.Users {
		if user, ok := u.(*tg.User); ok {
			userMap[user.ID] = user
		}
	}
	chatMap := make(map[int64]tg.ChatClass, len(result.Chats))
	for _, ch := range result.Chats {
		switch v := ch.(type) {
		case *tg.Chat:
			chatMap[v.ID] = v
		case *tg.Channel:
			chatMap[v.ID] = v
		case *tg.ChatForbidden:
			chatMap[v.ID] = v
		case *tg.ChannelForbidden:
			chatMap[v.ID] = v
		}
	}
	msgMap := make(map[int]*tg.Message, len(result.Messages))
	for _, m := range result.Messages {
		if msg, ok := m.(*tg.Message); ok {
			msgMap[msg.ID] = msg
		}
	}

	out := make([]*types.MCUBDialog, 0, len(result.Dialogs))
	for _, d := range result.Dialogs {
		dlg, ok := d.(*tg.Dialog)
		if !ok {
			continue
		}
		var entity interface{}
		switch p := dlg.Peer.(type) {
		case *tg.PeerUser:
			entity = userMap[int64(p.UserID)]
		case *tg.PeerChat:
			entity = chatMap[int64(p.ChatID)]
		case *tg.PeerChannel:
			entity = chatMap[int64(p.ChannelID)]
		}
		topMsg := msgMap[dlg.TopMessage]
		out = append(out, types.NewMCUBDialog(dlg, entity, topMsg, nil, false))
	}
	return out, nil
}

// ReorderPinnedDialogs reorders the pinned dialogs for the given folder.
// peerIDs must be ordered from first-pinned to last-pinned.
// Ported from Telethon's messages.reorderPinnedDialogs.
func (c *MCUBClient) ReorderPinnedDialogs(ctx context.Context, peerIDs []int64, folderID int) error {
	order := make([]tg.InputDialogPeerClass, 0, len(peerIDs))
	for _, id := range peerIDs {
		peer, err := c.resolvePeer(ctx, id)
		if err != nil {
			return fmt.Errorf("resolve peer %d: %w", id, err)
		}
		order = append(order, &tg.InputDialogPeer{Peer: peer})
	}

	_, err := c.api.MessagesReorderPinnedDialogs(ctx, &tg.MessagesReorderPinnedDialogsRequest{
		FolderID: folderID,
		Order:    order,
		Force:    true,
	})
	if err != nil {
		return fmt.Errorf("reorder pinned dialogs: %w", err)
	}
	return nil
}

// peerToID converts a tg.PeerClass to a signed bot-API style peer ID.
func peerToID(peer tg.PeerClass) int64 {
	switch p := peer.(type) {
	case *tg.PeerUser:
		return int64(p.UserID)
	case *tg.PeerChat:
		return -int64(p.ChatID)
	case *tg.PeerChannel:
		return -1000000000000 - int64(p.ChannelID)
	}
	return 0
}
