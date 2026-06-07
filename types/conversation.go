package types

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gotd/td/tg"
)

// Conversation represents an ongoing conversation context in a chat.
// It allows waiting for specific messages or events within a timeout,
// mirroring Telethon's tl/custom/conversation.Conversation class.
type Conversation struct {
	// client is the owning MCUBClient; typed as interface{} to avoid an
	// import cycle with the client package.
	client interface{}

	// chatID is the signed peer ID of the conversation chat.
	chatID int64

	// timeout is the per-operation timeout.
	timeout time.Duration

	// incoming receives new inbound messages for this conversation.
	incoming chan *tg.Message

	// done is closed when the conversation is cancelled.
	done chan struct{}

	mu        sync.Mutex
	cancelled bool
}

// NewConversation creates a new Conversation for the given chat.
// client should be a *client.MCUBClient (typed as interface{} to avoid cycles).
func NewConversation(client interface{}, chatID int64, timeout time.Duration) *Conversation {
	return &Conversation{
		client:   client,
		chatID:   chatID,
		timeout:  timeout,
		incoming: make(chan *tg.Message, 64),
		done:     make(chan struct{}),
	}
}

// ChatID returns the chat ID this conversation is tied to.
func (c *Conversation) ChatID() int64 {
	return c.chatID
}

// SendMessage sends a message in the conversation chat.
// The client field must implement a SendText(ctx, chatID, text) method or
// callers should use the client directly.
func (c *Conversation) SendMessage(ctx context.Context, text string) (*tg.Message, error) {
	if c.isCancelled() {
		return nil, fmt.Errorf("conversation cancelled")
	}
	// Delegate to the client via interface assertion.
	type sender interface {
		SendText(ctx context.Context, chatID int64, text string) (*tg.Message, error)
	}
	s, ok := c.client.(sender)
	if !ok {
		return nil, fmt.Errorf("client does not implement SendText")
	}
	return s.SendText(ctx, c.chatID, text)
}

// GetResponse waits for the next incoming message (subject to timeout).
func (c *Conversation) GetResponse(ctx context.Context) (*tg.Message, error) {
	return c.waitMessage(ctx, func(*tg.Message) bool { return true })
}

// GetReply waits for a reply to a specific message ID.
func (c *Conversation) GetReply(ctx context.Context, msgID int) (*tg.Message, error) {
	return c.waitMessage(ctx, func(m *tg.Message) bool {
		if m.ReplyTo == nil {
			return false
		}
		if rt, ok := m.ReplyTo.(*tg.MessageReplyHeader); ok {
			return rt.ReplyToMsgID == msgID
		}
		return false
	})
}

// WaitEvent waits for any incoming message matching predicate.
func (c *Conversation) WaitEvent(ctx context.Context, predicate func(*tg.Message) bool) (*tg.Message, error) {
	return c.waitMessage(ctx, predicate)
}

// waitMessage blocks until a message satisfying pred arrives, the conversation
// timeout fires, or ctx is cancelled.
func (c *Conversation) waitMessage(ctx context.Context, pred func(*tg.Message) bool) (*tg.Message, error) {
	if c.isCancelled() {
		return nil, fmt.Errorf("conversation cancelled")
	}

	deadline := time.Now().Add(c.timeout)
	timer := time.NewTimer(time.Until(deadline))
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-c.done:
			return nil, fmt.Errorf("conversation cancelled")
		case <-timer.C:
			return nil, fmt.Errorf("conversation timed out after %s", c.timeout)
		case msg, ok := <-c.incoming:
			if !ok {
				return nil, fmt.Errorf("conversation incoming channel closed")
			}
			if pred(msg) {
				return msg, nil
			}
		}
	}
}

// MarkRead marks all messages in the conversation as read by delegating to the client.
func (c *Conversation) MarkRead(ctx context.Context) error {
	if c.isCancelled() {
		return fmt.Errorf("conversation cancelled")
	}
	type reader interface {
		MarkRead(ctx context.Context, chatID int64) error
	}
	r, ok := c.client.(reader)
	if !ok {
		return fmt.Errorf("client does not implement MarkRead")
	}
	return r.MarkRead(ctx, c.chatID)
}

// Deliver delivers an incoming message to the conversation's internal queue.
// This is intended to be called by the event dispatcher.
func (c *Conversation) Deliver(msg *tg.Message) {
	if c.isCancelled() {
		return
	}
	select {
	case c.incoming <- msg:
	default:
		// Drop if the buffer is full; callers should not flood a single conversation.
	}
}

// Cancel cancels the conversation context.  All pending waits will return an error.
func (c *Conversation) Cancel() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.cancelled {
		c.cancelled = true
		close(c.done)
	}
}

// isCancelled reports whether the conversation has been cancelled.
func (c *Conversation) isCancelled() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cancelled
}
