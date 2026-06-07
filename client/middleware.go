package client

import (
	"context"

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
