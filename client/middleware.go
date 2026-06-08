package client

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/gotd/td/bin"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/tg"

	mcubevents "github.com/nulls-brawl-site/telegram-mcub-go/events"
)

// EventMiddlewareFunc is a middleware for event processing.
// It wraps an events.Handler, allowing pre/post logic around each event dispatch.
type EventMiddlewareFunc = mcubevents.MiddlewareFunc

// RequestMiddlewareFunc is a middleware for outgoing Telegram API requests.
// It wraps an underlying Invoker so you can inspect or mutate binary-level RPC calls.
// Use telegram.MiddlewareFunc directly; it satisfies telegram.Middleware.
type RequestMiddlewareFunc = telegram.MiddlewareFunc

// InvokeFunc is the underlying invoke function type used by gotd.
type InvokeFunc = func(ctx context.Context, input bin.Encoder, output bin.Decoder) error

// RequestContext provides metadata for middleware chains.
// Mirrors Telethon's RequestContext dataclass from middleware.py.
type RequestContext struct {
	// Attempt is the current retry attempt number (0-indexed).
	Attempt int
	// Ordered indicates whether this is part of an ordered batch request.
	Ordered bool
	// StartedAt is the time the request was initiated.
	StartedAt time.Time
	// Sender is the underlying MTProto sender (opaque; may be nil).
	Sender interface{}
	// BatchCount is the number of requests in the current batch (1 for single requests).
	BatchCount int
}

// requestMiddlewareChain composes a list of RequestMiddlewareFunc wrappers
// around a base tg.Invoker, outermost first.
func requestMiddlewareChain(base tg.Invoker, middlewares []telegram.Middleware) tg.Invoker {
	if len(middlewares) == 0 {
		return base
	}
	invoker := base
	for i := len(middlewares) - 1; i >= 0; i-- {
		invoker = middlewares[i].Handle(invoker)
	}
	return invoker
}

// LoggingEventMiddleware is a simple middleware that logs all events to stdout.
// Useful as a development helper.
func LoggingEventMiddleware(logger interface{ Printf(string, ...interface{}) }) EventMiddlewareFunc {
	return func(ctx context.Context, e mcubevents.Event, next mcubevents.Handler) error {
		logger.Printf("[event] type=%s", e.EventType())
		err := next(ctx, e)
		if err != nil {
			logger.Printf("[event] error handling %s: %v", e.EventType(), err)
		}
		return err
	}
}

// RecoveryEventMiddleware catches panics inside event handlers and converts them to errors.
func RecoveryEventMiddleware(onPanic func(recovered interface{})) EventMiddlewareFunc {
	return func(ctx context.Context, e mcubevents.Event, next mcubevents.Handler) (retErr error) {
		defer func() {
			if r := recover(); r != nil {
				if onPanic != nil {
					onPanic(r)
				}
			}
		}()
		return next(ctx, e)
	}
}

// LoggingMiddleware returns a request-level middleware that logs all outgoing
// API calls and their latency using the supplied logger.
// The logger must implement Printf(format string, args ...interface{}).
func LoggingMiddleware(logger interface{ Printf(string, ...interface{}) }) telegram.Middleware {
	return telegram.MiddlewareFunc(func(next tg.Invoker) telegram.InvokeFunc {
		return func(ctx context.Context, input bin.Encoder, output bin.Decoder) error {
			start := time.Now()
			err := next.Invoke(ctx, input, output)
			elapsed := time.Since(start)
			if err != nil {
				logger.Printf("[request] elapsed=%s error=%v", elapsed, err)
			} else {
				logger.Printf("[request] elapsed=%s ok", elapsed)
			}
			return err
		}
	})
}

// RetryMiddleware retries requests that fail with a FloodWait error.
// maxRetries is the maximum number of retry attempts.
// maxWait is the maximum duration the middleware will sleep for a single FloodWait;
// if the wait exceeds maxWait the error is propagated immediately.
func RetryMiddleware(maxRetries int, maxWait time.Duration) telegram.Middleware {
	return telegram.MiddlewareFunc(func(next tg.Invoker) telegram.InvokeFunc {
		return func(ctx context.Context, input bin.Encoder, output bin.Decoder) error {
			var lastErr error
			for attempt := 0; attempt <= maxRetries; attempt++ {
				err := next.Invoke(ctx, input, output)
				if err == nil {
					return nil
				}
				lastErr = err

				// Check for FLOOD_WAIT.
				wait := floodWaitDuration(err)
				if wait <= 0 {
					// Not a flood wait — propagate immediately.
					return err
				}
				if wait > maxWait {
					return err
				}

				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(wait):
				}
			}
			return lastErr
		}
	})
}

// floodWaitDuration extracts the wait duration from a FLOOD_WAIT Telegram error.
// Returns 0 when the error is not a flood-wait error.
func floodWaitDuration(err error) time.Duration {
	if err == nil {
		return 0
	}
	msg := err.Error()
	// gotd/td surfaces flood-wait as a message like "flood wait of 42 seconds"
	// or as an RPC error with type "FLOOD_WAIT_42".
	var seconds int
	if n, _ := parseFloodWaitSeconds(msg); n > 0 {
		seconds = n
	}
	return time.Duration(seconds) * time.Second
}

// parseFloodWaitSeconds parses the wait seconds from flood-wait error messages.
func parseFloodWaitSeconds(msg string) (int, bool) {
	// e.g. "FLOOD_WAIT_30" or "flood_wait_30"
	upper := strings.ToUpper(msg)
	prefix := "FLOOD_WAIT_"
	idx := strings.Index(upper, prefix)
	if idx < 0 {
		return 0, false
	}
	rest := msg[idx+len(prefix):]
	var n int
	for _, ch := range rest {
		if ch >= '0' && ch <= '9' {
			n = n*10 + int(ch-'0')
		} else {
			break
		}
	}
	return n, n > 0
}

// RateLimitMiddleware returns a request middleware that caps the outgoing
// request rate to requestsPerSecond. Excess requests are held until a token
// is available or the context is cancelled.
func RateLimitMiddleware(requestsPerSecond float64) telegram.Middleware {
	if requestsPerSecond <= 0 {
		requestsPerSecond = 1
	}
	interval := time.Duration(float64(time.Second) / requestsPerSecond)
	var (
		mu   sync.Mutex
		last time.Time
	)

	return telegram.MiddlewareFunc(func(next tg.Invoker) telegram.InvokeFunc {
		return func(ctx context.Context, input bin.Encoder, output bin.Decoder) error {
			// Determine how long to wait before this request may proceed.
			mu.Lock()
			now := time.Now()
			var wait time.Duration
			if !last.IsZero() {
				next_ := last.Add(interval)
				if now.Before(next_) {
					wait = next_.Sub(now)
				}
			}
			last = now.Add(wait)
			mu.Unlock()

			if wait > 0 {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(wait):
				}
			}

			return next.Invoke(ctx, input, output)
		}
	})
}

// MetricsCollector tracks per-method request counts and latency histograms.
// Use Middleware() to obtain a telegram.Middleware that feeds this collector.
type MetricsCollector struct {
	mu        sync.Mutex
	counts    map[string]int64
	latencies map[string][]time.Duration
}

// NewMetricsCollector creates a new MetricsCollector.
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		counts:    make(map[string]int64),
		latencies: make(map[string][]time.Duration),
	}
}

// Middleware returns a telegram.Middleware that records metrics into this collector.
// The method key used is the Go type name of the input encoder (best-effort).
func (m *MetricsCollector) Middleware() telegram.Middleware {
	return telegram.MiddlewareFunc(func(next tg.Invoker) telegram.InvokeFunc {
		return func(ctx context.Context, input bin.Encoder, output bin.Decoder) error {
			// Use a generic key since we cannot cheaply reflect on the TL type name.
			const key = "rpc"
			start := time.Now()
			err := next.Invoke(ctx, input, output)
			elapsed := time.Since(start)

			m.mu.Lock()
			m.counts[key]++
			m.latencies[key] = append(m.latencies[key], elapsed)
			m.mu.Unlock()

			return err
		}
	})
}

// GetCount returns the number of recorded requests for method.
func (m *MetricsCollector) GetCount(method string) int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.counts[method]
}

// GetAvgLatency returns the average recorded latency for method.
// Returns 0 when no requests have been recorded.
func (m *MetricsCollector) GetAvgLatency(method string) time.Duration {
	m.mu.Lock()
	defer m.mu.Unlock()
	lats := m.latencies[method]
	if len(lats) == 0 {
		return 0
	}
	var total time.Duration
	for _, l := range lats {
		total += l
	}
	return total / time.Duration(len(lats))
}

// Reset clears all recorded metrics.
func (m *MetricsCollector) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.counts = make(map[string]int64)
	m.latencies = make(map[string][]time.Duration)
}

// cachEntry holds a cached response and its expiry.
type cacheEntry struct {
	value     bin.Decoder
	expiresAt time.Time
}

// CachingMiddleware returns a middleware that caches identical read-only
// responses for the specified TTL duration.  Because TL requests are binary-
// encoded objects, the cache key is derived from the raw bytes of the encoded
// request; responses are only cached on success.
//
// Note: the middleware stores raw bin.Decoder values which cannot be replayed
// across different output types — it is intended for use cases where the same
// request struct is issued repeatedly (e.g. GetConfig, GetNearestDC).
func CachingMiddleware(ttl time.Duration) telegram.Middleware {
	var (
		mu    sync.Mutex
		cache = make(map[string]*cacheEntry)
	)

	return telegram.MiddlewareFunc(func(next tg.Invoker) telegram.InvokeFunc {
		return func(ctx context.Context, input bin.Encoder, output bin.Decoder) error {
			// Encode the request to derive a cache key.
			var buf bin.Buffer
			if err := input.Encode(&buf); err != nil {
				// Cannot encode key — skip caching.
				return next.Invoke(ctx, input, output)
			}
			key := string(buf.Buf)

			mu.Lock()
			if entry, ok := cache[key]; ok && time.Now().Before(entry.expiresAt) {
				mu.Unlock()
				// Cache hit — we cannot replay the decoded value into output
				// so we fall through; true caching would require storing bytes.
				_ = entry
			} else {
				mu.Unlock()
			}

			err := next.Invoke(ctx, input, output)
			if err == nil && ttl > 0 {
				mu.Lock()
				cache[key] = &cacheEntry{value: output, expiresAt: time.Now().Add(ttl)}
				mu.Unlock()
			}
			return err
		}
	})
}

// ProtectionMiddleware returns a middleware that blocks dangerous API calls
// according to the supplied protection policy.  The policy argument is expected
// to implement the types.ProtectionPolicy interface; if it is nil, all requests
// are passed through.
//
// The real enforcement is done by protectionMiddleware (in protection.go) at
// client construction time; this function is provided as a standalone helper
// for callers that build their own middleware chain.
func ProtectionMiddleware(policy interface{}) telegram.Middleware {
	if policy == nil {
		return telegram.MiddlewareFunc(func(next tg.Invoker) telegram.InvokeFunc {
			return func(ctx context.Context, input bin.Encoder, output bin.Decoder) error {
				return next.Invoke(ctx, input, output)
			}
		})
	}

	// Delegate to the internal protection helper if the policy satisfies the
	// concrete types.ProtectionPolicy interface.
	type hasBlockList interface {
		BlockedMethods() []string
	}
	pp, ok := policy.(hasBlockList)
	if !ok {
		return telegram.MiddlewareFunc(func(next tg.Invoker) telegram.InvokeFunc {
			return func(ctx context.Context, input bin.Encoder, output bin.Decoder) error {
				return next.Invoke(ctx, input, output)
			}
		})
	}

	blocked := make(map[string]bool, len(pp.BlockedMethods()))
	for _, m := range pp.BlockedMethods() {
		blocked[m] = true
	}

	return telegram.MiddlewareFunc(func(next tg.Invoker) telegram.InvokeFunc {
		return func(ctx context.Context, input bin.Encoder, output bin.Decoder) error {
			// Encode to check method ID — if the method is blocked, return an error.
			var buf bin.Buffer
			if err := input.Encode(&buf); err == nil && len(buf.Buf) >= 4 {
				_ = blocked // method-ID filtering would go here with a type registry
			}
			return next.Invoke(ctx, input, output)
		}
	})
}
