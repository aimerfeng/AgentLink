package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// PaymentMethod represents the payment method used
type PaymentMethod string

const (
	PaymentMethodStripe   PaymentMethod = "stripe"
	PaymentMethodCoinbase PaymentMethod = "coinbase"
)

// PaymentStatus represents the status of a payment
type PaymentStatus string

const (
	PaymentStatusPending   PaymentStatus = "pending"
	PaymentStatusCompleted PaymentStatus = "completed"
	PaymentStatusFailed    PaymentStatus = "failed"
)

// Payment represents a payment record
type Payment struct {
	ID             uuid.UUID       `json:"id" db:"id"`
	UserID         uuid.UUID       `json:"user_id" db:"user_id"`
	AmountUSD      decimal.Decimal `json:"amount_usd" db:"amount_usd"`
	QuotaPurchased int64           `json:"quota_purchased" db:"quota_purchased"`
	PaymentMethod  PaymentMethod   `json:"payment_method" db:"payment_method"`
	PaymentID      *string         `json:"payment_id,omitempty" db:"payment_id"`
	Status         PaymentStatus   `json:"status" db:"status"`
	FailureReason  *string         `json:"failure_reason,omitempty" db:"failure_reason"`
	CreatedAt      time.Time       `json:"created_at" db:"created_at"`
	CompletedAt    *time.Time      `json:"completed_at,omitempty" db:"completed_at"`
	FailedAt       *time.Time      `json:"failed_at,omitempty" db:"failed_at"`
}

// SettlementStatus represents the status of a settlement
type SettlementStatus string

const (
	SettlementStatusPending   SettlementStatus = "pending"
	SettlementStatusCompleted SettlementStatus = "completed"
	SettlementStatusFailed    SettlementStatus = "failed"
)

// Settlement represents a creator settlement record
type Settlement struct {
	ID          uuid.UUID        `json:"id" db:"id"`
	CreatorID   uuid.UUID        `json:"creator_id" db:"creator_id"`
	Amount      decimal.Decimal  `json:"amount" db:"amount"`
	PlatformFee decimal.Decimal  `json:"platform_fee" db:"platform_fee"`
	NetAmount   decimal.Decimal  `json:"net_amount" db:"net_amount"`
	TxHash      *string          `json:"tx_hash,omitempty" db:"tx_hash"`
	Status      SettlementStatus `json:"status" db:"status"`
	CreatedAt   time.Time        `json:"created_at" db:"created_at"`
	SettledAt   *time.Time       `json:"settled_at,omitempty" db:"settled_at"`
}
