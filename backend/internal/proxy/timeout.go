package proxy

import (
	"context"
	"time"
)

// TimeoutConfig holds timeout configuration
type TimeoutConfig struct {
	// DefaultTimeout is the default request timeout
	DefaultTimeout time.Duration
	// MaxTimeout is the maximum allowed timeout
	MaxTimeout time.Duration
	// MinTimeout is the minimum allowed timeout
	MinTimeout time.Duration
}

// DefaultTimeoutConfig returns default timeout configuration
func DefaultTimeoutConfig() *TimeoutConfig {
	return &TimeoutConfig{
		DefaultTimeout: 30 * time.Second,
		MaxTimeout:     120 * time.Second,
		MinTimeout:     5 * time.Second,
	}
}

// TimeoutManager manages request timeouts
type TimeoutManager struct {
	config *TimeoutConfig
}

// NewTimeoutManager creates a new timeout manager
func NewTimeoutManager(config *TimeoutConfig) *TimeoutManager {
	if config == nil {
		config = DefaultTimeoutConfig()
	}
	return &TimeoutManager{
		config: config,
	}
}

// GetTimeout returns the appropriate timeout for a request
// If requestedTimeout is 0, returns the default timeout
// If requestedTimeout is outside bounds, it's clamped to min/max
func (t *TimeoutManager) GetTimeout(requestedTimeout time.Duration) time.Duration {
	if requestedTimeout == 0 {
		return t.config.DefaultTimeout
	}

	if requestedTimeout < t.config.MinTimeout {
		return t.config.MinTimeout
	}

	if requestedTimeout > t.config.MaxTimeout {
		return t.config.MaxTimeout
	}

	return requestedTimeout
}

// WithTimeout creates a context with the specified timeout
// Returns the context, cancel function, and the actual timeout used
func (t *TimeoutManager) WithTimeout(ctx context.Context, requestedTimeout time.Duration) (context.Context, context.CancelFunc, time.Duration) {
	timeout := t.GetTimeout(requestedTimeout)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	return ctx, cancel, timeout
}

// IsTimeoutError checks if an error is a timeout error
func IsTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	return err == context.DeadlineExceeded || err == ErrUpstreamTimeout
}

// GetDefaultTimeout returns the default timeout
func (t *TimeoutManager) GetDefaultTimeout() time.Duration {
	return t.config.DefaultTimeout
}

// GetMaxTimeout returns the maximum timeout
func (t *TimeoutManager) GetMaxTimeout() time.Duration {
	return t.config.MaxTimeout
}

// GetMinTimeout returns the minimum timeout
func (t *TimeoutManager) GetMinTimeout() time.Duration {
	return t.config.MinTimeout
}
