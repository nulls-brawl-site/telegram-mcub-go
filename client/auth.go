package client

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

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
