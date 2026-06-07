package client

import (
	"context"
	"fmt"
	"time"

	"github.com/gotd/td/bin"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/tg"
	"github.com/nulls-brawl-site/telegram-mcub-go/types"
)

// protectionMiddleware returns a telegram.MiddlewareFunc that enforces the given
// ProtectionPolicy (flood-wait handling, retry logic, etc.).
func protectionMiddleware(policy types.ProtectionPolicy) telegram.MiddlewareFunc {
	return func(next tg.Invoker) telegram.InvokeFunc {
		return func(ctx context.Context, input bin.Encoder, output bin.Decoder) error {
			attempts := policy.RetryCount + 1
			if attempts < 1 {
				attempts = 1
			}

			var lastErr error
			for i := 0; i < attempts; i++ {
				lastErr = next.Invoke(ctx, input, output)
				if lastErr == nil {
					return nil
				}

				// Handle FLOOD_WAIT.
				if secs, ok := parseFloodWait(lastErr); ok {
					if policy.MaxFloodWaitSeconds > 0 && secs <= policy.MaxFloodWaitSeconds {
						select {
						case <-ctx.Done():
							return ctx.Err()
						case <-time.After(time.Duration(secs) * time.Second):
						}
						continue // retry
					}
					// Flood wait exceeds policy limit — return the error immediately.
					return lastErr
				}

				// Non-retriable error — break out of retry loop.
				break
			}
			return lastErr
		}
	}
}

// parseFloodWait tries to extract a FLOOD_WAIT wait time from an error string.
func parseFloodWait(err error) (int, bool) {
	if err == nil {
		return 0, false
	}
	var secs int
	_, scanErr := fmt.Sscanf(err.Error(), "FLOOD_WAIT_%d", &secs)
	if scanErr == nil && secs > 0 {
		return secs, true
	}
	return 0, false
}
