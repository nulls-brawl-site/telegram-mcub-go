package client

import (
	"context"
	"fmt"

	"github.com/gotd/td/tg"
)

// GetContactsCount returns the total number of contacts in the user's contact list.
func (c *MCUBClient) GetContactsCount(ctx context.Context) (int, error) {
	result, err := c.api.ContactsGetContacts(ctx, 0)
	if err != nil {
		return 0, fmt.Errorf("get contacts count: %w", err)
	}
	switch r := result.(type) {
	case *tg.ContactsContacts:
		return len(r.Contacts), nil
	case *tg.ContactsContactsNotModified:
		return 0, nil
	}
	return 0, nil
}

// SearchContacts searches the user's contacts by name or phone number.
// limit controls the maximum number of results (0 = server default of 100).
func (c *MCUBClient) SearchContacts(ctx context.Context, query string, limit int) ([]*tg.User, error) {
	if limit <= 0 {
		limit = 100
	}
	result, err := c.api.ContactsSearch(ctx, &tg.ContactsSearchRequest{
		Q:     query,
		Limit: limit,
	})
	if err != nil {
		return nil, fmt.Errorf("search contacts %q: %w", query, err)
	}
	out := make([]*tg.User, 0, len(result.Users))
	for _, u := range result.Users {
		if user, ok := u.(*tg.User); ok {
			out = append(out, user)
		}
	}
	return out, nil
}

// GetTopPeers returns the frequently contacted peers from Telegram's "top peers" feature.
// Set correspondents=true to return the top correspondent peers (chats with direct messages).
// limit controls the maximum number of results per category.
// Returns a slice of tg.TopPeerCategoryPeers (one per category).
func (c *MCUBClient) GetTopPeers(ctx context.Context, correspondents bool, limit int) ([]interface{}, error) {
	if limit <= 0 {
		limit = 30
	}
	req := &tg.ContactsGetTopPeersRequest{
		Correspondents: correspondents,
		Limit:          limit,
		Offset:         0,
		Hash:           0,
	}
	result, err := c.api.ContactsGetTopPeers(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("get top peers: %w", err)
	}
	var out []interface{}
	switch r := result.(type) {
	case *tg.ContactsTopPeers:
		for i := range r.Categories {
			out = append(out, &r.Categories[i])
		}
	case *tg.ContactsTopPeersNotModified:
		// no-op
	case *tg.ContactsTopPeersDisabled:
		// feature disabled server-side
	}
	return out, nil
}

// ResetTopPeerRating resets the usage statistics for Telegram's "top peers" ranking.
func (c *MCUBClient) ResetTopPeerRating(ctx context.Context) error {
	_, err := c.api.ContactsResetTopPeerRating(ctx, &tg.ContactsResetTopPeerRatingRequest{
		Category: &tg.TopPeerCategoryCorrespondents{},
		Peer:     &tg.InputPeerEmpty{},
	})
	if err != nil {
		return fmt.Errorf("reset top peer rating: %w", err)
	}
	return nil
}

// GetWebAuthorizations returns all active Telegram web login sessions.
func (c *MCUBClient) GetWebAuthorizations(ctx context.Context) ([]*tg.WebAuthorization, error) {
	result, err := c.api.AccountGetWebAuthorizations(ctx)
	if err != nil {
		return nil, fmt.Errorf("get web authorizations: %w", err)
	}
	out := make([]*tg.WebAuthorization, 0, len(result.Authorizations))
	for i := range result.Authorizations {
		out = append(out, &result.Authorizations[i])
	}
	return out, nil
}

// ResetWebAuthorization terminates the web login session identified by hash.
func (c *MCUBClient) ResetWebAuthorization(ctx context.Context, hash int64) error {
	_, err := c.api.AccountResetWebAuthorization(ctx, hash)
	if err != nil {
		return fmt.Errorf("reset web authorization: %w", err)
	}
	return nil
}

// GetAutoDownloadSettings returns the current media auto-download settings.
func (c *MCUBClient) GetAutoDownloadSettings(ctx context.Context) (*tg.AccountAutoDownloadSettings, error) {
	result, err := c.api.AccountGetAutoDownloadSettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("get auto download settings: %w", err)
	}
	return result, nil
}

// SaveAutoDownloadSettings updates the media auto-download settings.
// Pass the low, medium, and high preset settings as returned by GetAutoDownloadSettings.
// Set the preset you want to update; nil presets are not changed.
func (c *MCUBClient) SaveAutoDownloadSettings(ctx context.Context, low, medium, high *tg.AutoDownloadSettings) error {
	if low != nil {
		if _, err := c.api.AccountSaveAutoDownloadSettings(ctx, &tg.AccountSaveAutoDownloadSettingsRequest{
			Low:      true,
			Settings: *low,
		}); err != nil {
			return fmt.Errorf("save auto download settings (low): %w", err)
		}
	}
	if medium != nil {
		if _, err := c.api.AccountSaveAutoDownloadSettings(ctx, &tg.AccountSaveAutoDownloadSettingsRequest{
			Settings: *medium,
		}); err != nil {
			return fmt.Errorf("save auto download settings (medium): %w", err)
		}
	}
	if high != nil {
		if _, err := c.api.AccountSaveAutoDownloadSettings(ctx, &tg.AccountSaveAutoDownloadSettingsRequest{
			High:     true,
			Settings: *high,
		}); err != nil {
			return fmt.Errorf("save auto download settings (high): %w", err)
		}
	}
	return nil
}

// GetSuggestedDialogFilters returns Telegram's suggested dialog folder filters.
func (c *MCUBClient) GetSuggestedDialogFilters(ctx context.Context) ([]*tg.DialogFilterSuggested, error) {
	result, err := c.api.MessagesGetSuggestedDialogFilters(ctx)
	if err != nil {
		return nil, fmt.Errorf("get suggested dialog filters: %w", err)
	}
	out := make([]*tg.DialogFilterSuggested, 0, len(result))
	for i := range result {
		out = append(out, &result[i])
	}
	return out, nil
}

// UpdateDialogFilter creates or updates a dialog folder (filter).
// id is the folder ID; filter holds the folder configuration.
func (c *MCUBClient) UpdateDialogFilter(ctx context.Context, id int, filter *tg.DialogFilter) error {
	req := &tg.MessagesUpdateDialogFilterRequest{
		ID: id,
	}
	if filter != nil {
		req.SetFilter(filter)
	}
	_, err := c.api.MessagesUpdateDialogFilter(ctx, req)
	if err != nil {
		return fmt.Errorf("update dialog filter %d: %w", id, err)
	}
	return nil
}

// DeleteDialogFilter deletes the dialog folder with the given id.
func (c *MCUBClient) DeleteDialogFilter(ctx context.Context, id int) error {
	// Deleting = updating without the filter flag set.
	_, err := c.api.MessagesUpdateDialogFilter(ctx, &tg.MessagesUpdateDialogFilterRequest{
		ID: id,
	})
	if err != nil {
		return fmt.Errorf("delete dialog filter %d: %w", id, err)
	}
	return nil
}

// GetDialogFilters returns all dialog folders configured by the user.
func (c *MCUBClient) GetDialogFilters(ctx context.Context) ([]tg.DialogFilterClass, error) {
	result, err := c.api.MessagesGetDialogFilters(ctx)
	if err != nil {
		return nil, fmt.Errorf("get dialog filters: %w", err)
	}
	out := make([]tg.DialogFilterClass, 0, len(result.Filters))
	for _, f := range result.Filters {
		out = append(out, f)
	}
	return out, nil
}
