// Package client provides the MCUBClient, a gotd/td wrapper with MCUB-specific extensions.
//
// MCUBClient adds:
//   - Event and request middleware chains
//   - Protection/security profiles (off, safe, strict, custom) with ProtectionPolicy
//   - Forum topic helpers (iter, get, create, send)
//   - Resumable file downloads and uploads
//   - Reaction methods
//   - JoinRequest events
//   - History export utilities
package client

import (
	"context"
	"fmt"
	"sync"

	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/tg"

	mcubevents "github.com/nulls-brawl-site/telegram-mcub-go/events"
	"github.com/nulls-brawl-site/telegram-mcub-go/types"
)

// EventFilter is an alias for events.Filter for ergonomic use from this package.
type EventFilter = mcubevents.Filter

// EventHandler is an alias for events.Handler.
type EventHandler = mcubevents.Handler

// MCUBClient is the central client struct. It wraps gotd/td's telegram.Client
// and adds MCUB-specific features such as middleware chains, protection profiles,
// forum topic helpers, resumable transfers, and reaction helpers.
type MCUBClient struct {
	// client is the underlying gotd/td Telegram client.
	client *telegram.Client

	// api is the raw TL API accessor (shorthand for c.client.API()).
	api *tg.Client

	// dispatcher manages event subscriptions and middleware.
	dispatcher *mcubevents.Dispatcher

	// eventMiddlewares is the ordered list of event middleware functions.
	eventMiddlewares []EventMiddlewareFunc

	// requestMiddlewares is the ordered list of request middleware functions.
	requestMiddlewares []telegram.Middleware

	// protectionMode is the active protection mode.
	protectionMode types.ProtectionMode

	// protectionPolicy is the active protection policy (derived from mode or custom).
	protectionPolicy types.ProtectionPolicy

	// mu guards mutable fields.
	mu sync.RWMutex

	// updCfg holds the active updates processing configuration.
	updCfg UpdatesConfig

	// options holds the configuration used to build this client.
	options Options
}

// Options configures the MCUBClient.
type Options struct {
	// AppID is the Telegram API application ID.
	AppID int

	// AppHash is the Telegram API application hash.
	AppHash string

	// Session is the session storage backend.
	// Defaults to an in-memory session when nil.
	Session session.Storage

	// ProtectionMode sets the initial protection mode.
	// Defaults to ProtectionSafe.
	ProtectionMode types.ProtectionMode

	// ProtectionPolicy is used when ProtectionMode is ProtectionCustom.
	ProtectionPolicy types.ProtectionPolicy

	// Logger is an optional logger. Must implement Printf(format string, args ...interface{}).
	Logger interface{ Printf(string, ...interface{}) }

	// ExtraMiddlewares are additional request middlewares added at construction time.
	ExtraMiddlewares []telegram.Middleware
}

// New creates and returns a new MCUBClient.
func New(opts Options) (*MCUBClient, error) {
	if opts.AppID == 0 {
		return nil, fmt.Errorf("AppID is required")
	}
	if opts.AppHash == "" {
		return nil, fmt.Errorf("AppHash is required")
	}

	c := &MCUBClient{
		dispatcher:     mcubevents.NewDispatcher(),
		protectionMode: opts.ProtectionMode,
		options:        opts,
	}

	// Determine protection policy.
	if opts.ProtectionMode == types.ProtectionCustom {
		c.protectionPolicy = opts.ProtectionPolicy
	} else {
		c.protectionPolicy = types.PolicyForMode(opts.ProtectionMode)
	}

	// Compose gotd middleware list.
	var middlewares []telegram.Middleware
	if opts.ProtectionMode != types.ProtectionOff {
		middlewares = append(middlewares, protectionMiddleware(c.protectionPolicy))
	}
	middlewares = append(middlewares, opts.ExtraMiddlewares...)
	c.requestMiddlewares = middlewares

	// Build gotd client options.
	tdOpts := telegram.Options{
		Middlewares: middlewares,
	}
	if opts.Session != nil {
		tdOpts.SessionStorage = opts.Session
	}

	c.client = telegram.NewClient(opts.AppID, opts.AppHash, tdOpts)
	c.api = c.client.API()

	return c, nil
}

// Run connects the client to Telegram and starts event processing.
// The provided function f is called once the client is ready; Run blocks until
// f returns or the context is cancelled.
func (c *MCUBClient) Run(ctx context.Context, f func(ctx context.Context) error) error {
	return c.client.Run(ctx, func(ctx context.Context) error {
		return f(ctx)
	})
}

// Connect is a convenience wrapper that calls Run with a blocking function.
// The client stays connected until the context is cancelled.
func (c *MCUBClient) Connect(ctx context.Context) error {
	return c.Run(ctx, func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	})
}

// Self returns information about the authenticated user/bot.
func (c *MCUBClient) Self(ctx context.Context) (*tg.User, error) {
	result, err := c.api.UsersGetFullUser(ctx, &tg.InputUserSelf{})
	if err != nil {
		return nil, fmt.Errorf("get self: %w", err)
	}
	for _, u := range result.Users {
		if user, ok := u.(*tg.User); ok && user.Self {
			return user, nil
		}
	}
	return nil, fmt.Errorf("self user not found in response")
}

// GetMe is an alias for Self.
func (c *MCUBClient) GetMe(ctx context.Context) (*tg.User, error) {
	return c.Self(ctx)
}

// GetEntity resolves a username to a Telegram peer.
func (c *MCUBClient) GetEntity(ctx context.Context, username string) (tg.UserClass, error) {
	result, err := c.api.ContactsResolveUsername(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("resolve username %q: %w", username, err)
	}
	if len(result.Users) > 0 {
		return result.Users[0], nil
	}
	return nil, fmt.Errorf("entity %q not found", username)
}

// AddEventHandler registers an event handler with an optional filter.
// If filter is nil, the handler receives all events.
func (c *MCUBClient) AddEventHandler(filter EventFilter, handler EventHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.dispatcher.AddHandler(filter, handler)
}

// AddEventMiddleware appends an event middleware to the chain.
// Middlewares are called in the order they are added, innermost first.
func (c *MCUBClient) AddEventMiddleware(middleware EventMiddlewareFunc) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.eventMiddlewares = append(c.eventMiddlewares, middleware)
	c.dispatcher.AddMiddleware(middleware)
}

// RemoveEventMiddleware removes an event middleware. Because Go function values
// are not comparable by pointer, this removes by position. Prefer
// RemoveEventMiddlewareAt for deterministic removal.
func (c *MCUBClient) RemoveEventMiddleware(middleware EventMiddlewareFunc) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.dispatcher.RemoveMiddleware(middleware)
}

// RemoveEventMiddlewareAt removes the event middleware at the given index.
func (c *MCUBClient) RemoveEventMiddlewareAt(index int) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if index < 0 || index >= len(c.eventMiddlewares) {
		return fmt.Errorf("middleware index %d out of range [0, %d)", index, len(c.eventMiddlewares))
	}
	c.eventMiddlewares = append(c.eventMiddlewares[:index], c.eventMiddlewares[index+1:]...)
	return nil
}

// AddRequestMiddleware appends a request middleware to the chain.
// Note: request middlewares are baked into the gotd client at construction time
// via Options.ExtraMiddlewares. This method records the middleware but cannot
// retroactively inject it into the live connection. Rebuild the client to apply
// new request middlewares to the underlying transport.
func (c *MCUBClient) AddRequestMiddleware(middleware telegram.Middleware) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.requestMiddlewares = append(c.requestMiddlewares, middleware)
}

// SetProtectionMode changes the active protection mode at runtime.
// This updates the policy record but does not affect the underlying gotd
// connection middleware chain (which is fixed at construction time).
func (c *MCUBClient) SetProtectionMode(mode types.ProtectionMode) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.protectionMode = mode
	if mode != types.ProtectionCustom {
		c.protectionPolicy = types.PolicyForMode(mode)
	}
}

// SetProtectionPolicy sets a custom ProtectionPolicy and switches mode to Custom.
func (c *MCUBClient) SetProtectionPolicy(policy types.ProtectionPolicy) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.protectionMode = types.ProtectionCustom
	c.protectionPolicy = policy
}

// ProtectionMode returns the current protection mode.
func (c *MCUBClient) ProtectionMode() types.ProtectionMode {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.protectionMode
}

// ProtectionPolicy returns the current protection policy.
func (c *MCUBClient) ProtectionPolicy() types.ProtectionPolicy {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.protectionPolicy
}

// API returns the raw tg.Client for direct API calls.
// Most users should prefer the higher-level helpers on MCUBClient.
func (c *MCUBClient) API() *tg.Client {
	return c.api
}

// dispatch routes a tg update through the event system.
func (c *MCUBClient) dispatch(ctx context.Context, u tg.UpdateClass) error {
	if ev, ok := mcubevents.NewMessageFromUpdate(ctx, u); ok {
		return c.dispatcher.Dispatch(ctx, ev)
	}
	if ev, ok := mcubevents.MessageEditedFromUpdate(ctx, u); ok {
		return c.dispatcher.Dispatch(ctx, ev)
	}
	if ev, ok := mcubevents.JoinRequestFromUpdate(ctx, u); ok {
		return c.dispatcher.Dispatch(ctx, ev)
	}
	if ev, ok := mcubevents.CallbackQueryFromUpdate(ctx, u); ok {
		ev.Answerer = c.api
		return c.dispatcher.Dispatch(ctx, ev)
	}
	if ev, ok := mcubevents.InlineQueryFromUpdate(ctx, u); ok {
		return c.dispatcher.Dispatch(ctx, ev)
	}
	return nil
}

// HandleUpdates processes a batch of raw Telegram updates.
// Wire this to the gotd UpdateHandlerFunc to receive events via the MCUBClient.
func (c *MCUBClient) HandleUpdates(ctx context.Context, updates tg.UpdatesClass) error {
	switch u := updates.(type) {
	case *tg.Updates:
		for _, upd := range u.Updates {
			if err := c.dispatch(ctx, upd); err != nil {
				return err
			}
		}
	case *tg.UpdatesCombined:
		for _, upd := range u.Updates {
			if err := c.dispatch(ctx, upd); err != nil {
				return err
			}
		}
	case *tg.UpdateShort:
		if err := c.dispatch(ctx, u.Update); err != nil {
			return err
		}
	}
	return nil
}
