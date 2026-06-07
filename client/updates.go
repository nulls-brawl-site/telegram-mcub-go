package client

import (
	"context"
	"fmt"
	"sync"

	"github.com/gotd/td/tg"
)

// UpdatesConfig configures which update types the client processes.
type UpdatesConfig struct {
	ReceiveMessages    bool
	ReceiveEdits       bool
	ReceiveDeletes     bool
	ReceiveReadHistory bool
	ReceiveUserStatus  bool
	ReceiveTyping      bool
	ReceiveCallbacks   bool
	ReceiveInlines     bool
}

// updatesState holds the client's mutable update-processing configuration.
type updatesState struct {
	mu  sync.RWMutex
	cfg UpdatesConfig
}

// CatchUp sends an UpdatesTooLong signal to trigger processing of all missed
// updates since the last known state — mirroring Telethon's catch_up().
// In gotd/td the update gap detection is handled internally; calling
// UpdatesGetState is the standard way to kick off a full catch-up.
func (c *MCUBClient) CatchUp(ctx context.Context) error {
	_, err := c.api.UpdatesGetState(ctx)
	if err != nil {
		return fmt.Errorf("catch up: %w", err)
	}
	return nil
}

// GetUpdateState returns the current update sequence state (pts, date, qts, seq).
func (c *MCUBClient) GetUpdateState(ctx context.Context) (*tg.UpdatesState, error) {
	state, err := c.api.UpdatesGetState(ctx)
	if err != nil {
		return nil, fmt.Errorf("get update state: %w", err)
	}
	return state, nil
}

// GetDifference fetches the update difference starting from the given pts/date.
// The caller should walk the returned UpdatesDifferenceClass to apply updates.
func (c *MCUBClient) GetDifference(ctx context.Context, pts, date int) (tg.UpdatesDifferenceClass, error) {
	diff, err := c.api.UpdatesGetDifference(ctx, &tg.UpdatesGetDifferenceRequest{
		Pts:  pts,
		Date: date,
		Qts:  0,
	})
	if err != nil {
		return nil, fmt.Errorf("get difference: %w", err)
	}
	return diff, nil
}

// GetChannelDifference fetches the update difference for a specific channel.
// channelID is the raw channel ID (without the -100 prefix).
// pts is the channel's last known pts.
// limit controls the maximum number of events returned (0 = server default).
func (c *MCUBClient) GetChannelDifference(ctx context.Context, channelID int64, pts int, limit int) (tg.UpdatesChannelDifferenceClass, error) {
	if limit <= 0 {
		limit = 100
	}
	diff, err := c.api.UpdatesGetChannelDifference(ctx, &tg.UpdatesGetChannelDifferenceRequest{
		Channel: &tg.InputChannel{ChannelID: channelID},
		Filter:  &tg.ChannelMessagesFilterEmpty{},
		Pts:     pts,
		Limit:   limit,
	})
	if err != nil {
		return nil, fmt.Errorf("get channel difference %d: %w", channelID, err)
	}
	return diff, nil
}

// ConfigureUpdates stores an UpdatesConfig on the client for use by event
// handlers and middleware. The configuration is applied lazily: individual
// handler registrations should respect the flags when filtering incoming
// updates.
func (c *MCUBClient) ConfigureUpdates(cfg UpdatesConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.updCfg = cfg
}

// GetUpdatesConfig returns the current UpdatesConfig.
func (c *MCUBClient) GetUpdatesConfig() UpdatesConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.updCfg
}
