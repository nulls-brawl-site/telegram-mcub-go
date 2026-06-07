package client

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"
)

// AuthOptions holds authentication configuration.
type AuthOptions struct {
	// Phone is the phone number (e.g. "+1234567890").
	// Used for user authentication.
	Phone string

	// BotToken is the bot token.
	// If set, performs bot authentication instead of user authentication.
	BotToken string

	// CodePrompt is called to request the verification code from the user.
	// If nil, reads from stdin.
	CodePrompt func(ctx context.Context, sentCode *tg.AuthSentCode) (string, error)

	// PasswordPrompt is called to request the 2FA password.
	// If nil, reads from stdin.
	PasswordPrompt func(ctx context.Context) (string, error)
}

// AuthenticateAsBot authenticates the client using a bot token.
func (c *MCUBClient) AuthenticateAsBot(ctx context.Context, token string) error {
	_, err := c.client.Auth().Bot(ctx, token)
	return err
}

// AuthenticateAsUser performs interactive user authentication.
func (c *MCUBClient) AuthenticateAsUser(ctx context.Context, opts AuthOptions) error {
	flow := auth.NewFlow(
		&mcubAuthenticator{opts: opts},
		auth.SendCodeOptions{},
	)
	return c.client.Auth().IfNecessary(ctx, flow)
}

// IsAuthorized reports whether the current session is authenticated.
func (c *MCUBClient) IsAuthorized(ctx context.Context) (bool, error) {
	_, err := c.client.Auth().Status(ctx)
	if err != nil {
		return false, err
	}
	return true, nil
}

// mcubAuthenticator implements auth.UserAuthenticator.
type mcubAuthenticator struct {
	opts AuthOptions
}

func (a *mcubAuthenticator) Phone(_ context.Context) (string, error) {
	if a.opts.Phone != "" {
		return a.opts.Phone, nil
	}
	return promptStdin("Phone number: ")
}

func (a *mcubAuthenticator) Password(_ context.Context) (string, error) {
	if a.opts.PasswordPrompt != nil {
		return a.opts.PasswordPrompt(context.Background())
	}
	return promptStdin("2FA password: ")
}

func (a *mcubAuthenticator) Code(ctx context.Context, sentCode *tg.AuthSentCode) (string, error) {
	if a.opts.CodePrompt != nil {
		return a.opts.CodePrompt(ctx, sentCode)
	}
	return promptStdin("Verification code: ")
}

func (a *mcubAuthenticator) AcceptTermsOfService(_ context.Context, tos tg.HelpTermsOfService) error {
	fmt.Printf("Terms of service: %s\nAccepting automatically.\n", tos.Text)
	return nil
}

func (a *mcubAuthenticator) SignUp(_ context.Context) (auth.UserInfo, error) {
	firstName, _ := promptStdin("First name: ")
	lastName, _ := promptStdin("Last name (optional): ")
	return auth.UserInfo{FirstName: firstName, LastName: lastName}, nil
}

func promptStdin(prompt string) (string, error) {
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("read stdin: %w", err)
	}
	return strings.TrimSpace(line), nil
}

// ─────────────────────────────────────────────────────────────────────────────
// QR Login — ported from Telethon-MCUB/telethon/tl/custom/qrlogin.py
// ─────────────────────────────────────────────────────────────────────────────

// QRLoginToken holds the data returned by the initial QR login request.
type QRLoginToken struct {
	// Token is the raw binary token from Telegram.
	Token []byte
	// Expires is the UTC time at which the QR code becomes invalid.
	Expires time.Time
	// URL is the tg://login?token=… deep-link that should be encoded as QR.
	URL string
}

// QRLoginStart requests a new QR-login token from Telegram and returns a
// QRLoginToken that the caller should render as a QR code.
//
// Equivalent to telethon's client.qr_login() / QRLogin.recreate().
func (c *MCUBClient) QRLoginStart(ctx context.Context) (*QRLoginToken, error) {
	req := &tg.AuthExportLoginTokenRequest{
		APIID:     c.options.AppID,
		APIHash:   c.options.AppHash,
		ExceptIDs: []int64{},
	}

	raw, err := c.api.AuthExportLoginToken(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("export login token: %w", err)
	}

	tok, ok := raw.(*tg.AuthLoginToken)
	if !ok {
		return nil, fmt.Errorf("unexpected response type %T", raw)
	}

	url := "tg://login?token=" + base64.RawURLEncoding.EncodeToString(tok.Token)
	return &QRLoginToken{
		Token:   tok.Token,
		Expires: time.Unix(int64(tok.Expires), 0).UTC(),
		URL:     url,
	}, nil
}

// QRLoginAccept imports a QR-login token that was scanned by another authorised
// Telegram client (e.g. the mobile app scanned the QR and called
// auth.importLoginToken on its end).
//
// This mirrors the importLoginToken path in qrlogin.py::wait().
func (c *MCUBClient) QRLoginAccept(ctx context.Context, token []byte) error {
	raw, err := c.api.AuthImportLoginToken(ctx, token)
	if err != nil {
		return fmt.Errorf("import login token: %w", err)
	}
	switch v := raw.(type) {
	case *tg.AuthLoginTokenSuccess:
		_ = v // success — session is now authorised
		return nil
	default:
		return fmt.Errorf("unexpected import result: %T", raw)
	}
}

// QRLoginWait polls Telegram until the QR code represented by token is scanned
// or ctx is cancelled.  On success the client session is authorised.
//
// Mirrors QRLogin.wait() from qrlogin.py.
func (c *MCUBClient) QRLoginWait(ctx context.Context, token *QRLoginToken) error {
	if token == nil {
		return fmt.Errorf("token must not be nil")
	}

	deadline := token.Expires
	if time.Now().After(deadline) {
		return fmt.Errorf("QR token has already expired")
	}

	// Poll every second until the token is accepted or the deadline passes.
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("QR login timed out — token expired")
		}

		// Re-export to check whether the token has been scanned.
		raw, err := c.api.AuthExportLoginToken(ctx, &tg.AuthExportLoginTokenRequest{
			APIID:     c.options.AppID,
			APIHash:   c.options.AppHash,
			ExceptIDs: []int64{},
		})
		if err != nil {
			return fmt.Errorf("poll login token: %w", err)
		}

		switch v := raw.(type) {
		case *tg.AuthLoginTokenSuccess:
			_ = v
			return nil // authorised
		case *tg.AuthLoginTokenMigrateTo:
			// DC migration: accept the new token on the target DC.
			return c.QRLoginAccept(ctx, v.Token)
		case *tg.AuthLoginToken:
			// Still waiting; update the deadline.
			deadline = time.Unix(int64(v.Expires), 0).UTC()
			token.Token = v.Token
			token.Expires = deadline
		}

		// Brief sleep before the next poll.
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
}
