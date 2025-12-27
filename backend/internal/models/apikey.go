package models

import (
	"time"

	"github.com/google/uuid"
)

// APIKey represents a developer's API key
type APIKey struct {
	ID          uuid.UUID          `json:"id" db:"id"`
	UserID      uuid.UUID          `json:"user_id" db:"user_id"`
	KeyHash     string             `json:"-" db:"key_hash"`
	KeyPrefix   string             `json:"key_prefix" db:"key_prefix"`
	Name        *string            `json:"name,omitempty" db:"name"`
	Permissions map[string]bool    `json:"permissions" db:"permissions"`
	LastUsedAt  *time.Time         `json:"last_used_at,omitempty" db:"last_used_at"`
	CreatedAt   time.Time          `json:"created_at" db:"created_at"`
	RevokedAt   *time.Time         `json:"revoked_at,omitempty" db:"revoked_at"`
}

// Quota represents a user's API quota
type Quota struct {
	UserID     uuid.UUID `json:"user_id" db:"user_id"`
	TotalQuota int64     `json:"total_quota" db:"total_quota"`
	UsedQuota  int64     `json:"used_quota" db:"used_quota"`
	FreeQuota  int64     `json:"free_quota" db:"free_quota"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
}

// RemainingQuota returns the remaining quota for a user
func (q *Quota) RemainingQuota() int64 {
	return q.TotalQuota + q.FreeQuota - q.UsedQuota
}
