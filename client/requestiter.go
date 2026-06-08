package client

import (
	"context"
	"fmt"
)

// RequestIter is a generic paginated request iterator.
// It mirrors Telethon's RequestIter base class, providing buffered paging,
// a limit cap, and helpers for channel-based consumption.
type RequestIter[T any] struct {
	client *MCUBClient

	// limit is the total maximum items to yield (0 = unlimited).
	limit int

	// left tracks how many items are still allowed to be yielded.
	left int

	// total is the server-reported count, populated after the first fetch.
	total int

	// buffer holds the items fetched in the current page.
	buffer []T

	// bufPos is the current position within buffer.
	bufPos int

	// done signals the iterator has been exhausted.
	done bool

	// initialized tracks whether initFn has been called.
	initialized bool

	// initFn is called once before the first fetch; implemented by sub-types.
	// It may pre-populate buffer for the first page.
	initFn func(ctx context.Context) error

	// loadNextFn fetches the next page into buffer and updates total.
	// It must set buffer to nil / empty slice to signal exhaustion.
	loadNextFn func(ctx context.Context) error
}

// NewRequestIter creates a new RequestIter with the given limit.
// The caller must set initFn and loadNextFn before calling Next.
func NewRequestIter[T any](client *MCUBClient, limit int) *RequestIter[T] {
	left := limit
	if limit <= 0 {
		left = -1 // unlimited
	}
	return &RequestIter[T]{
		client: client,
		limit:  limit,
		left:   left,
	}
}

// Total returns the server-reported total count.
// This value is only meaningful after the first call to Next.
func (it *RequestIter[T]) Total() int {
	return it.total
}

// Next advances to the next item.
// Returns (item, true, nil) while items remain,
// (zero, false, nil) when exhausted,
// (zero, false, err) on error.
func (it *RequestIter[T]) Next(ctx context.Context) (T, bool, error) {
	var zero T

	if it.done {
		return zero, false, nil
	}

	// Initialize on first call.
	if !it.initialized {
		it.initialized = true
		if it.initFn != nil {
			if err := it.initFn(ctx); err != nil {
				return zero, false, fmt.Errorf("requestiter init: %w", err)
			}
		}
	}

	// Serve from buffer.
	for {
		if it.bufPos < len(it.buffer) {
			item := it.buffer[it.bufPos]
			it.bufPos++
			if it.left > 0 {
				it.left--
				if it.left == 0 {
					it.done = true
				}
			}
			return item, true, nil
		}

		// Buffer exhausted — check hard limit.
		if it.left == 0 {
			it.done = true
			return zero, false, nil
		}

		// Fetch next page.
		if it.loadNextFn == nil {
			it.done = true
			return zero, false, nil
		}

		it.buffer = it.buffer[:0]
		it.bufPos = 0
		if err := it.loadNextFn(ctx); err != nil {
			return zero, false, fmt.Errorf("requestiter load: %w", err)
		}

		if len(it.buffer) == 0 {
			it.done = true
			return zero, false, nil
		}
	}
}

// Collect collects all remaining items into a slice.
// Respects the limit set at construction time.
func (it *RequestIter[T]) Collect(ctx context.Context) ([]T, error) {
	var out []T
	for {
		item, ok, err := it.Next(ctx)
		if err != nil {
			return out, err
		}
		if !ok {
			break
		}
		out = append(out, item)
	}
	return out, nil
}

// Channel returns two channels: one that yields items and one for errors.
// The item channel is closed when the iterator is exhausted or an error occurs.
// The caller must drain both channels to avoid goroutine leaks.
func (it *RequestIter[T]) Channel(ctx context.Context) (<-chan T, <-chan error) {
	ch := make(chan T)
	errCh := make(chan error, 1)

	go func() {
		defer close(ch)
		defer close(errCh)

		for {
			item, ok, err := it.Next(ctx)
			if err != nil {
				errCh <- err
				return
			}
			if !ok {
				return
			}
			select {
			case <-ctx.Done():
				errCh <- ctx.Err()
				return
			case ch <- item:
			}
		}
	}()

	return ch, errCh
}
