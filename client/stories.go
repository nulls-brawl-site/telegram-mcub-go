package client

import (
	"context"
	"fmt"
	"math/rand"

	"github.com/gotd/td/tg"
)

// Story wraps a tg.StoryItemClass for ergonomic access.
type Story struct {
	// ID is the story identifier.
	ID int

	// PeerID is the packed peer ID of the story author.
	PeerID int64

	// Raw is the underlying TL story item object.
	Raw tg.StoryItemClass
}

// GetStories returns the active stories for the given peer.
// peerID may be a user ID (positive) or channel packed ID.
func (c *MCUBClient) GetStories(ctx context.Context, peerID int64) ([]*Story, error) {
	peer, err := c.resolvePeer(ctx, peerID)
	if err != nil {
		return nil, fmt.Errorf("resolve peer: %w", err)
	}
	result, err := c.client.API().StoriesGetPeerStories(ctx, peer)
	if err != nil {
		return nil, fmt.Errorf("get stories for peer %d: %w", peerID, err)
	}
	return peerStoriesToStories(peerID, result.Stories.Stories), nil
}

// GetAllStories returns stories from all subscribed contacts/channels.
// The first call should pass state="" and next=false.
// Subsequent calls use the state returned in AllStoriesResult.NextState with next=true.
type AllStoriesResult struct {
	// Stories is the flat list of story items across all peers.
	Stories []*Story

	// NextState is the pagination cursor for the next call.
	// Empty string means there are no more stories.
	NextState string

	// HasMore indicates whether more stories are available.
	HasMore bool
}

// GetAllStories returns a page of stories from subscribed peers.
func (c *MCUBClient) GetAllStories(ctx context.Context, next bool, state string) (*AllStoriesResult, error) {
	req := &tg.StoriesGetAllStoriesRequest{}
	if state != "" {
		req.SetState(state)
	}
	req.Next = next

	raw, err := c.client.API().StoriesGetAllStories(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("get all stories: %w", err)
	}

	result := &AllStoriesResult{}

	switch v := raw.(type) {
	case *tg.StoriesAllStories:
		result.HasMore = v.HasMore
		result.NextState = v.State
		for _, ps := range v.PeerStories {
			peerID := peerStoryPeerID(ps.Peer)
			result.Stories = append(result.Stories, peerStoriesToStories(peerID, ps.Stories)...)
		}
	case *tg.StoriesAllStoriesNotModified:
		result.NextState = v.State
	}

	return result, nil
}

// SendStoryParams holds the parameters for sending a story.
type SendStoryParams struct {
	// PeerID is the peer on whose behalf to post the story (0 = self).
	PeerID int64

	// Media is the story media (photo or video input).
	Media tg.InputMediaClass

	// Caption is the optional story caption.
	Caption string

	// Period is the TTL in seconds (86400 = 24 h, 0 = server default).
	Period int

	// Pinned keeps the story on the profile after the TTL expires.
	Pinned bool

	// NoForwards prevents forwarding the story.
	NoForwards bool

	// PrivacyRules controls who can see the story.
	// Pass nil for the default (contacts only).
	PrivacyRules []tg.InputPrivacyRuleClass
}

// SendStory posts a new story on behalf of peerID (or self if 0).
// The Media field must be populated before calling.
func (c *MCUBClient) SendStory(ctx context.Context, params SendStoryParams) (*tg.UpdatesClass, error) {
	peer, err := c.resolvePeer(ctx, params.PeerID)
	if err != nil {
		return nil, fmt.Errorf("resolve peer: %w", err)
	}

	if params.Media == nil {
		return nil, fmt.Errorf("SendStory: Media must not be nil")
	}

	privacyRules := params.PrivacyRules
	if privacyRules == nil {
		// Default: contacts only
		privacyRules = []tg.InputPrivacyRuleClass{&tg.InputPrivacyValueAllowContacts{}}
	}

	req := &tg.StoriesSendStoryRequest{
		Peer:         peer,
		Media:        params.Media,
		PrivacyRules: privacyRules,
		RandomID:     rand.Int63(),
		Pinned:       params.Pinned,
		Noforwards:   params.NoForwards,
	}
	if params.Caption != "" {
		req.SetCaption(params.Caption)
	}
	if params.Period > 0 {
		req.SetPeriod(params.Period)
	}

	upd, err := c.client.API().StoriesSendStory(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("send story: %w", err)
	}
	return &upd, nil
}

// DeleteStory deletes one or more of the current user's stories.
// peerID is the peer that owns the stories (0 = self).
func (c *MCUBClient) DeleteStory(ctx context.Context, peerID int64, storyIDs []int) error {
	if len(storyIDs) == 0 {
		return nil
	}
	peer, err := c.resolvePeer(ctx, peerID)
	if err != nil {
		return fmt.Errorf("resolve peer: %w", err)
	}
	_, err = c.client.API().StoriesDeleteStories(ctx, &tg.StoriesDeleteStoriesRequest{
		Peer: peer,
		ID:   storyIDs,
	})
	if err != nil {
		return fmt.Errorf("delete stories: %w", err)
	}
	return nil
}

// ReactToStory sends an emoji reaction to a story.
// peerID is the packed peer ID of the story's author.
func (c *MCUBClient) ReactToStory(ctx context.Context, peerID int64, storyID int, emoji string) error {
	peer, err := c.resolvePeer(ctx, peerID)
	if err != nil {
		return fmt.Errorf("resolve peer: %w", err)
	}
	req := &tg.StoriesSendReactionRequest{
		Peer:        peer,
		StoryID:     storyID,
		Reaction:    &tg.ReactionEmoji{Emoticon: emoji},
		AddToRecent: true,
	}
	_, err = c.client.API().StoriesSendReaction(ctx, req)
	if err != nil {
		return fmt.Errorf("react to story %d: %w", storyID, err)
	}
	return nil
}

// CanSendStory checks whether the account is allowed to post a story for the given peer.
func (c *MCUBClient) CanSendStory(ctx context.Context, peerID int64) (bool, error) {
	peer, err := c.resolvePeer(ctx, peerID)
	if err != nil {
		return false, fmt.Errorf("resolve peer: %w", err)
	}
	result, err := c.client.API().StoriesCanSendStory(ctx, peer)
	if err != nil {
		return false, fmt.Errorf("can send story: %w", err)
	}
	return result.CountRemains > 0, nil
}

// peerStoriesToStories converts raw story items to []*Story, attaching peerID.
func peerStoriesToStories(peerID int64, items []tg.StoryItemClass) []*Story {
	out := make([]*Story, 0, len(items))
	for _, item := range items {
		s := &Story{Raw: item, PeerID: peerID}
		if si, ok := item.(*tg.StoryItem); ok {
			s.ID = si.ID
		}
		out = append(out, s)
	}
	return out
}

// peerStoryPeerID extracts a packed peer ID from a tg.PeerClass.
func peerStoryPeerID(peer tg.PeerClass) int64 {
	switch p := peer.(type) {
	case *tg.PeerUser:
		return int64(p.UserID)
	case *tg.PeerChannel:
		return -1000000000000 - int64(p.ChannelID)
	case *tg.PeerChat:
		return -int64(p.ChatID)
	}
	return 0
}
