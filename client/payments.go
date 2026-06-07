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

// GetSavedPaymentInfo returns the saved payment credentials / shipping info.
func (c *MCUBClient) GetSavedPaymentInfo(ctx context.Context) (*tg.PaymentsSavedInfo, error) {
	info, err := c.client.API().PaymentsGetSavedInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("get saved payment info: %w", err)
	}
	return info, nil
}

// GetPaymentReceipt retrieves a payment receipt for the given message.
// chatID is the packed peer ID of the chat; msgID is the receipt message ID.
func (c *MCUBClient) GetPaymentReceipt(ctx context.Context, chatID int64, msgID int) (*tg.PaymentsPaymentReceipt, error) {
	peer, err := c.resolvePeer(ctx, chatID)
	if err != nil {
		return nil, fmt.Errorf("resolve peer: %w", err)
	}
	receipt, err := c.client.API().PaymentsGetPaymentReceipt(ctx, &tg.PaymentsGetPaymentReceiptRequest{
		Peer:  peer,
		MsgID: msgID,
	})
	if err != nil {
		return nil, fmt.Errorf("get payment receipt: %w", err)
	}
	return receipt, nil
}

// ClearSavedInfo clears saved payment info and/or saved credentials.
// Pass info=true to clear saved payment info; credentials=true to clear saved credentials.
func (c *MCUBClient) ClearSavedInfo(ctx context.Context, info, credentials bool) error {
	req := &tg.PaymentsClearSavedInfoRequest{}
	if info {
		req.Info = true
	}
	if credentials {
		req.Credentials = true
	}
	_, err := c.client.API().PaymentsClearSavedInfo(ctx, req)
	if err != nil {
		return fmt.Errorf("clear saved info: %w", err)
	}
	return nil
}

// GetBankCardInfo returns publicly available info about a bank card number.
func (c *MCUBClient) GetBankCardInfo(ctx context.Context, number string) (*tg.PaymentsBankCardData, error) {
	data, err := c.client.API().PaymentsGetBankCardData(ctx, number)
	if err != nil {
		return nil, fmt.Errorf("get bank card info: %w", err)
	}
	return data, nil
}

// GetPremiumPromoInfo returns Telegram Premium promotional information.
func (c *MCUBClient) GetPremiumPromoInfo(ctx context.Context) (*tg.HelpPremiumPromo, error) {
	promo, err := c.client.API().HelpGetPremiumPromo(ctx)
	if err != nil {
		return nil, fmt.Errorf("get premium promo info: %w", err)
	}
	return promo, nil
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

// GetStarTransactions returns Telegram Stars payment transaction history.
//
// Note: the payments.getStarsTransactions RPC was introduced after gotd v0.89.0.
// This stub returns an informative error; upgrade the dependency for full support.
func (c *MCUBClient) GetStarTransactions(ctx context.Context, limit int) (interface{}, error) {
	return nil, fmt.Errorf(
		"GetStarTransactions: payments.getStarsTransactions is not available in gotd v0.89.0; " +
			"upgrade the gotd/td dependency to access star transactions",
	)
}

// ExportStoryLink exports a public link to a story.
// peerID is the packed peer ID of the story author; storyID is the story ID.
func (c *MCUBClient) ExportStoryLink(ctx context.Context, peerID int64, storyID int) (string, error) {
	peer, err := c.resolvePeer(ctx, peerID)
	if err != nil {
		return "", fmt.Errorf("resolve peer: %w", err)
	}
	result, err := c.client.API().StoriesExportStoryLink(ctx, &tg.StoriesExportStoryLinkRequest{
		Peer:    peer,
		ID:      storyID,
	})
	if err != nil {
		return "", fmt.Errorf("export story link: %w", err)
	}
	return result.Link, nil
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
