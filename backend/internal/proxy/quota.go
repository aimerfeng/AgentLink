package proxy

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

// QuotaManager handles quota operations with atomic guarantees
type QuotaManager struct {
	service *Service
}

// NewQuotaManager creates a new quota manager
func NewQuotaManager(svc *Service) *QuotaManager {
	return &QuotaManager{service: svc}
}

// Lua script for atomic quota check and decrement
// Returns: remaining quota after decrement, or -1 if insufficient
const luaDecrementQuota = `
local key = KEYS[1]
local amount = tonumber(ARGV[1])

local current = redis.call('GET', key)
if not current then
    return -1
end

current = tonumber(current)
if current < amount then
    return -1
end

local new_value = current - amount
redis.call('SET', key, new_value)
return new_value
`

// Lua script for atomic quota check, decrement, and rate limit check
// Returns: {remaining_quota, rate_limit_count, allowed}
const luaCheckAndDecrement = `
local quota_key = KEYS[1]
local rate_key = KEYS[2]
local amount = tonumber(ARGV[1])
local rate_limit = tonumber(ARGV[2])
local rate_window = tonumber(ARGV[3])

-- Check rate limit first
local rate_count = redis.call('INCR', rate_key)
if rate_count == 1 then
    redis.call('EXPIRE', rate_key, rate_window)
end

if rate_count > rate_limit then
    return {-2, rate_count, 0}  -- Rate limited
end

-- Check and decrement quota
local current = redis.call('GET', quota_key)
if not current then
    return {-1, rate_count, 0}  -- No quota found
end

current = tonumber(current)
if current < amount then
    return {current, rate_count, 0}  -- Insufficient quota
end

local new_value = current - amount
redis.call('SET', quota_key, new_value)
return {new_value, rate_count, 1}  -- Success
`

// AtomicDecrementQuota decrements quota atomically using Lua script
// Returns remaining quota or error if insufficient
func (qm *QuotaManager) AtomicDecrementQuota(ctx context.Context, userID uuid.UUID, amount int64) (int64, error) {
	key := fmt.Sprintf("quota:%s", userID.String())
	
	result, err := qm.service.redis.Client.Eval(ctx, luaDecrementQuota, []string{key}, amount).Int64()
	if err != nil {
		if err == redis.Nil {
			// Key doesn't exist, sync from database
			return qm.syncQuotaFromDB(ctx, userID, amount)
		}
		return 0, fmt.Errorf("failed to decrement quota: %w", err)
	}

	if result < 0 {
		// Insufficient quota or key not found
		return 0, ErrQuotaExhausted
	}

	// Async sync to database
	go qm.syncQuotaToDB(context.Background(), userID, amount)

	return result, nil
}

// CheckAndDecrementQuota atomically checks rate limit and decrements quota
// Returns (remaining_quota, rate_count, allowed, error)
func (qm *QuotaManager) CheckAndDecrementQuota(ctx context.Context, userID uuid.UUID, amount int64, rateLimit int, windowSeconds int) (int64, int64, bool, error) {
	quotaKey := fmt.Sprintf("quota:%s", userID.String())
	rateKey := fmt.Sprintf("ratelimit:%s:%d", userID.String(), windowSeconds)

	result, err := qm.service.redis.Client.Eval(ctx, luaCheckAndDecrement, 
		[]string{quotaKey, rateKey}, 
		amount, rateLimit, windowSeconds,
	).Int64Slice()
	
	if err != nil {
		if err == redis.Nil {
			// Quota key doesn't exist, sync from database
			if _, syncErr := qm.syncQuotaFromDB(ctx, userID, 0); syncErr != nil {
				return 0, 0, false, syncErr
			}
			// Retry the operation
			return qm.CheckAndDecrementQuota(ctx, userID, amount, rateLimit, windowSeconds)
		}
		return 0, 0, false, fmt.Errorf("failed to check and decrement: %w", err)
	}

	if len(result) != 3 {
		return 0, 0, false, fmt.Errorf("unexpected result length: %d", len(result))
	}

	quotaRemaining := result[0]
	rateCount := result[1]
	allowed := result[2] == 1

	// Handle special cases
	if quotaRemaining == -2 {
		return 0, rateCount, false, ErrRateLimited
	}
	if quotaRemaining == -1 {
		// Sync from database and retry
		if _, syncErr := qm.syncQuotaFromDB(ctx, userID, 0); syncErr != nil {
			return 0, rateCount, false, syncErr
		}
		return qm.CheckAndDecrementQuota(ctx, userID, amount, rateLimit, windowSeconds)
	}

	if !allowed && quotaRemaining >= 0 {
		return quotaRemaining, rateCount, false, ErrQuotaExhausted
	}

	// Async sync to database if successful
	if allowed {
		go qm.syncQuotaToDB(context.Background(), userID, amount)
	}

	return quotaRemaining, rateCount, allowed, nil
}


// syncQuotaFromDB syncs quota from database to Redis
func (qm *QuotaManager) syncQuotaFromDB(ctx context.Context, userID uuid.UUID, decrementAmount int64) (int64, error) {
	// Get quota from database
	var totalQuota, usedQuota, freeQuota int64
	err := qm.service.db.QueryRow(ctx, `
		SELECT total_quota, used_quota, free_quota
		FROM quotas WHERE user_id = $1
	`, userID).Scan(&totalQuota, &usedQuota, &freeQuota)
	if err != nil {
		return 0, fmt.Errorf("failed to get quota from database: %w", err)
	}

	remaining := totalQuota + freeQuota - usedQuota

	// If we need to decrement, do it in the database
	if decrementAmount > 0 {
		if remaining < decrementAmount {
			return remaining, ErrQuotaExhausted
		}
		
		err = qm.service.db.QueryRow(ctx, `
			UPDATE quotas 
			SET used_quota = used_quota + $1, updated_at = NOW()
			WHERE user_id = $2
			RETURNING (total_quota + free_quota - used_quota)
		`, decrementAmount, userID).Scan(&remaining)
		if err != nil {
			return 0, fmt.Errorf("failed to decrement quota in database: %w", err)
		}
	}

	// Cache in Redis
	key := fmt.Sprintf("quota:%s", userID.String())
	err = qm.service.redis.Client.Set(ctx, key, remaining, 0).Err()
	if err != nil {
		log.Warn().Err(err).Str("user_id", userID.String()).Msg("Failed to cache quota in Redis")
	}

	return remaining, nil
}

// syncQuotaToDB syncs quota decrement to database
func (qm *QuotaManager) syncQuotaToDB(ctx context.Context, userID uuid.UUID, amount int64) {
	_, err := qm.service.db.Exec(ctx, `
		UPDATE quotas 
		SET used_quota = used_quota + $1, updated_at = NOW()
		WHERE user_id = $2
	`, amount, userID)
	if err != nil {
		log.Error().Err(err).
			Str("user_id", userID.String()).
			Int64("amount", amount).
			Msg("Failed to sync quota to database")
	}
}

// RefundQuotaAtomic refunds quota atomically
func (qm *QuotaManager) RefundQuotaAtomic(ctx context.Context, userID uuid.UUID, amount int64) error {
	key := fmt.Sprintf("quota:%s", userID.String())
	
	// Increment in Redis
	if redisErr := qm.service.redis.Client.IncrBy(ctx, key, amount).Err(); redisErr != nil {
		log.Warn().Err(redisErr).Str("user_id", userID.String()).Msg("Failed to refund quota in Redis")
	}

	// Update database
	_, dbErr := qm.service.db.Exec(ctx, `
		UPDATE quotas 
		SET used_quota = GREATEST(0, used_quota - $1), updated_at = NOW()
		WHERE user_id = $2
	`, amount, userID)
	if dbErr != nil {
		return fmt.Errorf("failed to refund quota in database: %w", dbErr)
	}

	return nil
}

// GetQuotaInfo returns detailed quota information
func (qm *QuotaManager) GetQuotaInfo(ctx context.Context, userID uuid.UUID) (*QuotaInfo, error) {
	var info QuotaInfo
	err := qm.service.db.QueryRow(ctx, `
		SELECT total_quota, used_quota, free_quota, updated_at
		FROM quotas WHERE user_id = $1
	`, userID).Scan(&info.TotalQuota, &info.UsedQuota, &info.FreeQuota, &info.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to get quota info: %w", err)
	}

	info.RemainingQuota = info.TotalQuota + info.FreeQuota - info.UsedQuota
	return &info, nil
}

// QuotaInfo represents detailed quota information
type QuotaInfo struct {
	TotalQuota     int64     `json:"total_quota"`
	UsedQuota      int64     `json:"used_quota"`
	FreeQuota      int64     `json:"free_quota"`
	RemainingQuota int64     `json:"remaining_quota"`
	UpdatedAt      string    `json:"updated_at"`
}

// EnsureQuotaInRedis ensures the user's quota is cached in Redis
func (qm *QuotaManager) EnsureQuotaInRedis(ctx context.Context, userID uuid.UUID) (int64, error) {
	key := fmt.Sprintf("quota:%s", userID.String())
	
	// Check if already in Redis
	remaining, err := qm.service.redis.Client.Get(ctx, key).Int64()
	if err == nil {
		return remaining, nil
	}
	
	if err != redis.Nil {
		return 0, fmt.Errorf("failed to get quota from Redis: %w", err)
	}

	// Not in Redis, sync from database
	return qm.syncQuotaFromDB(ctx, userID, 0)
}
