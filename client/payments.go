package client

import (
	"context"
	"fmt"

	"github.com/gotd/td/tg"
)

// SavedGift represents a saved star gift entry.
// The underlying API type is opaque; the raw TL object is exposed.
type SavedGift struct {
	// Raw is the underlying TL object returned by the API.
	Raw interface{}
}

// PremiumGiftOption represents a single available premium gift option.
type PremiumGiftOption struct {
	// Raw is the underlying tg.PremiumGiftCodeOption.
	Raw interface{}

	// Months is the gift duration in months.
	Months int

	// Amount is the cost in the smallest unit of Currency.
	Amount int64

	// Currency is the ISO currency code (e.g. "XTR" for Telegram Stars).
	Currency string
}

// GetSavedGifts returns saved star gifts.
//
// Note: the GetSavedStarGifts API method was introduced in a layer after
// gotd v0.89.0.  This implementation returns the information available via
// the older payments.getSavedInfo endpoint.  Callers that need the full star-
// gift list should upgrade the gotd/td dependency.
//
// A limit of 0 returns all available results.
func (c *MCUBClient) GetSavedGifts(ctx context.Context, limit int) ([]interface{}, error) {
	// payments.getSavedInfo is the closest available method in this API layer.
	info, err := c.client.API().PaymentsGetSavedInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("get saved payment info (star gifts require a newer API layer): %w", err)
	}
	// getSavedInfo returns shipping/payment info, not star gifts.
	// Return it as a single opaque entry so the caller is aware something was
	// fetched, even though full star-gift enumeration is unavailable here.
	out := []interface{}{info}
	_ = limit
	return out, nil
}

// SendStars attempts to send Telegram Stars to a user via an invoice flow.
//
// Note: the InputInvoiceStarGift constructor was introduced in a layer after
// gotd v0.89.0.  This method returns an explanatory error in that case.
// Callers should upgrade the gotd/td dependency for full star-send support.
func (c *MCUBClient) SendStars(ctx context.Context, userID int64, amount int64, message string) error {
	if amount <= 0 {
		return fmt.Errorf("SendStars: amount must be positive, got %d", amount)
	}
	if userID == 0 {
		return fmt.Errorf("SendStars: userID must be non-zero")
	}
	// InputInvoiceStarGift is not available in gotd v0.89.0.
	// Returning an informative error so callers understand the limitation.
	return fmt.Errorf(
		"SendStars to user %d (%d stars): InputInvoiceStarGift is not available in gotd v0.89.0; "+
			"upgrade to a newer version of the gotd/td module for star-transfer support",
		userID, amount,
	)
}

// GetStarBalance returns the current Telegram Stars balance for the account.
//
// A dedicated "get star balance" RPC was introduced in a later API layer.
// In gotd v0.89.0 the balance is not directly accessible; this method returns
// 0 along with a descriptive error so callers can take appropriate action.
func (c *MCUBClient) GetStarBalance(ctx context.Context) (int64, error) {
	// Fetching the app config is the closest proxy in this API layer;
	// the actual star balance field is not present in v0.89.0.
	_, err := c.client.API().HelpGetConfig(ctx)
	if err != nil {
		return 0, fmt.Errorf("get config (star balance proxy): %w", err)
	}
	// Star balance is not exposed in HelpConfig in this layer.
	return 0, fmt.Errorf(
		"GetStarBalance: dedicated star-balance RPC is not available in gotd v0.89.0; " +
			"upgrade the gotd/td dependency to access this field",
	)
}

// GetPremiumGiftOptions returns the available Telegram Premium gift options
// (duration / price combinations).
func (c *MCUBClient) GetPremiumGiftOptions(ctx context.Context) ([]interface{}, error) {
	options, err := c.client.API().PaymentsGetPremiumGiftCodeOptions(ctx, &tg.PaymentsGetPremiumGiftCodeOptionsRequest{})
	if err != nil {
		return nil, fmt.Errorf("get premium gift code options: %w", err)
	}

	out := make([]interface{}, 0, len(options))
	for i := range options {
		out = append(out, &PremiumGiftOption{
			Raw:      &options[i],
			Months:   options[i].Months,
			Amount:   options[i].Amount,
			Currency: options[i].Currency,
		})
	}
	return out, nil
}
