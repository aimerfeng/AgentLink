package models

import (
	"time"

	"github.com/google/uuid"
)

// WebhookEvent represents the type of webhook event
type WebhookEvent string

const (
	WebhookEventQuotaLow      WebhookEvent = "quota.low"
	WebhookEventCallCompleted WebhookEvent = "call.completed"
	WebhookEventPaymentDone   WebhookEvent = "payment.completed"
)

// Webhook represents a webhook configuration
type Webhook struct {
	ID        uuid.UUID      `json:"id" db:"id"`
	UserID    uuid.UUID      `json:"user_id" db:"user_id"`
	URL       string         `json:"url" db:"url"`
	Events    []WebhookEvent `json:"events" db:"events"`
	Secret    string         `json:"-" db:"secret"`
	Active    bool           `json:"active" db:"active"`
	CreatedAt time.Time      `json:"created_at" db:"created_at"`
}
