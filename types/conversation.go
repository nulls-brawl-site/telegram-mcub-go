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

	// edits receives edited messages for this conversation.
	edits chan *tg.Message

	// reads receives max-read-ID notifications for this conversation.
	reads chan int

	// done is closed when the conversation is cancelled.
	done chan struct{}

	mu         sync.Mutex
	cancelled  bool
	sentMsgIDs []int // IDs of messages sent via SendMessage
}

// NewConversation creates a new Conversation for the given chat.
// client should be a *client.MCUBClient (typed as interface{} to avoid cycles).
func NewConversation(client interface{}, chatID int64, timeout time.Duration) *Conversation {
	return &Conversation{
		client:   client,
		chatID:   chatID,
		timeout:  timeout,
		incoming: make(chan *tg.Message, 64),
		edits:    make(chan *tg.Message, 64),
		reads:    make(chan int, 64),
		done:     make(chan struct{}),
	}
}

// Enter sets up the conversation for use. Call this before using the conversation,
// or use it with defer c.Exit() for automatic cleanup.
// If the conversation was previously cancelled, Enter re-initialises it.
func (c *Conversation) Enter() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cancelled {
		c.cancelled = false
		c.done = make(chan struct{})
		c.incoming = make(chan *tg.Message, 64)
		c.edits = make(chan *tg.Message, 64)
		c.reads = make(chan int, 64)
	}
	c.sentMsgIDs = nil
}

// Exit tears down the conversation.  Typically called as defer c.Exit() after Enter.
func (c *Conversation) Exit() {
	c.Cancel()
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
	msg, err := s.SendText(ctx, c.chatID, text)
	if err != nil {
		return nil, err
	}
	// Track sent message ID for reply waiting.
	if msg != nil {
		c.mu.Lock()
		c.sentMsgIDs = append(c.sentMsgIDs, msg.ID)
		c.mu.Unlock()
	}
	return msg, nil
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

// GetEdit waits for an edited message to arrive in this conversation (subject to timeout).
func (c *Conversation) GetEdit(ctx context.Context) (*tg.Message, error) {
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
		case msg, ok := <-c.edits:
			if !ok {
				return nil, fmt.Errorf("conversation edits channel closed")
			}
			return msg, nil
		}
	}
}

// WaitRead waits until the given message ID has been read by the other party.
func (c *Conversation) WaitRead(ctx context.Context, msgID int) error {
	if c.isCancelled() {
		return fmt.Errorf("conversation cancelled")
	}

	deadline := time.Now().Add(c.timeout)
	timer := time.NewTimer(time.Until(deadline))
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-c.done:
			return fmt.Errorf("conversation cancelled")
		case <-timer.C:
			return fmt.Errorf("conversation timed out after %s", c.timeout)
		case maxID, ok := <-c.reads:
			if !ok {
				return fmt.Errorf("conversation reads channel closed")
			}
			if maxID >= msgID {
				return nil
			}
		}
	}
}

// SetTimeout changes the per-operation timeout for this conversation.
func (c *Conversation) SetTimeout(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.timeout = d
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

// DeliverEdit delivers an edited message to the conversation's edits queue.
// This is intended to be called by the event dispatcher.
func (c *Conversation) DeliverEdit(msg *tg.Message) {
	if c.isCancelled() {
		return
	}
	select {
	case c.edits <- msg:
	default:
	}
}

// DeliverRead delivers a max-read-ID notification to the conversation's reads queue.
// This is intended to be called by the event dispatcher when outbox read events arrive.
func (c *Conversation) DeliverRead(maxID int) {
	if c.isCancelled() {
		return
	}
	select {
	case c.reads <- maxID:
	default:
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
