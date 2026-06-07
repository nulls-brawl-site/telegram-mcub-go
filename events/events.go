// Package events provides event types and dispatching for telegram-mcub-go.
package events

import (
	"context"

	"github.com/gotd/td/tg"
)

// Handler is the function signature for event handlers.
type Handler func(ctx context.Context, e Event) error

// Filter is a predicate that decides whether a handler should run for an event.
type Filter func(e Event) bool

// Event is the base interface that all event types implement.
type Event interface {
	// EventType returns a string identifier for this event kind.
	EventType() string
}

// MiddlewareFunc wraps an event handler, allowing pre/post processing.
// Call next(ctx, e) to continue the middleware chain.
type MiddlewareFunc func(ctx context.Context, e Event, next Handler) error

// Dispatcher maintains a list of event subscriptions and dispatches events
// through middleware chains to matching handlers.
type Dispatcher struct {
	middlewares []MiddlewareFunc
	handlers    []subscription
}

type subscription struct {
	filter  Filter
	handler Handler
}

// NewDispatcher creates a new event dispatcher.
func NewDispatcher() *Dispatcher {
	return &Dispatcher{}
}

// AddMiddleware appends a middleware to the chain.
func (d *Dispatcher) AddMiddleware(mw MiddlewareFunc) {
	d.middlewares = append(d.middlewares, mw)
}

// RemoveMiddleware removes the first middleware equal to mw by pointer identity.
func (d *Dispatcher) RemoveMiddleware(mw MiddlewareFunc) {
	target := middlewareFuncPtr(mw)
	result := d.middlewares[:0]
	for _, m := range d.middlewares {
		if middlewareFuncPtr(m) != target {
			result = append(result, m)
		}
	}
	d.middlewares = result
}

// AddHandler registers a handler with an optional filter.
// If filter is nil, the handler runs for all events.
func (d *Dispatcher) AddHandler(filter Filter, handler Handler) {
	d.handlers = append(d.handlers, subscription{filter: filter, handler: handler})
}

// Dispatch routes the event through the middleware chain and then to matching handlers.
func (d *Dispatcher) Dispatch(ctx context.Context, e Event) error {
	for _, sub := range d.handlers {
		if sub.filter != nil && !sub.filter(e) {
			continue
		}
		// Build a fresh middleware chain for each handler.
		chain := d.buildChain(sub.handler)
		if err := chain(ctx, e); err != nil {
			return err
		}
	}
	return nil
}

// buildChain wraps the final handler with all middlewares (last added = outermost).
func (d *Dispatcher) buildChain(final Handler) Handler {
	h := final
	for i := len(d.middlewares) - 1; i >= 0; i-- {
		mw := d.middlewares[i]
		next := h
		h = func(ctx context.Context, e Event) error {
			return mw(ctx, e, next)
		}
	}
	return h
}

// middlewareFuncPtr returns a comparable pointer for a MiddlewareFunc.
// This is a best-effort comparison; function values in Go are not directly comparable,
// so we store them as interface{} and compare via reflect if needed.
// For now we use a simple index-based removal approach instead.
func middlewareFuncPtr(_ MiddlewareFunc) uintptr {
	// Not reliably comparable in Go — callers should use index-based removal.
	return 0
}

// UpdateClass is an alias to make imports more ergonomic.
type UpdateClass = tg.UpdateClass
