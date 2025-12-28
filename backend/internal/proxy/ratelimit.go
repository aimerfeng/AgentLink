package proxy

import (
	"context"
	"fmt"
	"time"

	"github.com/aimerfeng/AgentLink/internal/cache"
	"github.com/aimerfeng/AgentLink/internal/config"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

// RateLimiter implements sliding window rate limiting using Redis
type RateLimiter struct {
	redis  *cache.Redis
	config *config.RateLimitConfig
}

// RateLimitResult contains the result of a rate limit check
type RateLimitResult struct {
	Allowed    bool
	Remaining  int64
	Limit      int
	RetryAfter time.Duration
	ResetAt    time.Time
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(redis *cache.Redis, cfg *config.RateLimitConfig) *RateLimiter {
	return &RateLimiter{
		redis:  redis,
		config: cfg,
	}
}

// Check checks if a request is allowed under the rate limit
// Uses sliding window algorithm for accurate rate limiting
func (r *RateLimiter) Check(ctx context.Context, userID string, isPaidUser bool) (*RateLimitResult, error) {
	limit := r.config.FreeUserLimit
	if isPaidUser {
		limit = r.config.PaidUserLimit
	}

	windowSeconds := r.config.WindowSeconds
	if windowSeconds <= 0 {
		windowSeconds = 60 // Default to 60 seconds
	}

	return r.checkSlidingWindow(ctx, userID, limit, windowSeconds)
}

// checkSlidingWindow implements sliding window rate limiting
// This provides more accurate rate limiting than fixed windows
func (r *RateLimiter) checkSlidingWindow(ctx context.Context, userID string, limit int, windowSeconds int) (*RateLimitResult, error) {
	now := time.Now()
	windowDuration := time.Duration(windowSeconds) * time.Second
	windowStart := now.Add(-windowDuration)

	key := fmt.Sprintf("ratelimit:sliding:%s", userID)

	// Use Redis sorted set for sliding window
	// Score = timestamp, Member = unique request ID
	pipe := r.redis.Client.Pipeline()

	// Remove old entries outside the window
	pipe.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%d", windowStart.UnixNano()))

	// Count current entries in window
	countCmd := pipe.ZCard(ctx, key)

	// Execute pipeline
	_, err := pipe.Exec(ctx)
	if err != nil {
		log.Error().Err(err).Str("user_id", userID).Msg("Failed to check rate limit")
		// On Redis error, allow the request (fail open)
		return &RateLimitResult{
			Allowed:   true,
			Remaining: int64(limit),
			Limit:     limit,
		}, nil
	}

	currentCount := countCmd.Val()
	remaining := int64(limit) - currentCount

	result := &RateLimitResult{
		Limit:   limit,
		ResetAt: now.Add(windowDuration),
	}

	if currentCount >= int64(limit) {
		// Rate limit exceeded
		result.Allowed = false
		result.Remaining = 0

		// Calculate retry after based on oldest entry
		oldestScore, err := r.redis.Client.ZRangeWithScores(ctx, key, 0, 0).Result()
		if err == nil && len(oldestScore) > 0 {
			oldestTime := time.Unix(0, int64(oldestScore[0].Score))
			result.RetryAfter = oldestTime.Add(windowDuration).Sub(now)
			if result.RetryAfter < 0 {
				result.RetryAfter = time.Second
			}
		} else {
			result.RetryAfter = windowDuration
		}

		return result, nil
	}

	// Add new entry
	requestID := fmt.Sprintf("%d-%s", now.UnixNano(), userID)
	err = r.redis.Client.ZAdd(ctx, key, redis.Z{
		Score:  float64(now.UnixNano()),
		Member: requestID,
	}).Err()
	if err != nil {
		log.Warn().Err(err).Str("user_id", userID).Msg("Failed to add rate limit entry")
	}

	// Set expiration on the key
	r.redis.Client.Expire(ctx, key, windowDuration*2)

	result.Allowed = true
	result.Remaining = remaining - 1 // Account for the request we just added
	if result.Remaining < 0 {
		result.Remaining = 0
	}

	return result, nil
}

// CheckSimple uses the simpler fixed window approach (for backward compatibility)
func (r *RateLimiter) CheckSimple(ctx context.Context, userID string, isPaidUser bool) (bool, int64, error) {
	limit := r.config.FreeUserLimit
	if isPaidUser {
		limit = r.config.PaidUserLimit
	}

	return r.redis.CheckRateLimit(ctx, userID, limit, r.config.WindowSeconds)
}

// Reset resets the rate limit for a user (for testing or admin purposes)
func (r *RateLimiter) Reset(ctx context.Context, userID string) error {
	key := fmt.Sprintf("ratelimit:sliding:%s", userID)
	return r.redis.Client.Del(ctx, key).Err()
}

// GetStatus returns the current rate limit status for a user
func (r *RateLimiter) GetStatus(ctx context.Context, userID string, isPaidUser bool) (*RateLimitResult, error) {
	limit := r.config.FreeUserLimit
	if isPaidUser {
		limit = r.config.PaidUserLimit
	}

	windowSeconds := r.config.WindowSeconds
	if windowSeconds <= 0 {
		windowSeconds = 60
	}

	now := time.Now()
	windowDuration := time.Duration(windowSeconds) * time.Second
	windowStart := now.Add(-windowDuration)

	key := fmt.Sprintf("ratelimit:sliding:%s", userID)

	// Count entries in current window
	count, err := r.redis.Client.ZCount(ctx, key,
		fmt.Sprintf("%d", windowStart.UnixNano()),
		fmt.Sprintf("%d", now.UnixNano())).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get rate limit status: %w", err)
	}

	remaining := int64(limit) - count
	if remaining < 0 {
		remaining = 0
	}

	return &RateLimitResult{
		Allowed:   count < int64(limit),
		Remaining: remaining,
		Limit:     limit,
		ResetAt:   now.Add(windowDuration),
	}, nil
}
