// Package types provides shared types for telegram-mcub-go.
package types

// ProtectionMode defines the security profile for the client.
type ProtectionMode int

const (
	// ProtectionOff disables all protection measures.
	ProtectionOff ProtectionMode = iota
	// ProtectionSafe enables basic safety checks (default).
	ProtectionSafe
	// ProtectionStrict enables strict security checks (rate-limiting, flood guards, etc.).
	ProtectionStrict
	// ProtectionCustom enables user-defined protection policy.
	ProtectionCustom
)

func (m ProtectionMode) String() string {
	switch m {
	case ProtectionOff:
		return "off"
	case ProtectionSafe:
		return "safe"
	case ProtectionStrict:
		return "strict"
	case ProtectionCustom:
		return "custom"
	default:
		return "unknown"
	}
}

// ProtectionPolicy holds configuration for the custom protection mode.
type ProtectionPolicy struct {
	// MaxFloodWaitSeconds is the maximum number of seconds to wait on FLOOD_WAIT errors.
	// A value of 0 means never wait (return error immediately).
	MaxFloodWaitSeconds int

	// RetryCount is the number of times to retry a request on server errors.
	RetryCount int

	// AutoReconnect controls whether the client reconnects automatically on disconnect.
	AutoReconnect bool

	// IgnoreMediaErrors controls whether media-related errors are silently ignored.
	IgnoreMediaErrors bool

	// MaxConcurrentRequests limits parallel requests to the Telegram API.
	// 0 means unlimited.
	MaxConcurrentRequests int

	// RateLimitPerSecond limits the number of API calls per second.
	// 0 means unlimited.
	RateLimitPerSecond int
}

// DefaultSafePolicy returns the ProtectionPolicy for ProtectionSafe mode.
func DefaultSafePolicy() ProtectionPolicy {
	return ProtectionPolicy{
		MaxFloodWaitSeconds:   60,
		RetryCount:            3,
		AutoReconnect:         true,
		IgnoreMediaErrors:     false,
		MaxConcurrentRequests: 10,
		RateLimitPerSecond:    30,
	}
}

// DefaultStrictPolicy returns the ProtectionPolicy for ProtectionStrict mode.
func DefaultStrictPolicy() ProtectionPolicy {
	return ProtectionPolicy{
		MaxFloodWaitSeconds:   300,
		RetryCount:            5,
		AutoReconnect:         true,
		IgnoreMediaErrors:     false,
		MaxConcurrentRequests: 5,
		RateLimitPerSecond:    10,
	}
}

// DefaultOffPolicy returns the ProtectionPolicy for ProtectionOff mode.
func DefaultOffPolicy() ProtectionPolicy {
	return ProtectionPolicy{
		MaxFloodWaitSeconds:   0,
		RetryCount:            0,
		AutoReconnect:         false,
		IgnoreMediaErrors:     false,
		MaxConcurrentRequests: 0,
		RateLimitPerSecond:    0,
	}
}

// PolicyForMode returns the default ProtectionPolicy for the given mode.
// If mode is ProtectionCustom, returns DefaultSafePolicy as a starting point.
func PolicyForMode(mode ProtectionMode) ProtectionPolicy {
	switch mode {
	case ProtectionOff:
		return DefaultOffPolicy()
	case ProtectionSafe:
		return DefaultSafePolicy()
	case ProtectionStrict:
		return DefaultStrictPolicy()
	default:
		return DefaultSafePolicy()
	}
}
