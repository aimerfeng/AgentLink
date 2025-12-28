package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// WithdrawalMethod represents the method used for withdrawal
type WithdrawalMethod string

const (
	WithdrawalMethodStripe WithdrawalMethod = "stripe"
	WithdrawalMethodCrypto WithdrawalMethod = "crypto"
	WithdrawalMethodBank   WithdrawalMethod = "bank"
)

// WithdrawalStatus represents the status of a withdrawal
type WithdrawalStatus string

const (
	WithdrawalStatusPending    WithdrawalStatus = "pending"
	WithdrawalStatusProcessing WithdrawalStatus = "processing"
	WithdrawalStatusCompleted  WithdrawalStatus = "completed"
	WithdrawalStatusFailed     WithdrawalStatus = "failed"
)

// Withdrawal represents a creator withdrawal request
type Withdrawal struct {
	ID                 uuid.UUID        `json:"id" db:"id"`
	CreatorID          uuid.UUID        `json:"creator_id" db:"creator_id"`
	Amount             decimal.Decimal  `json:"amount" db:"amount"`
	PlatformFee        decimal.Decimal  `json:"platform_fee" db:"platform_fee"`
	NetAmount          decimal.Decimal  `json:"net_amount" db:"net_amount"`
	WithdrawalMethod   WithdrawalMethod `json:"withdrawal_method" db:"withdrawal_method"`
	DestinationAddress *string          `json:"destination_address,omitempty" db:"destination_address"`
	Status             WithdrawalStatus `json:"status" db:"status"`
	FailureReason      *string          `json:"failure_reason,omitempty" db:"failure_reason"`
	ExternalTxID       *string          `json:"external_tx_id,omitempty" db:"external_tx_id"`
	CreatedAt          time.Time        `json:"created_at" db:"created_at"`
	ProcessedAt        *time.Time       `json:"processed_at,omitempty" db:"processed_at"`
	CompletedAt        *time.Time       `json:"completed_at,omitempty" db:"completed_at"`
	FailedAt           *time.Time       `json:"failed_at,omitempty" db:"failed_at"`
}

// WithdrawalConfig holds withdrawal configuration
type WithdrawalConfig struct {
	MinimumAmount   decimal.Decimal `json:"minimum_amount"`   // Minimum withdrawal amount (default: $10.00)
	PlatformFeeRate decimal.Decimal `json:"platform_fee_rate"` // Platform fee percentage (default: 2.5%)
}

// DefaultWithdrawalConfig returns the default withdrawal configuration
func DefaultWithdrawalConfig() *WithdrawalConfig {
	return &WithdrawalConfig{
		MinimumAmount:   decimal.NewFromFloat(10.00),
		PlatformFeeRate: decimal.NewFromFloat(0.025), // 2.5%
	}
}
