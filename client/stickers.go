package client

import (
	"context"
	"fmt"

	"github.com/gotd/td/tg"
)

// GetStickerSet returns detailed information about a sticker set identified by
// its short name (e.g. "Animals").
func (c *MCUBClient) GetStickerSet(ctx context.Context, shortName string) (*tg.MessagesStickerSet, error) {
	result, err := c.client.API().MessagesGetStickerSet(ctx, &tg.MessagesGetStickerSetRequest{
		Stickerset: &tg.InputStickerSetShortName{ShortName: shortName},
		Hash:       0,
	})
	if err != nil {
		return nil, fmt.Errorf("get sticker set %q: %w", shortName, err)
	}
	set, ok := result.(*tg.MessagesStickerSet)
	if !ok {
		return nil, fmt.Errorf("sticker set %q not found or not modified", shortName)
	}
	return set, nil
}

// GetFeaturedStickerSets returns the currently featured / trending sticker sets.
// limit controls the maximum number of sets returned (0 = server default).
// The returned value is a *tg.MessagesFeaturedStickers; callers may also receive
// *tg.MessagesFeaturedStickersNotModified when the cached hash matches.
func (c *MCUBClient) GetFeaturedStickerSets(ctx context.Context, limit int) (*tg.MessagesFeaturedStickers, error) {
	result, err := c.client.API().MessagesGetFeaturedStickers(ctx, 0)
	if err != nil {
		return nil, fmt.Errorf("get featured sticker sets: %w", err)
	}
	featured, ok := result.(*tg.MessagesFeaturedStickers)
	if !ok {
		// MessagesFeaturedStickersNotModified — return empty result.
		return &tg.MessagesFeaturedStickers{}, nil
	}
	if limit > 0 && len(featured.Sets) > limit {
		featured.Sets = featured.Sets[:limit]
	}
	return featured, nil
}

// GetInstalledStickers returns all sticker sets currently installed by the user.
func (c *MCUBClient) GetInstalledStickers(ctx context.Context) ([]tg.StickerSetCoveredClass, error) {
	result, err := c.client.API().MessagesGetAllStickers(ctx, 0)
	if err != nil {
		return nil, fmt.Errorf("get installed stickers: %w", err)
	}
	all, ok := result.(*tg.MessagesAllStickers)
	if !ok {
		// MessagesAllStickersNotModified
		return nil, nil
	}
	// Convert StickerSet to StickerSetCovered wrappers for API consistency.
	out := make([]tg.StickerSetCoveredClass, 0, len(all.Sets))
	for i := range all.Sets {
		out = append(out, &tg.StickerSetCovered{Set: all.Sets[i]})
	}
	return out, nil
}

// InstallStickerSet installs the sticker set identified by shortName.
// Pass archived=true to archive the set instead of making it active.
func (c *MCUBClient) InstallStickerSet(ctx context.Context, shortName string) error {
	_, err := c.client.API().MessagesInstallStickerSet(ctx, &tg.MessagesInstallStickerSetRequest{
		Stickerset: &tg.InputStickerSetShortName{ShortName: shortName},
		Archived:   false,
	})
	if err != nil {
		return fmt.Errorf("install sticker set %q: %w", shortName, err)
	}
	return nil
}

// UninstallStickerSet removes the sticker set identified by shortName from the
// user's installed sets.
func (c *MCUBClient) UninstallStickerSet(ctx context.Context, shortName string) error {
	_, err := c.client.API().MessagesUninstallStickerSet(ctx, &tg.InputStickerSetShortName{ShortName: shortName})
	if err != nil {
		return fmt.Errorf("uninstall sticker set %q: %w", shortName, err)
	}
	return nil
}
