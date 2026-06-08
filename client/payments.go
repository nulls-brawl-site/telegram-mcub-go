// SPDX-License-Identifier: MIT
// Payments and Telegram Stars API methods.

package client

import (
	"context"
	"fmt"

	"github.com/gotd/td/tg"
)

// SavedGift represents a saved star gift entry.
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
	r, ok := receipt.(*tg.PaymentsPaymentReceipt)
	if !ok {
		return nil, fmt.Errorf("get payment receipt: unexpected type %T", receipt)
	}
	return r, nil
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

// GetStarBalance returns the current Telegram Stars balance for the authenticated account.
func (c *MCUBClient) GetStarBalance(ctx context.Context) (int64, error) {
	result, err := c.client.API().PaymentsGetStarsStatus(ctx, &tg.PaymentsGetStarsStatusRequest{
		Peer: &tg.InputPeerSelf{},
	})
	if err != nil {
		return 0, fmt.Errorf("get stars status: %w", err)
	}
	switch v := result.Balance.(type) {
	case *tg.StarsAmount:
		return v.Amount, nil
	case *tg.StarsTonAmount:
		return v.Amount, nil
	default:
		return 0, fmt.Errorf("get stars balance: unexpected balance type %T", result.Balance)
	}
}

// GetStarTransactions returns the Stars transaction history for the account.
// limit controls the maximum number of entries (0 → server default of 20).
// Set inbound/outbound to filter direction; both false = all transactions.
func (c *MCUBClient) GetStarTransactions(ctx context.Context, limit int, inbound, outbound bool) ([]*tg.StarsTransaction, error) {
	if limit <= 0 {
		limit = 20
	}
	req := &tg.PaymentsGetStarsTransactionsRequest{
		Peer:     &tg.InputPeerSelf{},
		Limit:    limit,
		Inbound:  inbound,
		Outbound: outbound,
	}
	result, err := c.client.API().PaymentsGetStarsTransactions(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("get stars transactions: %w", err)
	}
	out := make([]*tg.StarsTransaction, 0, len(result.History))
	for i := range result.History {
		out = append(out, &result.History[i])
	}
	return out, nil
}

// GetSavedGifts returns saved star gifts for the account.
// limit controls the maximum number of entries (0 → 20).
func (c *MCUBClient) GetSavedGifts(ctx context.Context, limit int) ([]*tg.SavedStarGift, error) {
	if limit <= 0 {
		limit = 20
	}
	result, err := c.client.API().PaymentsGetSavedStarGifts(ctx, &tg.PaymentsGetSavedStarGiftsRequest{
		Peer:  &tg.InputPeerSelf{},
		Limit: limit,
	})
	if err != nil {
		return nil, fmt.Errorf("get saved star gifts: %w", err)
	}
	out := make([]*tg.SavedStarGift, 0, len(result.Gifts))
	for i := range result.Gifts {
		out = append(out, &result.Gifts[i])
	}
	return out, nil
}

// SendStars sends Telegram Stars via an invoice payment form.
// formID is obtained from payments.getPaymentForm; invoice is the invoice reference.
func (c *MCUBClient) SendStars(ctx context.Context, formID int64, invoice tg.InputInvoiceClass) (tg.PaymentsPaymentResultClass, error) {
	result, err := c.client.API().PaymentsSendStarsForm(ctx, &tg.PaymentsSendStarsFormRequest{
		FormID:  formID,
		Invoice: invoice,
	})
	if err != nil {
		return nil, fmt.Errorf("send stars: %w", err)
	}
	return result, nil
}

// ExportStoryLink exports a public link to a story.
// peerID is the packed peer ID of the story author; storyID is the story ID.
func (c *MCUBClient) ExportStoryLink(ctx context.Context, peerID int64, storyID int) (string, error) {
	peer, err := c.resolvePeer(ctx, peerID)
	if err != nil {
		return "", fmt.Errorf("resolve peer: %w", err)
	}
	result, err := c.client.API().StoriesExportStoryLink(ctx, &tg.StoriesExportStoryLinkRequest{
		Peer: peer,
		ID:   storyID,
	})
	if err != nil {
		return "", fmt.Errorf("export story link: %w", err)
	}
	return result.Link, nil
}
