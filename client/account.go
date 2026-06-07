package client

import (
	"context"
	"fmt"
	"time"

	"github.com/gotd/td/tg"
)

// privacyKeyFromString converts a privacy key name to a tg.InputPrivacyKeyClass.
// Supported keys: "phone", "status", "pfp", "forwards", "calls", "groups", "about".
func privacyKeyFromString(key string) (tg.InputPrivacyKeyClass, error) {
	switch key {
	case "phone":
		return &tg.InputPrivacyKeyPhoneNumber{}, nil
	case "status":
		return &tg.InputPrivacyKeyStatusTimestamp{}, nil
	case "pfp", "photo":
		return &tg.InputPrivacyKeyProfilePhoto{}, nil
	case "forwards":
		return &tg.InputPrivacyKeyForwards{}, nil
	case "calls":
		return &tg.InputPrivacyKeyPhoneCall{}, nil
	case "groups":
		return &tg.InputPrivacyKeyChatInvite{}, nil
	case "about":
		return &tg.InputPrivacyKeyAbout{}, nil
	default:
		return nil, fmt.Errorf("unknown privacy key %q", key)
	}
}

// GetPrivacySettings returns the current privacy settings for the given key.
// Supported keys: "phone", "status", "pfp", "forwards", "calls", "groups", "about".
func (c *MCUBClient) GetPrivacySettings(ctx context.Context, key string) (interface{}, error) {
	privKey, err := privacyKeyFromString(key)
	if err != nil {
		return nil, err
	}

	result, err := c.api.AccountGetPrivacy(ctx, privKey)
	if err != nil {
		return nil, fmt.Errorf("get privacy settings: %w", err)
	}
	return result, nil
}

// SetPrivacySettings sets the privacy rule for a given key.
//
// rule can be: "everyone", "contacts", "nobody".
// allowIDs and disallowIDs are user IDs that are explicitly allowed or denied,
// and are applied on top of the rule.
func (c *MCUBClient) SetPrivacySettings(ctx context.Context, key string, rule string, allowIDs, disallowIDs []int64) error {
	privKey, err := privacyKeyFromString(key)
	if err != nil {
		return err
	}

	var rules []tg.InputPrivacyRuleClass

	switch rule {
	case "everyone":
		rules = append(rules, &tg.InputPrivacyValueAllowAll{})
	case "contacts":
		rules = append(rules, &tg.InputPrivacyValueAllowContacts{})
	case "nobody":
		rules = append(rules, &tg.InputPrivacyValueDisallowAll{})
	default:
		return fmt.Errorf("unknown privacy rule %q; use 'everyone', 'contacts', or 'nobody'", rule)
	}

	if len(allowIDs) > 0 {
		users := make([]tg.InputUserClass, 0, len(allowIDs))
		for _, uid := range allowIDs {
			users = append(users, &tg.InputUser{UserID: uid})
		}
		rules = append(rules, &tg.InputPrivacyValueAllowUsers{Users: users})
	}

	if len(disallowIDs) > 0 {
		users := make([]tg.InputUserClass, 0, len(disallowIDs))
		for _, uid := range disallowIDs {
			users = append(users, &tg.InputUser{UserID: uid})
		}
		rules = append(rules, &tg.InputPrivacyValueDisallowUsers{Users: users})
	}

	_, err = c.api.AccountSetPrivacy(ctx, &tg.AccountSetPrivacyRequest{
		Key:   privKey,
		Rules: rules,
	})
	return err
}

// GetBlockedUsers returns up to limit blocked users.
// Pass limit <= 0 to use the default of 100.
func (c *MCUBClient) GetBlockedUsers(ctx context.Context, limit int) ([]*tg.User, error) {
	if limit <= 0 {
		limit = 100
	}

	result, err := c.api.ContactsGetBlocked(ctx, &tg.ContactsGetBlockedRequest{
		Offset: 0,
		Limit:  limit,
	})
	if err != nil {
		return nil, fmt.Errorf("get blocked users: %w", err)
	}

	var userList []tg.UserClass
	switch r := result.(type) {
	case *tg.ContactsBlocked:
		userList = r.Users
	case *tg.ContactsBlockedSlice:
		userList = r.Users
	}

	out := make([]*tg.User, 0, len(userList))
	for _, u := range userList {
		if user, ok := u.(*tg.User); ok {
			out = append(out, user)
		}
	}
	return out, nil
}

// BlockUser adds a user to the block list.
func (c *MCUBClient) BlockUser(ctx context.Context, userID int64) error {
	_, err := c.api.ContactsBlock(ctx, &tg.ContactsBlockRequest{
		ID: &tg.InputPeerUser{UserID: userID},
	})
	if err != nil {
		return fmt.Errorf("block user %d: %w", userID, err)
	}
	return nil
}

// UnblockUser removes a user from the block list.
func (c *MCUBClient) UnblockUser(ctx context.Context, userID int64) error {
	_, err := c.api.ContactsUnblock(ctx, &tg.ContactsUnblockRequest{
		ID: &tg.InputPeerUser{UserID: userID},
	})
	if err != nil {
		return fmt.Errorf("unblock user %d: %w", userID, err)
	}
	return nil
}

// GetNotifySettings returns notification settings for the given peer.
func (c *MCUBClient) GetNotifySettings(ctx context.Context, peerID int64) (*tg.PeerNotifySettings, error) {
	peer, err := c.resolvePeer(ctx, peerID)
	if err != nil {
		return nil, fmt.Errorf("resolve peer: %w", err)
	}

	result, err := c.api.AccountGetNotifySettings(ctx, &tg.InputNotifyPeer{Peer: peer})
	if err != nil {
		return nil, fmt.Errorf("get notify settings: %w", err)
	}
	return result, nil
}

// MuteChat mutes notifications from a peer for the given duration in seconds.
// Pass seconds = 0 to unmute.
func (c *MCUBClient) MuteChat(ctx context.Context, peerID int64, seconds int) error {
	peer, err := c.resolvePeer(ctx, peerID)
	if err != nil {
		return fmt.Errorf("resolve peer: %w", err)
	}

	settings := tg.InputPeerNotifySettings{}

	if seconds == 0 {
		// Unmute: set MuteUntil to 0.
		settings.SetMuteUntil(0)
	} else {
		muteUntil := int(time.Now().Unix()) + seconds
		settings.SetMuteUntil(muteUntil)
	}

	_, err = c.api.AccountUpdateNotifySettings(ctx, &tg.AccountUpdateNotifySettingsRequest{
		Peer:     &tg.InputNotifyPeer{Peer: peer},
		Settings: settings,
	})
	if err != nil {
		return fmt.Errorf("mute chat: %w", err)
	}
	return nil
}

// GetTwoFAStatus returns whether two-factor authentication (cloud password) is enabled.
func (c *MCUBClient) GetTwoFAStatus(ctx context.Context) (bool, error) {
	result, err := c.api.AccountGetPassword(ctx)
	if err != nil {
		return false, fmt.Errorf("get 2FA status: %w", err)
	}
	return result.HasPassword, nil
}

// GetSessions returns all active login sessions.
func (c *MCUBClient) GetSessions(ctx context.Context) ([]*tg.Authorization, error) {
	result, err := c.api.AccountGetAuthorizations(ctx)
	if err != nil {
		return nil, fmt.Errorf("get sessions: %w", err)
	}

	out := make([]*tg.Authorization, 0, len(result.Authorizations))
	for i := range result.Authorizations {
		out = append(out, &result.Authorizations[i])
	}
	return out, nil
}

// TerminateSession terminates a specific active session by its hash.
func (c *MCUBClient) TerminateSession(ctx context.Context, hash int64) error {
	_, err := c.api.AccountResetAuthorization(ctx, hash)
	if err != nil {
		return fmt.Errorf("terminate session: %w", err)
	}
	return nil
}

// TerminateAllOtherSessions terminates all active sessions except the current one.
func (c *MCUBClient) TerminateAllOtherSessions(ctx context.Context) error {
	_, err := c.api.AuthResetAuthorizations(ctx)
	if err != nil {
		return fmt.Errorf("terminate all sessions: %w", err)
	}
	return nil
}

// DeleteAccount permanently deletes the Telegram account.
// reason is an optional explanation; it can be empty.
// Note: if 2FA is enabled and no password is provided, deletion is delayed by 7 days.
func (c *MCUBClient) DeleteAccount(ctx context.Context, reason string) error {
	req := &tg.AccountDeleteAccountRequest{
		Reason: reason,
	}
	_, err := c.api.AccountDeleteAccount(ctx, req)
	if err != nil {
		return fmt.Errorf("delete account: %w", err)
	}
	return nil
}

// SetGlobalTTL sets the auto-delete timer (TTL) for all new private chats.
// days is the number of days; pass 0 to disable.
func (c *MCUBClient) SetGlobalTTL(ctx context.Context, days int) error {
	period := days * 24 * 60 * 60
	_, err := c.api.MessagesSetDefaultHistoryTTL(ctx, period)
	if err != nil {
		return fmt.Errorf("set global TTL: %w", err)
	}
	return nil
}
