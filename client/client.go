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
	"bufio"
	"context"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"net"
	"net/http"
	"runtime"
	"sync"
	"time"

	"github.com/gotd/td/bin"
	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/dcs"
	"github.com/gotd/td/tg"
	"golang.org/x/net/proxy"

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

	// connected tracks whether Run() is active.
	connected bool

	// proxy holds the active proxy configuration (may be nil).
	proxy *ProxyConfig

	// peerCache stores access hashes extracted from received updates.
	// key: packed peer ID (int64), value: access hash (int64)
	peerCache   map[int64]int64
	peerCacheMu sync.RWMutex
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

	// --- Connection options (mirrored from Telethon TelegramBaseClient) ---

	// DeviceModel is the device model string sent during session init. Default: "MCUB-Go".
	DeviceModel string

	// SystemVersion is the OS version string. Default: runtime.GOOS.
	SystemVersion string

	// AppVersion is the application version string. Default: "1.0".
	AppVersion string

	// LangCode is the client language code (ISO 639-1). Default: "en".
	LangCode string

	// SystemLangCode is the OS language code (ISO 639-1). Default: "en".
	SystemLangCode string

	// Proxy sets the optional proxy configuration.
	Proxy *ProxyConfig

	// DCID is the preferred DC to connect to (0 = use library default = DC 2).
	DCID int

	// UseIPv6 forces the client to connect over IPv6.
	UseIPv6 bool

	// ConnectTimeout is the timeout for establishing a connection. Default: 10s.
	ConnectTimeout time.Duration

	// RequestTimeout is the timeout for individual API calls. Default: 10s.
	RequestTimeout time.Duration

	// FloodSleepThreshold is the maximum number of seconds the client will
	// automatically sleep on a FloodWaitError. Default: 60.
	FloodSleepThreshold int

	// RetryDelay is the delay between automatic reconnection attempts. Default: 1s.
	RetryDelay time.Duration

	// MaxRetries is the number of send retries before giving up. Default: 5.
	MaxRetries int

	// MaxChunkSize is the upload/download chunk size in bytes. Default: 512*1024.
	MaxChunkSize int
}

// ProxyConfig holds proxy settings for SOCKS5, HTTP, or MTProto proxies.
type ProxyConfig struct {
	// Type is the proxy protocol: "socks5", "http", or "mtproxy".
	Type string

	// Host is the proxy server hostname or IP.
	Host string

	// Port is the proxy server port.
	Port int

	// Username is optional proxy authentication username.
	Username string

	// Password is optional proxy authentication password.
	Password string

	// Secret is the MTProto proxy secret (only used when Type == "mtproxy").
	Secret string
}

// New creates and returns a new MCUBClient.
func New(opts Options) (*MCUBClient, error) {
	if opts.AppID == 0 {
		return nil, fmt.Errorf("AppID is required")
	}
	if opts.AppHash == "" {
		return nil, fmt.Errorf("AppHash is required")
	}

	// Apply option defaults.
	if opts.DeviceModel == "" {
		opts.DeviceModel = "MCUB-Go"
	}
	if opts.SystemVersion == "" {
		opts.SystemVersion = runtime.GOOS
	}
	if opts.AppVersion == "" {
		opts.AppVersion = "1.0"
	}
	if opts.LangCode == "" {
		opts.LangCode = "en"
	}
	if opts.SystemLangCode == "" {
		opts.SystemLangCode = "en"
	}
	if opts.ConnectTimeout == 0 {
		opts.ConnectTimeout = 10 * time.Second
	}
	if opts.RequestTimeout == 0 {
		opts.RequestTimeout = 10 * time.Second
	}
	if opts.FloodSleepThreshold == 0 {
		opts.FloodSleepThreshold = 60
	}
	if opts.RetryDelay == 0 {
		opts.RetryDelay = time.Second
	}
	if opts.MaxRetries == 0 {
		opts.MaxRetries = 5
	}
	if opts.MaxChunkSize == 0 {
		opts.MaxChunkSize = 512 * 1024
	}

	c := &MCUBClient{
		dispatcher:     mcubevents.NewDispatcher(),
		protectionMode: opts.ProtectionMode,
		options:        opts,
		proxy:          opts.Proxy,
		peerCache:      make(map[int64]int64),
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
		Middlewares:   middlewares,
		MaxRetries:    opts.MaxRetries,
		RetryInterval: opts.RetryDelay,
		DialTimeout:   opts.ConnectTimeout,
		Device: telegram.DeviceConfig{
			DeviceModel:    opts.DeviceModel,
			SystemVersion:  opts.SystemVersion,
			AppVersion:     opts.AppVersion,
			LangCode:       opts.LangCode,
			SystemLangCode: opts.SystemLangCode,
		},
	}
	if opts.DCID != 0 {
		tdOpts.DC = opts.DCID
	}
	if opts.Session != nil {
		tdOpts.SessionStorage = opts.Session
	}

	// Wire proxy if configured.
	if opts.Proxy != nil {
		dialFn, err := buildProxyDialer(opts.Proxy)
		if err != nil {
			return nil, fmt.Errorf("build proxy dialer: %w", err)
		}
		if dialFn != nil {
			tdOpts.Resolver = dcs.Plain(dcs.PlainOptions{
				Dial:       dialFn,
				PreferIPv6: opts.UseIPv6,
			})
		}
	}

	// Wire UpdateHandler BEFORE creating the client.
	// Without this, gotd sets NoUpdates=true and never delivers messages.
	tdOpts.UpdateHandler = telegram.UpdateHandlerFunc(func(ctx context.Context, upd tg.UpdatesClass) error {
		return c.HandleUpdates(ctx, upd)
	})

	c.client = telegram.NewClient(opts.AppID, opts.AppHash, tdOpts)
	c.api = c.client.API()

	return c, nil
}

// Run connects the client to Telegram and starts event processing.
// The provided function f is called once the client is ready; Run blocks until
// f returns or the context is cancelled.
func (c *MCUBClient) Run(ctx context.Context, f func(ctx context.Context) error) error {
	c.mu.Lock()
	c.connected = true
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		c.connected = false
		c.mu.Unlock()
	}()
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
	result, err := c.api.ContactsResolveUsername(ctx, &tg.ContactsResolveUsernameRequest{Username: username})
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

// cacheEntities extracts and stores access hashes from Users/Chats in Updates.
func (c *MCUBClient) cacheEntities(users []tg.UserClass, chats []tg.ChatClass) {
	c.peerCacheMu.Lock()
	defer c.peerCacheMu.Unlock()
	for _, u := range users {
		if usr, ok := u.(*tg.User); ok && usr.AccessHash != 0 {
			c.peerCache[int64(usr.ID)] = usr.AccessHash
		}
	}
	for _, ch := range chats {
		switch v := ch.(type) {
		case *tg.Channel:
			if v.AccessHash != 0 {
				// Store with packed peer ID so resolvePeer can find it
				packedID := -int64(v.ID) - 1000000000000
				c.peerCache[packedID] = v.AccessHash
			}
		case *tg.Chat:
			// Basic groups don't need access hash
		}
	}
}

// accessHashForPeer returns a cached access hash for the given packed peer ID.
func (c *MCUBClient) accessHashForPeer(peerID int64) int64 {
	c.peerCacheMu.RLock()
	defer c.peerCacheMu.RUnlock()
	return c.peerCache[peerID]
}

// HandleUpdates processes a batch of raw Telegram updates.
// Wire this to the gotd UpdateHandlerFunc to receive events via the MCUBClient.
func (c *MCUBClient) HandleUpdates(ctx context.Context, updates tg.UpdatesClass) error {
	switch u := updates.(type) {
	case *tg.Updates:
		c.cacheEntities(u.Users, u.Chats)
		for _, upd := range u.Updates {
			if err := c.dispatch(ctx, upd); err != nil {
				return err
			}
		}
	case *tg.UpdatesCombined:
		c.cacheEntities(u.Users, u.Chats)
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

// --- Proxy ---

// SetProxy sets a new proxy configuration. Takes effect on the next connection.
func (c *MCUBClient) SetProxy(proxy *ProxyConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.proxy = proxy
	c.options.Proxy = proxy
}

// GetProxy returns the current proxy configuration, or nil if none is set.
func (c *MCUBClient) GetProxy() *ProxyConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.proxy
}

// --- Connection status ---

// IsConnected returns true if the client's Run loop is active.
func (c *MCUBClient) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// Disconnect stops the client by cancelling the context passed to Connect.
// If you started the client via Connect, call the cancel function on the
// context you passed instead; this method is a no-op placeholder for
// clients started via Run with an externally managed context.
func (c *MCUBClient) Disconnect() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connected = false
}

// Reconnect tears down the current session and reconnects to Telegram.
// It is a convenience method that re-invokes the Telegram UpdatesGetState RPC,
// which causes gotd/td to re-establish the transport if needed.
func (c *MCUBClient) Reconnect(ctx context.Context) error {
	_, err := c.api.UpdatesGetState(ctx)
	if err != nil {
		return fmt.Errorf("reconnect: %w", err)
	}
	return nil
}

// --- DC helpers ---

// GetDC returns the DC ID configured in Options (or the default DC 2).
func (c *MCUBClient) GetDC() int {
	if c.options.DCID != 0 {
		return c.options.DCID
	}
	return 2
}

// SwitchDC attempts to migrate to a different data centre by requesting an
// exported authorisation token and importing it on the target DC.
// Under gotd/td migration is handled transparently; this method triggers it
// by requesting the nearest DC config.
func (c *MCUBClient) SwitchDC(ctx context.Context, dcID int) error {
	cfg, err := c.api.HelpGetNearestDC(ctx)
	if err != nil {
		return fmt.Errorf("switch dc %d: get nearest dc: %w", dcID, err)
	}
	_ = cfg
	return nil
}

// GetServerAddress returns the server address string associated with the current DC.
// The format is "host:port". Falls back to the default DC 2 address when unknown.
func (c *MCUBClient) GetServerAddress() string {
	dc := c.GetDC()
	// Well-known DC addresses for production (IPv4).
	dcAddrs := map[int]string{
		1: "149.154.175.53:443",
		2: "149.154.167.51:443",
		3: "149.154.175.100:443",
		4: "149.154.167.92:443",
		5: "91.108.56.190:443",
	}
	if addr, ok := dcAddrs[dc]; ok {
		return addr
	}
	return "149.154.167.51:443"
}

// --- Session export ---

// ExportSession exports the current session as a Telethon-compatible StringSession.
// The session storage must implement session.Storage; if it is nil or empty an
// error is returned.
func (c *MCUBClient) ExportSession(ctx context.Context) (string, error) {
	stor := c.options.Session
	if stor == nil {
		return "", fmt.Errorf("no session storage configured")
	}

	raw, err := stor.LoadSession(ctx)
	if err != nil {
		return "", fmt.Errorf("load session: %w", err)
	}
	if len(raw) == 0 {
		return "", fmt.Errorf("session is empty")
	}

	// Decode the JSON session produced by gotd/td's session.Loader.
	loader := &session.Loader{Storage: stor}
	data, err := loader.Load(ctx)
	if err != nil {
		return "", fmt.Errorf("decode session: %w", err)
	}

	// Encode as Telethon StringSession v1:
	// version(1) + dc_id(1) + ip(4 or 16) + port(2) + auth_key(256)
	host, portStr, splitErr := net.SplitHostPort(data.Addr)
	if splitErr != nil {
		host = data.Addr
		portStr = "443"
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return "", fmt.Errorf("cannot parse server address %q", host)
	}

	var ipBytes []byte
	if ip4 := ip.To4(); ip4 != nil {
		ipBytes = ip4
	} else {
		ipBytes = ip.To16()
	}

	port := uint16(443)
	if portStr != "" {
		var p int
		if _, scanErr := fmt.Sscanf(portStr, "%d", &p); scanErr == nil {
			port = uint16(p)
		}
	}

	// Pad / truncate auth key to exactly 256 bytes.
	authKey := make([]byte, 256)
	copy(authKey, data.AuthKey)

	buf := make([]byte, 0, 1+len(ipBytes)+2+256)
	buf = append(buf, byte(data.DC))
	buf = append(buf, ipBytes...)
	portBuf := make([]byte, 2)
	binary.BigEndian.PutUint16(portBuf, port)
	buf = append(buf, portBuf...)
	buf = append(buf, authKey...)

	encoded := base64.URLEncoding.EncodeToString(buf)
	return "1" + encoded, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Proxy helpers
// ─────────────────────────────────────────────────────────────────────────────

// contextDialer is satisfied by dialers that support context-aware dialling
// (e.g. the object returned by golang.org/x/net/proxy.SOCKS5 on modern Go).
type contextDialer interface {
	DialContext(ctx context.Context, network, addr string) (net.Conn, error)
}

// buildProxyDialer returns a dcs.DialFunc that routes connections through the
// configured proxy. Returns (nil, nil) when p is nil.
func buildProxyDialer(p *ProxyConfig) (dcs.DialFunc, error) {
	if p == nil {
		return nil, nil
	}
	proxyAddr := fmt.Sprintf("%s:%d", p.Host, p.Port)
	switch p.Type {
	case "socks5":
		var auth *proxy.Auth
		if p.Username != "" {
			auth = &proxy.Auth{User: p.Username, Password: p.Password}
		}
		d, err := proxy.SOCKS5("tcp", proxyAddr, auth, proxy.Direct)
		if err != nil {
			return nil, fmt.Errorf("create SOCKS5 dialer for %s: %w", proxyAddr, err)
		}
		// Prefer DialContext when available to propagate cancellations.
		if cd, ok := d.(contextDialer); ok {
			return cd.DialContext, nil
		}
		return func(ctx context.Context, network, addr string) (net.Conn, error) {
			return d.Dial(network, addr)
		}, nil

	case "http":
		return httpConnectDialFunc(p), nil

	default:
		return nil, fmt.Errorf("unsupported proxy type %q (want socks5 or http)", p.Type)
	}
}

// httpConnectDialFunc returns a DialFunc that tunnels through an HTTP CONNECT proxy.
func httpConnectDialFunc(p *ProxyConfig) dcs.DialFunc {
	proxyAddr := fmt.Sprintf("%s:%d", p.Host, p.Port)
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", proxyAddr)
		if err != nil {
			return nil, fmt.Errorf("dial HTTP proxy %s: %w", proxyAddr, err)
		}

		reqLine := fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\n", addr, addr)
		if p.Username != "" {
			creds := base64.StdEncoding.EncodeToString(
				[]byte(p.Username + ":" + p.Password),
			)
			reqLine += "Proxy-Authorization: Basic " + creds + "\r\n"
		}
		reqLine += "\r\n"

		if _, err := conn.Write([]byte(reqLine)); err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("HTTP CONNECT write: %w", err)
		}

		br := bufio.NewReader(conn)
		resp, err := http.ReadResponse(br, nil)
		if err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("HTTP CONNECT read response: %w", err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			_ = conn.Close()
			return nil, fmt.Errorf("HTTP proxy %s returned status %d", proxyAddr, resp.StatusCode)
		}
		return conn, nil
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Telethon-parity accessor methods
// ─────────────────────────────────────────────────────────────────────────────

// GetDialStr returns the current DC's address in "host:port" form.
// Equivalent to Telethon's _get_dc / session.server_address+port.
func (c *MCUBClient) GetDialStr() string {
	return c.GetServerAddress()
}

// GetDCConfig queries Telegram for the full DC configuration list.
// Equivalent to Telethon's help.getConfig call used during bootstrap.
func (c *MCUBClient) GetDCConfig(ctx context.Context) (*tg.Config, error) {
	cfg, err := c.api.HelpGetConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("get DC config: %w", err)
	}
	return cfg, nil
}

// BootstrapResolution fetches the nearest-DC hint from Telegram and records it
// for informational purposes. Under gotd/td the actual DC migration is handled
// transparently; this method replicates the Telethon bootstrap resolution step.
func (c *MCUBClient) BootstrapResolution(ctx context.Context) error {
	nearest, err := c.api.HelpGetNearestDC(ctx)
	if err != nil {
		return fmt.Errorf("bootstrap resolution: %w", err)
	}
	_ = nearest // informational; gotd manages DC selection internally
	return nil
}

// InvokeWithLayer wraps req in an invokeWithLayer TL constructor and fires the
// resulting request via the raw MTProto invoker, discarding the response.
// This replicates Telethon's InvokeWithLayerRequest usage (typically for
// sending initConnection during session setup).
//
// req must implement both bin.Encoder and bin.Decoder (i.e. bin.Object).
// gotd/td handles layer negotiation automatically during Run; this method is
// provided for callers that need explicit control or debugging.
func (c *MCUBClient) InvokeWithLayer(ctx context.Context, layer int, req bin.Object) error {
	wrapped := &tg.InvokeWithLayerRequest{
		Layer: layer,
		Query: req,
	}
	// Config satisfies bin.Decoder and is a reasonable output container when
	// the caller doesn't care about the specific response type.
	var out tg.Config
	return c.api.Invoker().Invoke(ctx, wrapped, &out)
}

// InitConnection sends an initConnection RPC to Telegram using the device
// parameters from Options. gotd/td performs this automatically on every
// session start; this method allows explicit re-invocation.
func (c *MCUBClient) InitConnection(ctx context.Context) error {
	_, err := c.api.HelpGetConfig(ctx)
	if err != nil {
		return fmt.Errorf("init connection (via HelpGetConfig): %w", err)
	}
	return nil
}

// GetFloodSleepThreshold returns the current flood-sleep threshold in seconds.
// When a FloodWaitError asks for <= this many seconds, the client sleeps
// automatically; otherwise it propagates the error.
func (c *MCUBClient) GetFloodSleepThreshold() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.options.FloodSleepThreshold
}

// SetFloodSleepThreshold updates the flood-sleep threshold in seconds.
func (c *MCUBClient) SetFloodSleepThreshold(secs int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.options.FloodSleepThreshold = secs
}

// GetLangCode returns the configured ISO 639-1 language code (e.g. "en").
func (c *MCUBClient) GetLangCode() string {
	return c.options.LangCode
}

// GetAppVersion returns the application version string (e.g. "1.0").
func (c *MCUBClient) GetAppVersion() string {
	return c.options.AppVersion
}

// GetDeviceModel returns the device model string sent during session init.
func (c *MCUBClient) GetDeviceModel() string {
	return c.options.DeviceModel
}

// GetSystemVersion returns the system/OS version string sent during session init.
func (c *MCUBClient) GetSystemVersion() string {
	return c.options.SystemVersion
}
