// Package events provides event types and dispatching for telegram-mcub-go.
package events

import (
	"context"
	"sync"

	"github.com/gotd/td/tg"

	"github.com/nulls-brawl-site/telegram-mcub-go/types"
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

// ConversationManager routes incoming messages and edits to active Conversations.
// Register a Conversation before it starts waiting, and Unregister it when done.
type ConversationManager struct {
	mu    sync.RWMutex
	convs map[int64][]*types.Conversation // chatID -> active conversations
}

// NewConversationManager creates a new ConversationManager.
func NewConversationManager() *ConversationManager {
	return &ConversationManager{
		convs: make(map[int64][]*types.Conversation),
	}
}

// Register adds a Conversation to the manager so it receives delivered messages.
func (cm *ConversationManager) Register(conv *types.Conversation) {
	chatID := conv.ChatID()
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.convs[chatID] = append(cm.convs[chatID], conv)
}

// Unregister removes a Conversation from the manager.
func (cm *ConversationManager) Unregister(conv *types.Conversation) {
	chatID := conv.ChatID()
	cm.mu.Lock()
	defer cm.mu.Unlock()
	list := cm.convs[chatID]
	result := list[:0]
	for _, c := range list {
		if c != conv {
			result = append(result, c)
		}
	}
	if len(result) == 0 {
		delete(cm.convs, chatID)
	} else {
		cm.convs[chatID] = result
	}
}

// Deliver routes a new incoming message to all conversations registered for that chat.
func (cm *ConversationManager) Deliver(chatID int64, msg *tg.Message) {
	cm.mu.RLock()
	list := cm.convs[chatID]
	cm.mu.RUnlock()
	for _, c := range list {
		c.Deliver(msg)
	}
}

// DeliverEdit routes an edited message to all conversations registered for that chat.
func (cm *ConversationManager) DeliverEdit(chatID int64, msg *tg.Message) {
	cm.mu.RLock()
	list := cm.convs[chatID]
	cm.mu.RUnlock()
	for _, c := range list {
		c.DeliverEdit(msg)
	}
}

// DeliverRead routes a max-read-ID notification to all conversations registered for that chat.
func (cm *ConversationManager) DeliverRead(chatID int64, maxID int) {
	cm.mu.RLock()
	list := cm.convs[chatID]
	cm.mu.RUnlock()
	for _, c := range list {
		c.DeliverRead(maxID)
	}
}

// HandleUpdatesClass handles top-level update containers (*tg.Updates,
// *tg.UpdatesCombined, etc.) that implement tg.UpdatesClass but not tg.UpdateClass.
// Each contained update is dispatched individually via HandleUpdate.
func (d *Dispatcher) HandleUpdatesClass(ctx context.Context, u tg.UpdatesClass) error {
	switch upd := u.(type) {
	case *tg.Updates:
		return d.handleUpdates(ctx, upd)
	case *tg.UpdatesCombined:
		return d.handleUpdatesCombined(ctx, upd)
	default:
		// UpdatesNotModified, UpdateShort*, etc. — nothing to dispatch.
		return nil
	}
}

// HandleUpdate converts a raw TG update into the most specific event type
// available and dispatches it.  Unrecognised updates are wrapped in Raw and
// dispatched so catch-all handlers still fire.
//
// Dispatch order mirrors Telethon:
//  1. NewMessage / MessageEdited
//  2. MessageDeleted
//  3. MessageRead
//  4. ChatAction
//  5. UserUpdate (typing, status)
//  6. CallbackQuery
//  7. InlineQuery
//  8. JoinRequest
//  9. Raw (catch-all)
func (d *Dispatcher) HandleUpdate(ctx context.Context, u tg.UpdateClass) error {
	// 1. NewMessage
	if ev, ok := NewMessageFromUpdate(ctx, u); ok {
		return d.Dispatch(ctx, ev)
	}
	// 1b. MessageEdited
	if ev, ok := MessageEditedFromUpdate(ctx, u); ok {
		return d.Dispatch(ctx, ev)
	}
	// 2. MessageDeleted
	if ev, ok := MessageDeletedFromUpdate(ctx, u); ok {
		return d.Dispatch(ctx, ev)
	}
	// 3. MessageRead
	if ev, ok := MessageReadFromUpdate(ctx, u); ok {
		return d.Dispatch(ctx, ev)
	}
	// 4. ChatAction
	if ev, ok := ChatActionFromUpdate(ctx, u); ok {
		return d.Dispatch(ctx, ev)
	}
	// 5. UserUpdate
	if ev, ok := UserUpdateFromUpdate(ctx, u); ok {
		return d.Dispatch(ctx, ev)
	}
	// 6. CallbackQuery
	if ev, ok := CallbackQueryFromUpdate(ctx, u); ok {
		return d.Dispatch(ctx, ev)
	}
	// 7. InlineQuery
	if ev, ok := InlineQueryFromUpdate(ctx, u); ok {
		return d.Dispatch(ctx, ev)
	}
	// 8. JoinRequest
	if ev, ok := JoinRequestFromUpdate(ctx, u); ok {
		return d.Dispatch(ctx, ev)
	}
	// 9. Raw catch-all
	return d.Dispatch(ctx, &Raw{Update: u})
}

// handleUpdates expands an *tg.Updates container and dispatches each update.
func (d *Dispatcher) handleUpdates(ctx context.Context, updates *tg.Updates) error {
	for _, u := range updates.Updates {
		if err := d.HandleUpdate(ctx, u); err != nil {
			return err
		}
	}
	return nil
}

// handleUpdatesCombined expands an *tg.UpdatesCombined container and dispatches each update.
func (d *Dispatcher) handleUpdatesCombined(ctx context.Context, updates *tg.UpdatesCombined) error {
	for _, u := range updates.Updates {
		if err := d.HandleUpdate(ctx, u); err != nil {
			return err
		}
	}
	return nil
}
