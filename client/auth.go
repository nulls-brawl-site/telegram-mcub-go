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

// ─────────────────────────────────────────────────────────────────────────────
// Auth methods — ported from Telethon-MCUB/telethon/client/auth.py
// ─────────────────────────────────────────────────────────────────────────────

// CodeRequest holds the result of a SendCodeRequest or ResendCode call.
// Ported from Telethon's auth.SentCode.
type CodeRequest struct {
	// PhoneCodeHash is the hash to pass to SignIn / ResendCode / CancelCode.
	PhoneCodeHash string
	// Type describes how the code was delivered: "app", "sms", "call",
	// "flash_call", "missed_call", "email_code", or "unknown".
	Type string
	// Timeout is the number of seconds before the code expires (0 if unset).
	Timeout int
	// NextType describes the delivery method that will be used if ResendCode is
	// called. Empty when no next type is advertised.
	NextType string
}

// sentCodeTypeName converts a tg.AuthSentCodeTypeClass to a human-readable string.
func sentCodeTypeName(t tg.AuthSentCodeTypeClass) string {
	if t == nil {
		return "unknown"
	}
	switch t.(type) {
	case *tg.AuthSentCodeTypeApp:
		return "app"
	case *tg.AuthSentCodeTypeSMS:
		return "sms"
	case *tg.AuthSentCodeTypeCall:
		return "call"
	case *tg.AuthSentCodeTypeFlashCall:
		return "flash_call"
	case *tg.AuthSentCodeTypeMissedCall:
		return "missed_call"
	case *tg.AuthSentCodeTypeEmailCode:
		return "email_code"
	default:
		return "unknown"
	}
}

// authCodeTypeName converts a tg.AuthCodeTypeClass (next-type) to a string.
func authCodeTypeName(t tg.AuthCodeTypeClass) string {
	if t == nil {
		return ""
	}
	switch t.(type) {
	case *tg.AuthCodeTypeSMS:
		return "sms"
	case *tg.AuthCodeTypeCall:
		return "call"
	case *tg.AuthCodeTypeFlashCall:
		return "flash_call"
	case *tg.AuthCodeTypeMissedCall:
		return "missed_call"
	default:
		return "unknown"
	}
}

// codeRequestFromSentCode builds a CodeRequest from a raw tg.AuthSentCodeClass.
func codeRequestFromSentCode(raw tg.AuthSentCodeClass) *CodeRequest {
	sc, ok := raw.(*tg.AuthSentCode)
	if !ok {
		return &CodeRequest{}
	}
	cr := &CodeRequest{
		PhoneCodeHash: sc.PhoneCodeHash,
		Type:          sentCodeTypeName(sc.Type),
	}
	if sc.Timeout != 0 {
		cr.Timeout = sc.Timeout
	}
	if next, ok2 := sc.GetNextType(); ok2 {
		cr.NextType = authCodeTypeName(next)
	}
	return cr
}

// SendCodeRequest sends a verification code to phone and returns the code metadata.
// Ported from Telethon's send_code_request().
func (c *MCUBClient) SendCodeRequest(ctx context.Context, phone string) (*CodeRequest, error) {
	raw, err := c.client.Auth().SendCode(ctx, phone, auth.SendCodeOptions{})
	if err != nil {
		return nil, fmt.Errorf("send code: %w", err)
	}
	return codeRequestFromSentCode(raw), nil
}

// ResendCode asks Telegram to resend the verification code via the next available
// method. Returns updated code metadata.
// Ported from Telethon's auth.resendCode.
func (c *MCUBClient) ResendCode(ctx context.Context, phone, phoneCodeHash string) (*CodeRequest, error) {
	raw, err := c.api.AuthResendCode(ctx, &tg.AuthResendCodeRequest{
		PhoneNumber:   phone,
		PhoneCodeHash: phoneCodeHash,
	})
	if err != nil {
		return nil, fmt.Errorf("resend code: %w", err)
	}
	return codeRequestFromSentCode(raw), nil
}

// CancelCode cancels a pending verification code request.
// Ported from Telethon's auth.cancelCode.
func (c *MCUBClient) CancelCode(ctx context.Context, phone, phoneCodeHash string) error {
	_, err := c.api.AuthCancelCode(ctx, &tg.AuthCancelCodeRequest{
		PhoneNumber:   phone,
		PhoneCodeHash: phoneCodeHash,
	})
	if err != nil {
		return fmt.Errorf("cancel code: %w", err)
	}
	return nil
}

// SignIn signs in with the phone code received after SendCodeRequest.
// Returns the authorised tg.User on success.
// Ported from Telethon's sign_in() (code path).
func (c *MCUBClient) SignIn(ctx context.Context, phone, phoneCode, phoneCodeHash string) (*tg.User, error) {
	result, err := c.client.Auth().SignIn(ctx, phone, phoneCode, phoneCodeHash)
	if err != nil {
		return nil, fmt.Errorf("sign in: %w", err)
	}
	user, ok := result.User.(*tg.User)
	if !ok {
		return nil, fmt.Errorf("unexpected user type %T", result.User)
	}
	return user, nil
}

// SignUp registers a new account using the verified phone number.
// Ported from Telethon's auth.signUp.
func (c *MCUBClient) SignUp(ctx context.Context, phone, phoneCodeHash, firstName, lastName string) (*tg.User, error) {
	result, err := c.client.Auth().SignUp(ctx, auth.SignUp{
		PhoneNumber:   phone,
		PhoneCodeHash: phoneCodeHash,
		FirstName:     firstName,
		LastName:      lastName,
	})
	if err != nil {
		return nil, fmt.Errorf("sign up: %w", err)
	}
	user, ok := result.User.(*tg.User)
	if !ok {
		return nil, fmt.Errorf("unexpected user type %T", result.User)
	}
	return user, nil
}

// SignOut logs out from the current session and invalidates the auth key.
// Ported from Telethon's log_out().
func (c *MCUBClient) SignOut(ctx context.Context) error {
	_, err := c.api.AuthLogOut(ctx)
	if err != nil {
		return fmt.Errorf("sign out: %w", err)
	}
	return nil
}

// IsUserAuthorized reports whether the current session is authenticated.
// Ported from Telethon's is_user_authorized().
func (c *MCUBClient) IsUserAuthorized(ctx context.Context) (bool, error) {
	status, err := c.client.Auth().Status(ctx)
	if err != nil {
		return false, fmt.Errorf("auth status: %w", err)
	}
	return status.Authorized, nil
}

// GetPassword returns the 2FA password settings for the current account,
// including the hint (if any). Ported from Telethon's account.getPassword.
func (c *MCUBClient) GetPassword(ctx context.Context) (*tg.AccountPassword, error) {
	pwd, err := c.api.AccountGetPassword(ctx)
	if err != nil {
		return nil, fmt.Errorf("get password: %w", err)
	}
	return pwd, nil
}

// CheckPassword verifies the 2FA password and, on success, returns the
// authorised user. Ported from Telethon's sign_in(password=...) path.
func (c *MCUBClient) CheckPassword(ctx context.Context, password string) (*tg.User, error) {
	result, err := c.client.Auth().Password(ctx, password)
	if err != nil {
		return nil, fmt.Errorf("check password: %w", err)
	}
	user, ok := result.User.(*tg.User)
	if !ok {
		return nil, fmt.Errorf("unexpected user type %T", result.User)
	}
	return user, nil
}

// Edit2FA changes, enables, or disables the 2FA cloud password.
//
//   - To enable 2FA for the first time:  currentPassword="", newPassword="secret"
//   - To change an existing password:    currentPassword="old", newPassword="new"
//   - To disable 2FA:                    currentPassword="current", newPassword=""
//
// Ported from Telethon's edit_2fa().
func (c *MCUBClient) Edit2FA(ctx context.Context, currentPassword, newPassword, hint, email string) error {
	var passwordCallback func(ctx context.Context) (string, error)
	if currentPassword != "" {
		passwordCallback = func(_ context.Context) (string, error) {
			return currentPassword, nil
		}
	}

	if newPassword == "" && currentPassword == "" {
		// Nothing to do.
		return nil
	}

	if newPassword == "" {
		// Removing password: set an empty new password hash via UpdatePassword
		// with an empty new password string. The gotd helper handles the SRP flow.
		return c.client.Auth().UpdatePassword(ctx, "", auth.UpdatePasswordOptions{
			Password: passwordCallback,
			Hint:     hint,
		})
	}

	return c.client.Auth().UpdatePassword(ctx, newPassword, auth.UpdatePasswordOptions{
		Password: passwordCallback,
		Hint:     hint,
	})
}
