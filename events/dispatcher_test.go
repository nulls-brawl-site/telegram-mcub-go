package events_test

import (
	"context"
	"errors"
	"testing"

	"github.com/gotd/td/tg"

	"github.com/nulls-brawl-site/telegram-mcub-go/events"
)

// ---------------------------------------------------------------------------
// Basic dispatch
// ---------------------------------------------------------------------------

func TestDispatchNewMessage(t *testing.T) {
	d := events.NewDispatcher()
	called := false
	d.AddHandler(nil, func(ctx context.Context, e events.Event) error {
		called = true
		return nil
	})
	err := d.Dispatch(context.Background(), &events.NewMessage{
		Raw: &tg.Message{Message: "test"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("handler was not called")
	}
}

func TestDispatchNoHandlers(t *testing.T) {
	d := events.NewDispatcher()
	err := d.Dispatch(context.Background(), &events.NewMessage{
		Raw: &tg.Message{Message: "test"},
	})
	if err != nil {
		t.Fatalf("expected nil error with no handlers, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Filtered dispatch
// ---------------------------------------------------------------------------

func TestDispatchFiltered(t *testing.T) {
	d := events.NewDispatcher()
	called := false
	d.AddHandler(
		events.PatternFilter(func(t string) bool { return t == "match" }),
		func(ctx context.Context, e events.Event) error {
			called = true
			return nil
		},
	)

	// Should not fire for non-matching text.
	_ = d.Dispatch(context.Background(), &events.NewMessage{
		Raw: &tg.Message{Message: "no match"},
	})
	if called {
		t.Fatal("handler should not have been called for 'no match'")
	}

	// Should fire for matching text.
	_ = d.Dispatch(context.Background(), &events.NewMessage{
		Raw: &tg.Message{Message: "match"},
	})
	if !called {
		t.Fatal("handler should have been called for 'match'")
	}
}

func TestDispatchMultipleHandlers(t *testing.T) {
	d := events.NewDispatcher()
	count := 0
	for i := 0; i < 3; i++ {
		d.AddHandler(nil, func(ctx context.Context, e events.Event) error {
			count++
			return nil
		})
	}
	_ = d.Dispatch(context.Background(), &events.NewMessage{
		Raw: &tg.Message{Message: "hi"},
	})
	if count != 3 {
		t.Fatalf("expected 3 handler calls, got %d", count)
	}
}

func TestDispatchFilteredCallbackQuery(t *testing.T) {
	d := events.NewDispatcher()
	called := false
	d.AddHandler(
		events.CallbackQueryFilter(),
		func(ctx context.Context, e events.Event) error {
			called = true
			return nil
		},
	)

	// NewMessage should not match.
	_ = d.Dispatch(context.Background(), &events.NewMessage{Raw: &tg.Message{}})
	if called {
		t.Fatal("CallbackQueryFilter should not fire for NewMessage")
	}

	// CallbackQuery should match.
	_ = d.Dispatch(context.Background(), &events.CallbackQuery{
		Raw: &tg.UpdateBotCallbackQuery{},
	})
	if !called {
		t.Fatal("CallbackQueryFilter should fire for CallbackQuery")
	}
}

// ---------------------------------------------------------------------------
// Error propagation
// ---------------------------------------------------------------------------

func TestDispatchHandlerError(t *testing.T) {
	d := events.NewDispatcher()
	sentinel := errors.New("handler error")
	d.AddHandler(nil, func(ctx context.Context, e events.Event) error {
		return sentinel
	})
	err := d.Dispatch(context.Background(), &events.NewMessage{
		Raw: &tg.Message{Message: "test"},
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Middleware
// ---------------------------------------------------------------------------

func TestMiddleware(t *testing.T) {
	d := events.NewDispatcher()
	order := []string{}

	d.AddMiddleware(func(ctx context.Context, e events.Event, next events.Handler) error {
		order = append(order, "mw1-before")
		err := next(ctx, e)
		order = append(order, "mw1-after")
		return err
	})
	d.AddMiddleware(func(ctx context.Context, e events.Event, next events.Handler) error {
		order = append(order, "mw2-before")
		err := next(ctx, e)
		order = append(order, "mw2-after")
		return err
	})
	d.AddHandler(nil, func(ctx context.Context, e events.Event) error {
		order = append(order, "handler")
		return nil
	})

	_ = d.Dispatch(context.Background(), &events.NewMessage{Raw: &tg.Message{}})

	// Middleware wraps from last-to-first, so mw1 is outermost:
	// mw1-before → mw2-before → handler → mw2-after → mw1-after
	expected := []string{"mw1-before", "mw2-before", "handler", "mw2-after", "mw1-after"}
	if len(order) != len(expected) {
		t.Fatalf("execution order length = %d, want %d; got %v", len(order), len(expected), order)
	}
	for i, v := range expected {
		if order[i] != v {
			t.Fatalf("execution order[%d] = %q, want %q; full order: %v", i, order[i], v, order)
		}
	}
}

func TestMiddlewareShortCircuit(t *testing.T) {
	d := events.NewDispatcher()
	handlerCalled := false

	d.AddMiddleware(func(ctx context.Context, e events.Event, next events.Handler) error {
		// Skip calling next — short-circuits the chain.
		return nil
	})
	d.AddHandler(nil, func(ctx context.Context, e events.Event) error {
		handlerCalled = true
		return nil
	})

	_ = d.Dispatch(context.Background(), &events.NewMessage{Raw: &tg.Message{}})
	if handlerCalled {
		t.Fatal("handler should not have been called when middleware short-circuits")
	}
}

// ---------------------------------------------------------------------------
// HandleUpdate integration
// ---------------------------------------------------------------------------

func TestHandleUpdateNewMessage(t *testing.T) {
	d := events.NewDispatcher()
	var received events.Event
	d.AddHandler(nil, func(ctx context.Context, e events.Event) error {
		received = e
		return nil
	})

	u := &tg.UpdateNewMessage{
		Message: &tg.Message{Message: "hello"},
		Pts:     1,
		PtsCount: 1,
	}
	err := d.HandleUpdate(context.Background(), u)
	if err != nil {
		t.Fatalf("HandleUpdate error: %v", err)
	}
	if received == nil {
		t.Fatal("no event received")
	}
	nm, ok := received.(*events.NewMessage)
	if !ok {
		t.Fatalf("expected *events.NewMessage, got %T", received)
	}
	if nm.Text() != "hello" {
		t.Fatalf("Text() = %q, want 'hello'", nm.Text())
	}
}

func TestHandleUpdateCallbackQuery(t *testing.T) {
	d := events.NewDispatcher()
	var received events.Event
	d.AddHandler(events.CallbackQueryFilter(), func(ctx context.Context, e events.Event) error {
		received = e
		return nil
	})

	u := &tg.UpdateBotCallbackQuery{
		UserID: 1,
		MsgID:  10,
		Peer:   &tg.PeerUser{UserID: 1},
		Data:   []byte("btn:1"),
	}
	err := d.HandleUpdate(context.Background(), u)
	if err != nil {
		t.Fatalf("HandleUpdate error: %v", err)
	}
	if received == nil {
		t.Fatal("no CallbackQuery event received")
	}
}

func TestHandleUpdateUnknown(t *testing.T) {
	d := events.NewDispatcher()
	var received events.Event
	d.AddHandler(nil, func(ctx context.Context, e events.Event) error {
		received = e
		return nil
	})

	// An update type not handled by any specific builder → Raw event.
	u := &tg.UpdateDraftMessage{}
	err := d.HandleUpdate(context.Background(), u)
	if err != nil {
		t.Fatalf("HandleUpdate error: %v", err)
	}
	raw, ok := received.(*events.Raw)
	if !ok {
		t.Fatalf("expected *events.Raw, got %T", received)
	}
	if raw.TypeName() == "" {
		t.Fatal("Raw.TypeName() should not be empty")
	}
}

func TestHandleUpdateBotInlineSend(t *testing.T) {
	d := events.NewDispatcher()
	var received events.Event
	d.AddHandler(events.BotInlineSendFilter(), func(ctx context.Context, e events.Event) error {
		received = e
		return nil
	})

	u := &tg.UpdateBotInlineSend{
		UserID: 42,
		Query:  "hello",
		ID:     "res-1",
	}
	err := d.HandleUpdate(context.Background(), u)
	if err != nil {
		t.Fatalf("HandleUpdate error: %v", err)
	}
	if received == nil {
		t.Fatal("no BotInlineSend event received")
	}
	bs, ok := received.(*events.BotInlineSend)
	if !ok {
		t.Fatalf("expected *events.BotInlineSend, got %T", received)
	}
	if bs.UserID != 42 {
		t.Fatalf("UserID = %d, want 42", bs.UserID)
	}
}
