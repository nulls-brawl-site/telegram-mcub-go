package client

import (
	"context"
	"fmt"
	"strings"

	"github.com/gotd/td/tg"
)

// GetChatInviteImporters returns the list of users who joined a chat via a specific
// invite link.  link is the full invite URL (e.g. "https://t.me/+xxx").
// limit controls the maximum number of results (0 = server default of 100).
func (c *MCUBClient) GetChatInviteImporters(ctx context.Context, chatID int64, link string, limit int) ([]interface{}, error) {
	if limit <= 0 {
		limit = 100
	}

	peer, err := c.resolvePeer(ctx, chatID)
	if err != nil {
		return nil, fmt.Errorf("resolve peer: %w", err)
	}

	result, err := c.api.MessagesGetChatInviteImporters(ctx, &tg.MessagesGetChatInviteImportersRequest{
		Peer:   peer,
		Link:   link,
		Limit:  limit,
	})
	if err != nil {
		return nil, fmt.Errorf("get chat invite importers: %w", err)
	}

	out := make([]interface{}, 0, len(result.Importers))
	for _, imp := range result.Importers {
		out = append(out, imp)
	}
	return out, nil
}

// JoinChannel joins a channel or supergroup identified by username or invite link.
// channelIDOrLink may be:
//   - A @username or bare username (e.g. "@example" or "example")
//   - An invite link (e.g. "https://t.me/joinchat/XXXX" or "t.me/+XXXX")
//
// Returns the raw tg.UpdatesClass on success.
func (c *MCUBClient) JoinChannel(ctx context.Context, channelIDOrLink string) (interface{}, error) {
	// If it looks like an invite link, use ImportChatInvite.
	if strings.Contains(channelIDOrLink, "joinchat") || strings.Contains(channelIDOrLink, "t.me/+") {
		hash := extractInviteHash(channelIDOrLink)
		return c.ImportChatInvite(ctx, hash)
	}

	// Otherwise treat as a username.
	username := strings.TrimPrefix(channelIDOrLink, "@")
	result, err := c.api.ContactsResolveUsername(ctx, &tg.ContactsResolveUsernameRequest{Username: username})
	if err != nil {
		return nil, fmt.Errorf("resolve username %q: %w", username, err)
	}

	peerChan, ok := result.Peer.(*tg.PeerChannel)
	if !ok {
		return nil, fmt.Errorf("entity %q is not a channel", channelIDOrLink)
	}

	// Find the matching channel to get the access hash.
	var inputChan tg.InputChannelClass
	for _, ch := range result.Chats {
		if channel, ok := ch.(*tg.Channel); ok && channel.ID == peerChan.ChannelID {
			inputChan = &tg.InputChannel{ChannelID: channel.ID, AccessHash: channel.AccessHash}
			break
		}
	}
	if inputChan == nil {
		return nil, fmt.Errorf("channel %q not found in resolution response", channelIDOrLink)
	}

	updates, err := c.api.ChannelsJoinChannel(ctx, inputChan)
	if err != nil {
		return nil, fmt.Errorf("join channel %q: %w", channelIDOrLink, err)
	}
	return updates, nil
}

// CheckChatInvite checks an invite link without joining.
// hash is the raw invite hash (the portion after "joinchat/" or "+").
// Returns a tg.ChatInviteClass (either *tg.ChatInvite or *tg.ChatInviteAlready).
func (c *MCUBClient) CheckChatInvite(ctx context.Context, hash string) (tg.ChatInviteClass, error) {
	result, err := c.api.MessagesCheckChatInvite(ctx, hash)
	if err != nil {
		return nil, fmt.Errorf("check chat invite %q: %w", hash, err)
	}
	return result, nil
}

// ImportChatInvite joins a chat using a raw invite hash.
// hash is the portion after "joinchat/" or "+" in the invite link.
// Returns the resulting tg.Updates.
func (c *MCUBClient) ImportChatInvite(ctx context.Context, hash string) (*tg.Updates, error) {
	result, err := c.api.MessagesImportChatInvite(ctx, hash)
	if err != nil {
		return nil, fmt.Errorf("import chat invite: %w", err)
	}
	updates, ok := result.(*tg.Updates)
	if !ok {
		return nil, fmt.Errorf("unexpected updates type %T", result)
	}
	return updates, nil
}

// extractInviteHash returns the hash part of an invite link.
// Handles forms: "https://t.me/joinchat/HASH", "t.me/+HASH", "+HASH", or bare "HASH".
func extractInviteHash(link string) string {
	// Strip protocol.
	link = strings.TrimPrefix(link, "https://")
	link = strings.TrimPrefix(link, "http://")
	link = strings.TrimPrefix(link, "t.me/joinchat/")
	link = strings.TrimPrefix(link, "t.me/+")
	link = strings.TrimPrefix(link, "+")
	// Remove any trailing path or query parameters.
	if idx := strings.IndexAny(link, "/?#"); idx >= 0 {
		link = link[:idx]
	}
	return link
}
