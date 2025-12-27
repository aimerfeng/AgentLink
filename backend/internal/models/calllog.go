package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// CallStatus represents the status of an API call
type CallStatus string

const (
	CallStatusSuccess CallStatus = "success"
	CallStatusError   CallStatus = "error"
	CallStatusTimeout CallStatus = "timeout"
)

// CallLog represents an API call log entry
type CallLog struct {
	ID           uuid.UUID       `json:"id" db:"id"`
	AgentID      uuid.UUID       `json:"agent_id" db:"agent_id"`
	APIKeyID     uuid.UUID       `json:"api_key_id" db:"api_key_id"`
	UserID       uuid.UUID       `json:"user_id" db:"user_id"`
	RequestID    *string         `json:"request_id,omitempty" db:"request_id"`
	InputTokens  *int            `json:"input_tokens,omitempty" db:"input_tokens"`
	OutputTokens *int            `json:"output_tokens,omitempty" db:"output_tokens"`
	LatencyMs    *int            `json:"latency_ms,omitempty" db:"latency_ms"`
	Status       CallStatus      `json:"status" db:"status"`
	ErrorCode    *string         `json:"error_code,omitempty" db:"error_code"`
	CostUSD      decimal.Decimal `json:"cost_usd" db:"cost_usd"`
	CreatedAt    time.Time       `json:"created_at" db:"created_at"`
}
