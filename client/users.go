package client

import (
	"context"
	"fmt"
	"time"

	"github.com/gotd/td/tg"
)

// ContactInfo holds a single contact entry for bulk import.
type ContactInfo struct {
	Phone     string
	FirstName string
	LastName  string
	ClientID  int64
}

// UserStatus represents a user's online presence.
type UserStatus struct {
	Online   bool
	LastSeen time.Time
	Expires  time.Time // populated when the user is currently online
}

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

// GetContacts returns the caller's contact list.
func (c *MCUBClient) GetContacts(ctx context.Context) ([]*tg.User, error) {
	result, err := c.api.ContactsGetContacts(ctx, 0)
	if err != nil {
		return nil, fmt.Errorf("get contacts: %w", err)
	}
	contacts, ok := result.(*tg.ContactsContacts)
	if !ok {
		return nil, nil // ContactsNotModified
	}
	out := make([]*tg.User, 0, len(contacts.Users))
	for _, u := range contacts.Users {
		if user, ok := u.(*tg.User); ok {
			out = append(out, user)
		}
	}
	return out, nil
}

// AddContact adds a user as a contact by first/last name and optional phone number.
func (c *MCUBClient) AddContact(ctx context.Context, userID int64, firstName, lastName, phone string) error {
	_, err := c.api.ContactsAddContact(ctx, &tg.ContactsAddContactRequest{
		ID:        &tg.InputUser{UserID: userID},
		FirstName: firstName,
		LastName:  lastName,
		Phone:     phone,
	})
	if err != nil {
		return fmt.Errorf("add contact %d: %w", userID, err)
	}
	return nil
}

// DeleteContact removes a user from the contact list.
func (c *MCUBClient) DeleteContact(ctx context.Context, userID int64) error {
	_, err := c.api.ContactsDeleteContacts(ctx, []tg.InputUserClass{
		&tg.InputUser{UserID: userID},
	})
	if err != nil {
		return fmt.Errorf("delete contact %d: %w", userID, err)
	}
	return nil
}

// ImportContacts bulk-imports phone contacts and returns matched Telegram users.
func (c *MCUBClient) ImportContacts(ctx context.Context, contacts []ContactInfo) ([]*tg.User, error) {
	tlContacts := make([]tg.InputPhoneContact, len(contacts))
	for i, c := range contacts {
		tlContacts[i] = tg.InputPhoneContact{
			ClientID:  c.ClientID,
			Phone:     c.Phone,
			FirstName: c.FirstName,
			LastName:  c.LastName,
		}
	}
	result, err := c.api.ContactsImportContacts(ctx, tlContacts)
	if err != nil {
		return nil, fmt.Errorf("import contacts: %w", err)
	}
	out := make([]*tg.User, 0, len(result.Users))
	for _, u := range result.Users {
		if user, ok := u.(*tg.User); ok {
			out = append(out, user)
		}
	}
	return out, nil
}

// GetCommonChats returns the chats shared between the caller and userID.
// limit controls the maximum number of results (0 = server default of 100).
func (c *MCUBClient) GetCommonChats(ctx context.Context, userID int64, limit int) ([]interface{}, error) {
	if limit <= 0 {
		limit = 100
	}
	result, err := c.api.MessagesGetCommonChats(ctx, &tg.MessagesGetCommonChatsRequest{
		UserID: &tg.InputUser{UserID: userID},
		MaxID:  0,
		Limit:  limit,
	})
	if err != nil {
		return nil, fmt.Errorf("get common chats with %d: %w", userID, err)
	}
	var chats []tg.ChatClass
	switch r := result.(type) {
	case *tg.MessagesChats:
		chats = r.Chats
	case *tg.MessagesChatsSlice:
		chats = r.Chats
	}
	out := make([]interface{}, len(chats))
	for i, ch := range chats {
		out[i] = ch
	}
	return out, nil
}

// ReportUser reports a user to Telegram for abuse.
// reason should be one of: "spam", "violence", "pornography", "child_abuse",
// "copyright", "geo_irrelevant", "fake", "illegal_drugs", "personal_details", "other".
func (c *MCUBClient) ReportUser(ctx context.Context, userID int64, reason string) error {
	peer := &tg.InputPeerUser{UserID: userID}
	var tlReason tg.ReportReasonClass
	switch reason {
	case "spam":
		tlReason = &tg.InputReportReasonSpam{}
	case "violence":
		tlReason = &tg.InputReportReasonViolence{}
	case "pornography":
		tlReason = &tg.InputReportReasonPornography{}
	case "child_abuse":
		tlReason = &tg.InputReportReasonChildAbuse{}
	case "copyright":
		tlReason = &tg.InputReportReasonCopyright{}
	case "geo_irrelevant":
		tlReason = &tg.InputReportReasonGeoIrrelevant{}
	case "fake":
		tlReason = &tg.InputReportReasonFake{}
	case "illegal_drugs":
		tlReason = &tg.InputReportReasonIllegalDrugs{}
	case "personal_details":
		tlReason = &tg.InputReportReasonPersonalDetails{}
	default:
		tlReason = &tg.InputReportReasonOther{}
	}
	_, err := c.api.AccountReportPeer(ctx, &tg.AccountReportPeerRequest{
		Peer:   peer,
		Reason: tlReason,
	})
	if err != nil {
		return fmt.Errorf("report user %d: %w", userID, err)
	}
	return nil
}

// GetUserStatus returns the online/offline status of a user.
func (c *MCUBClient) GetUserStatus(ctx context.Context, userID int64) (*UserStatus, error) {
	user, err := c.GetUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	status := &UserStatus{}
	if user.Status == nil {
		return status, nil
	}
	switch s := user.Status.(type) {
	case *tg.UserStatusOnline:
		status.Online = true
		status.Expires = time.Unix(int64(s.Expires), 0)
	case *tg.UserStatusOffline:
		status.Online = false
		status.LastSeen = time.Unix(int64(s.WasOnline), 0)
	case *tg.UserStatusRecently:
		status.Online = false
	case *tg.UserStatusLastWeek:
		status.Online = false
	case *tg.UserStatusLastMonth:
		status.Online = false
	}
	return status, nil
}

// ResolveUsername resolves a @username to the matching peer entity.
// The returned value is one of *tg.User, *tg.Chat, or *tg.Channel.
func (c *MCUBClient) ResolveUsername(ctx context.Context, username string) (interface{}, error) {
	// Strip leading '@' if present.
	if len(username) > 0 && username[0] == '@' {
		username = username[1:]
	}
	result, err := c.api.ContactsResolveUsername(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("resolve username @%s: %w", username, err)
	}
	switch p := result.Peer.(type) {
	case *tg.PeerUser:
		for _, u := range result.Users {
			if user, ok := u.(*tg.User); ok && user.ID == p.UserID {
				return user, nil
			}
		}
	case *tg.PeerChat:
		for _, ch := range result.Chats {
			if chat, ok := ch.(*tg.Chat); ok && chat.ID == p.ChatID {
				return chat, nil
			}
		}
	case *tg.PeerChannel:
		for _, ch := range result.Chats {
			if channel, ok := ch.(*tg.Channel); ok && channel.ID == p.ChannelID {
				return channel, nil
			}
		}
	}
	return nil, fmt.Errorf("username @%s not found in response", username)
}

// GetFullUser returns the full user profile for userID including bio and other extended fields.
func (c *MCUBClient) GetFullUser(ctx context.Context, userID int64) (*tg.UsersUserFull, error) {
	result, err := c.api.UsersGetFullUser(ctx, &tg.InputUser{UserID: userID})
	if err != nil {
		return nil, fmt.Errorf("get full user %d: %w", userID, err)
	}
	return result, nil
}
