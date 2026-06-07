package client

import (
	"context"
	"fmt"
	"time"

	"github.com/gotd/td/tg"

	"github.com/nulls-brawl-site/telegram-mcub-go/types"
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

// --- Entity resolution ---

// GetInputEntity resolves a variety of entity representations to an InputPeer.
// The entity argument may be:
//   - int64: interpreted as a peer ID (positive = user, negative = chat/channel)
//   - string: interpreted as a @username (leading @ is stripped)
//   - tg.InputPeerClass: returned as-is
//   - tg.UserClass, tg.ChatClass: converted to the corresponding InputPeer
func (c *MCUBClient) GetInputEntity(ctx context.Context, entity interface{}) (tg.InputPeerClass, error) {
	switch v := entity.(type) {
	case tg.InputPeerClass:
		return v, nil
	case *tg.InputPeerUser:
		return v, nil
	case *tg.InputPeerChat:
		return v, nil
	case *tg.InputPeerChannel:
		return v, nil
	case *tg.User:
		return &tg.InputPeerUser{UserID: v.ID, AccessHash: v.AccessHash}, nil
	case *tg.Channel:
		return &tg.InputPeerChannel{ChannelID: v.ID, AccessHash: v.AccessHash}, nil
	case *tg.Chat:
		return &tg.InputPeerChat{ChatID: v.ID}, nil
	case int64:
		peer, err := c.resolvePeer(ctx, v)
		if err != nil {
			return nil, fmt.Errorf("get input entity %d: %w", v, err)
		}
		return peer, nil
	case string:
		username := v
		if len(username) > 0 && username[0] == '@' {
			username = username[1:]
		}
		result, err := c.api.ContactsResolveUsername(ctx, username)
		if err != nil {
			return nil, fmt.Errorf("resolve username %q: %w", v, err)
		}
		switch p := result.Peer.(type) {
		case *tg.PeerUser:
			return &tg.InputPeerUser{UserID: p.UserID}, nil
		case *tg.PeerChat:
			return &tg.InputPeerChat{ChatID: p.ChatID}, nil
		case *tg.PeerChannel:
			return &tg.InputPeerChannel{ChannelID: p.ChannelID}, nil
		}
		return nil, fmt.Errorf("entity %q not found", v)
	default:
		return nil, fmt.Errorf("unsupported entity type %T", entity)
	}
}

// GetInputPeer resolves a numeric peer ID to an InputPeer.
// This is an alias for GetInputEntity with an int64 argument.
func (c *MCUBClient) GetInputPeer(ctx context.Context, peerID int64) (tg.InputPeerClass, error) {
	return c.GetInputEntity(ctx, peerID)
}

// --- Participant permissions ---

// GetPermissions returns a ParticipantPermissions for userID inside chatID.
// chatID must be a supergroup/channel (peer ID < -999999999).
func (c *MCUBClient) GetPermissions(ctx context.Context, chatID, userID int64) (*types.ParticipantPermissions, error) {
	participant, err := c.getChannelParticipant(ctx, chatID, userID)
	if err != nil {
		return nil, err
	}
	return types.NewParticipantPermissions(participant, false), nil
}

// GetParticipantFull returns the raw ChannelParticipantClass for a user in a channel.
func (c *MCUBClient) GetParticipantFull(ctx context.Context, chatID, userID int64) (interface{}, error) {
	return c.getChannelParticipant(ctx, chatID, userID)
}

// getChannelParticipant fetches the raw participant record from a channel/supergroup.
func (c *MCUBClient) getChannelParticipant(ctx context.Context, chatID, userID int64) (tg.ChannelParticipantClass, error) {
	channel, err := c.resolveInputChannel(ctx, chatID)
	if err != nil {
		return nil, err
	}
	result, err := c.api.ChannelsGetParticipant(ctx, &tg.ChannelsGetParticipantRequest{
		Channel:     channel,
		Participant: &tg.InputPeerUser{UserID: userID},
	})
	if err != nil {
		return nil, fmt.Errorf("get participant %d in %d: %w", userID, chatID, err)
	}
	return result.Participant, nil
}

// GetAdminRights returns the ChatAdminRights for a user if they are an admin,
// or nil if they are not. Returns an error if the participant cannot be fetched.
func (c *MCUBClient) GetAdminRights(ctx context.Context, chatID, userID int64) (interface{}, error) {
	participant, err := c.getChannelParticipant(ctx, chatID, userID)
	if err != nil {
		return nil, err
	}
	switch v := participant.(type) {
	case *tg.ChannelParticipantAdmin:
		return &v.AdminRights, nil
	case *tg.ChannelParticipantCreator:
		return &v.AdminRights, nil
	}
	return nil, nil
}

// --- EditAdmin ---

// EditAdminParams holds the parameters for promoting or demoting an admin.
type EditAdminParams struct {
	// ChatID is the channel or supergroup peer ID.
	ChatID int64
	// UserID is the target user.
	UserID int64

	// Admin rights flags.
	ChangeInfo     bool
	PostMessages   bool
	EditMessages   bool
	DeleteMessages bool
	BanUsers       bool
	InviteUsers    bool
	PinMessages    bool
	AddAdmins      bool
	Anonymous      bool
	ManageCall     bool

	// Rank is the custom admin title (empty = no title).
	Rank string

	// Demote removes all admin rights when true (all rights flags are ignored).
	Demote bool
}

// EditAdmin promotes or demotes a user in a channel or supergroup.
// Set Demote = true to remove all admin privileges.
func (c *MCUBClient) EditAdmin(ctx context.Context, params EditAdminParams) error {
	channel, err := c.resolveInputChannel(ctx, params.ChatID)
	if err != nil {
		return err
	}

	var rights tg.ChatAdminRights
	if !params.Demote {
		rights = tg.ChatAdminRights{
			ChangeInfo:     params.ChangeInfo,
			PostMessages:   params.PostMessages,
			EditMessages:   params.EditMessages,
			DeleteMessages: params.DeleteMessages,
			BanUsers:       params.BanUsers,
			InviteUsers:    params.InviteUsers,
			PinMessages:    params.PinMessages,
			AddAdmins:      params.AddAdmins,
			Anonymous:      params.Anonymous,
			ManageCall:     params.ManageCall,
		}
	}

	_, err = c.api.ChannelsEditAdmin(ctx, &tg.ChannelsEditAdminRequest{
		Channel:     channel,
		UserID:      &tg.InputUser{UserID: params.UserID},
		AdminRights: rights,
		Rank:        params.Rank,
	})
	if err != nil {
		return fmt.Errorf("edit admin %d in %d: %w", params.UserID, params.ChatID, err)
	}
	return nil
}

// --- IsBot / IsMutualContact ---

// IsBot returns true if the user identified by userID is a bot account.
func (c *MCUBClient) IsBot(ctx context.Context, userID int64) (bool, error) {
	user, err := c.GetUser(ctx, userID)
	if err != nil {
		return false, err
	}
	return user.Bot, nil
}

// IsMutualContact returns true if the given user has the current account in
// their own contact list (i.e. the contact relationship is mutual).
func (c *MCUBClient) IsMutualContact(ctx context.Context, userID int64) (bool, error) {
	user, err := c.GetUser(ctx, userID)
	if err != nil {
		return false, err
	}
	return user.MutualContact, nil
}
