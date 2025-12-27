package models

import (
	"time"

	"github.com/google/uuid"
)

// ReviewStatus represents the status of a review
type ReviewStatus string

const (
	ReviewStatusPending  ReviewStatus = "pending"
	ReviewStatusApproved ReviewStatus = "approved"
	ReviewStatusRejected ReviewStatus = "rejected"
)

// Review represents a user review of an agent
type Review struct {
	ID        uuid.UUID    `json:"id" db:"id"`
	AgentID   uuid.UUID    `json:"agent_id" db:"agent_id"`
	UserID    uuid.UUID    `json:"user_id" db:"user_id"`
	Rating    int          `json:"rating" db:"rating"`
	Content   *string      `json:"content,omitempty" db:"content"`
	Status    ReviewStatus `json:"status" db:"status"`
	CreatedAt time.Time    `json:"created_at" db:"created_at"`
}
