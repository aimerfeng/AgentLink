package proxy

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/sony/gobreaker"
)

// CircuitBreakerConfig holds configuration for circuit breakers
type CircuitBreakerConfig struct {
	// MaxRequests is the maximum number of requests allowed to pass through
	// when the circuit breaker is half-open
	MaxRequests uint32
	// Interval is the cyclic period of the closed state
	// for the circuit breaker to clear the internal counts
	Interval time.Duration
	// Timeout is the period of the open state,
	// after which the state of the circuit breaker becomes half-open
	Timeout time.Duration
	// FailureThreshold is the number of failures before opening the circuit
	FailureThreshold uint32
	// SuccessThreshold is the number of successes needed to close the circuit
	SuccessThreshold uint32
}

// DefaultCircuitBreakerConfig returns default circuit breaker configuration
func DefaultCircuitBreakerConfig() *CircuitBreakerConfig {
	return &CircuitBreakerConfig{
		MaxRequests:      5,
		Interval:         60 * time.Second,
		Timeout:          30 * time.Second,
		FailureThreshold: 5,
		SuccessThreshold: 3,
	}
}

// CircuitBreakerManager manages circuit breakers for different providers
type CircuitBreakerManager struct {
	breakers map[string]*gobreaker.CircuitBreaker
	config   *CircuitBreakerConfig
	mu       sync.RWMutex
}

// CircuitBreakerState represents the state of a circuit breaker
type CircuitBreakerState string

const (
	CircuitBreakerStateClosed   CircuitBreakerState = "closed"
	CircuitBreakerStateOpen     CircuitBreakerState = "open"
	CircuitBreakerStateHalfOpen CircuitBreakerState = "half-open"
)

// CircuitBreakerStatus contains status information about a circuit breaker
type CircuitBreakerStatus struct {
	Name         string              `json:"name"`
	State        CircuitBreakerState `json:"state"`
	Requests     uint32              `json:"requests"`
	TotalSuccess uint32              `json:"total_success"`
	TotalFailure uint32              `json:"total_failure"`
}

// ErrCircuitOpen is returned when the circuit breaker is open
var ErrCircuitOpen = errors.New("circuit breaker is open")

// NewCircuitBreakerManager creates a new circuit breaker manager
func NewCircuitBreakerManager(config *CircuitBreakerConfig) *CircuitBreakerManager {
	if config == nil {
		config = DefaultCircuitBreakerConfig()
	}
	return &CircuitBreakerManager{
		breakers: make(map[string]*gobreaker.CircuitBreaker),
		config:   config,
	}
}

// GetBreaker returns or creates a circuit breaker for the given provider
func (m *CircuitBreakerManager) GetBreaker(provider string) *gobreaker.CircuitBreaker {
	m.mu.RLock()
	cb, exists := m.breakers[provider]
	m.mu.RUnlock()

	if exists {
		return cb
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock
	if cb, exists = m.breakers[provider]; exists {
		return cb
	}

	// Create new circuit breaker
	cb = gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        fmt.Sprintf("provider-%s", provider),
		MaxRequests: m.config.MaxRequests,
		Interval:    m.config.Interval,
		Timeout:     m.config.Timeout,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= m.config.FailureThreshold
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			log.Info().
				Str("circuit_breaker", name).
				Str("from", stateToString(from)).
				Str("to", stateToString(to)).
				Msg("Circuit breaker state changed")
		},
		IsSuccessful: func(err error) bool {
			// Consider these errors as failures that should trip the breaker
			if err == nil {
				return true
			}
			// Upstream errors should trip the breaker
			if errors.Is(err, ErrUpstreamError) || errors.Is(err, ErrUpstreamTimeout) {
				return false
			}
			// Client errors (invalid request, etc.) should not trip the breaker
			return true
		},
	})

	m.breakers[provider] = cb
	return cb
}

// Execute executes a function with circuit breaker protection
func (m *CircuitBreakerManager) Execute(ctx context.Context, provider string, fn func() (interface{}, error)) (interface{}, error) {
	cb := m.GetBreaker(provider)

	result, err := cb.Execute(func() (interface{}, error) {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		return fn()
	})

	if err != nil {
		if errors.Is(err, gobreaker.ErrOpenState) || errors.Is(err, gobreaker.ErrTooManyRequests) {
			log.Warn().
				Str("provider", provider).
				Msg("Circuit breaker is open, rejecting request")
			return nil, ErrCircuitOpen
		}
		return nil, err
	}

	return result, nil
}

// GetStatus returns the status of a circuit breaker
func (m *CircuitBreakerManager) GetStatus(provider string) *CircuitBreakerStatus {
	m.mu.RLock()
	cb, exists := m.breakers[provider]
	m.mu.RUnlock()

	if !exists {
		return nil
	}

	counts := cb.Counts()
	state := cb.State()

	return &CircuitBreakerStatus{
		Name:         provider,
		State:        CircuitBreakerState(stateToString(state)),
		Requests:     counts.Requests,
		TotalSuccess: counts.TotalSuccesses,
		TotalFailure: counts.TotalFailures,
	}
}

// GetAllStatus returns status of all circuit breakers
func (m *CircuitBreakerManager) GetAllStatus() []*CircuitBreakerStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	statuses := make([]*CircuitBreakerStatus, 0, len(m.breakers))
	for provider, cb := range m.breakers {
		counts := cb.Counts()
		state := cb.State()
		statuses = append(statuses, &CircuitBreakerStatus{
			Name:         provider,
			State:        CircuitBreakerState(stateToString(state)),
			Requests:     counts.Requests,
			TotalSuccess: counts.TotalSuccesses,
			TotalFailure: counts.TotalFailures,
		})
	}
	return statuses
}

// Reset resets a circuit breaker (for testing or admin purposes)
func (m *CircuitBreakerManager) Reset(provider string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.breakers, provider)
}

// ResetAll resets all circuit breakers
func (m *CircuitBreakerManager) ResetAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.breakers = make(map[string]*gobreaker.CircuitBreaker)
}

// IsOpen checks if the circuit breaker for a provider is open
func (m *CircuitBreakerManager) IsOpen(provider string) bool {
	m.mu.RLock()
	cb, exists := m.breakers[provider]
	m.mu.RUnlock()

	if !exists {
		return false
	}

	return cb.State() == gobreaker.StateOpen
}

// stateToString converts gobreaker.State to string
func stateToString(state gobreaker.State) string {
	switch state {
	case gobreaker.StateClosed:
		return string(CircuitBreakerStateClosed)
	case gobreaker.StateOpen:
		return string(CircuitBreakerStateOpen)
	case gobreaker.StateHalfOpen:
		return string(CircuitBreakerStateHalfOpen)
	default:
		return "unknown"
	}
}
