package withdrawal

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aimerfeng/AgentLink/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// Service errors
var (
	ErrInsufficientBalance    = errors.New("insufficient balance for withdrawal")
	ErrBelowMinimumThreshold  = errors.New("withdrawal amount below minimum threshold")
	ErrWithdrawalNotFound     = errors.New("withdrawal not found")
	ErrWithdrawalNotPending   = errors.New("withdrawal is not in pending status")
	ErrWithdrawalAlreadyDone  = errors.New("withdrawal already completed or failed")
	ErrCreatorNotFound        = errors.New("creator not found")
	ErrNotCreator             = errors.New("user is not a creator")
	ErrNoWalletAddress        = errors.New("no wallet address configured for crypto withdrawal")
	ErrInvalidWithdrawalMethod = errors.New("invalid withdrawal method")
)

// Service handles withdrawal operations
type Service struct {
	db     *pgxpool.Pool
	config *models.WithdrawalConfig
}

// NewService creates a new withdrawal service
func NewService(db *pgxpool.Pool, config *models.WithdrawalConfig) *Service {
	if config == nil {
		config = models.DefaultWithdrawalConfig()
	}
	return &Service{
		db:     db,
		config: config,
	}
}

// CreateWithdrawalRequest represents a request to create a withdrawal
type CreateWithdrawalRequest struct {
	Amount           decimal.Decimal          `json:"amount" binding:"required"`
	WithdrawalMethod models.WithdrawalMethod  `json:"withdrawal_method" binding:"required"`
	DestinationAddress *string                `json:"destination_address,omitempty"`
}

// CreateWithdrawalResponse represents the response from creating a withdrawal
type CreateWithdrawalResponse struct {
	Withdrawal *models.Withdrawal `json:"withdrawal"`
	Message    string             `json:"message"`
}

// WithdrawalHistoryResponse represents the withdrawal history response
type WithdrawalHistoryResponse struct {
	Withdrawals []models.Withdrawal `json:"withdrawals"`
	Total       int                 `json:"total"`
	Page        int                 `json:"page"`
	PageSize    int                 `json:"page_size"`
	TotalPages  int                 `json:"total_pages"`
}

// EarningsInfo represents creator earnings information
type EarningsInfo struct {
	TotalEarnings     decimal.Decimal `json:"total_earnings"`
	PendingEarnings   decimal.Decimal `json:"pending_earnings"`
	AvailableBalance  decimal.Decimal `json:"available_balance"`
	MinimumWithdrawal decimal.Decimal `json:"minimum_withdrawal"`
	PlatformFeeRate   decimal.Decimal `json:"platform_fee_rate"`
}


// GetEarningsInfo retrieves the earnings information for a creator
func (s *Service) GetEarningsInfo(ctx context.Context, creatorID uuid.UUID) (*EarningsInfo, error) {
	// Verify user is a creator
	var userType models.UserType
	err := s.db.QueryRow(ctx, `
		SELECT user_type FROM users WHERE id = $1
	`, creatorID).Scan(&userType)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrCreatorNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	if userType != models.UserTypeCreator {
		return nil, ErrNotCreator
	}

	// Get earnings from creator_profiles
	var totalEarnings, pendingEarnings decimal.Decimal
	err = s.db.QueryRow(ctx, `
		SELECT COALESCE(total_earnings, 0), COALESCE(pending_earnings, 0)
		FROM creator_profiles WHERE user_id = $1
	`, creatorID).Scan(&totalEarnings, &pendingEarnings)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrCreatorNotFound
		}
		return nil, fmt.Errorf("failed to get creator profile: %w", err)
	}

	return &EarningsInfo{
		TotalEarnings:     totalEarnings,
		PendingEarnings:   pendingEarnings,
		AvailableBalance:  pendingEarnings,
		MinimumWithdrawal: s.config.MinimumAmount,
		PlatformFeeRate:   s.config.PlatformFeeRate,
	}, nil
}

// CalculateFee calculates the platform fee for a withdrawal amount
func (s *Service) CalculateFee(amount decimal.Decimal) decimal.Decimal {
	return amount.Mul(s.config.PlatformFeeRate).Round(8)
}

// CalculateFeeForMethod calculates the platform fee for a specific withdrawal method
// Different methods may have different fee structures
func (s *Service) CalculateFeeForMethod(amount decimal.Decimal, method models.WithdrawalMethod) decimal.Decimal {
	baseFee := s.CalculateFee(amount)
	
	// Apply method-specific adjustments
	switch method {
	case models.WithdrawalMethodCrypto:
		// Crypto withdrawals have lower fees (0.5% discount)
		discount := amount.Mul(decimal.NewFromFloat(0.005)).Round(8)
		adjustedFee := baseFee.Sub(discount)
		if adjustedFee.LessThan(decimal.Zero) {
			return decimal.Zero
		}
		return adjustedFee
	case models.WithdrawalMethodBank:
		// Bank transfers have a fixed minimum fee of $1.00
		minFee := decimal.NewFromFloat(1.00)
		if baseFee.LessThan(minFee) {
			return minFee
		}
		return baseFee
	case models.WithdrawalMethodStripe:
		// Stripe has standard platform fee
		return baseFee
	default:
		return baseFee
	}
}

// CalculateNetAmount calculates the net amount after platform fee
func (s *Service) CalculateNetAmount(amount decimal.Decimal) decimal.Decimal {
	fee := s.CalculateFee(amount)
	return amount.Sub(fee)
}

// CalculateNetAmountForMethod calculates the net amount for a specific withdrawal method
func (s *Service) CalculateNetAmountForMethod(amount decimal.Decimal, method models.WithdrawalMethod) decimal.Decimal {
	fee := s.CalculateFeeForMethod(amount, method)
	return amount.Sub(fee)
}

// FeeBreakdown represents a detailed breakdown of withdrawal fees
type FeeBreakdown struct {
	GrossAmount   decimal.Decimal `json:"gross_amount"`
	PlatformFee   decimal.Decimal `json:"platform_fee"`
	MethodFee     decimal.Decimal `json:"method_fee"`
	TotalFee      decimal.Decimal `json:"total_fee"`
	NetAmount     decimal.Decimal `json:"net_amount"`
	FeePercentage decimal.Decimal `json:"fee_percentage"`
}

// CalculateFeeBreakdown provides a detailed breakdown of all fees
func (s *Service) CalculateFeeBreakdown(amount decimal.Decimal, method models.WithdrawalMethod) *FeeBreakdown {
	baseFee := s.CalculateFee(amount)
	methodFee := s.CalculateFeeForMethod(amount, method)
	
	// Method fee is the difference from base fee (can be negative for discounts)
	methodAdjustment := methodFee.Sub(baseFee)
	
	totalFee := methodFee
	netAmount := amount.Sub(totalFee)
	
	// Calculate fee percentage
	var feePercentage decimal.Decimal
	if amount.GreaterThan(decimal.Zero) {
		feePercentage = totalFee.Div(amount).Mul(decimal.NewFromInt(100)).Round(2)
	}
	
	return &FeeBreakdown{
		GrossAmount:   amount,
		PlatformFee:   baseFee,
		MethodFee:     methodAdjustment,
		TotalFee:      totalFee,
		NetAmount:     netAmount,
		FeePercentage: feePercentage,
	}
}

// ValidateWithdrawalAmount validates if the withdrawal amount is valid
func (s *Service) ValidateWithdrawalAmount(amount, availableBalance decimal.Decimal) error {
	if amount.LessThan(s.config.MinimumAmount) {
		return ErrBelowMinimumThreshold
	}
	if amount.GreaterThan(availableBalance) {
		return ErrInsufficientBalance
	}
	return nil
}

// CreateWithdrawal creates a new withdrawal request
func (s *Service) CreateWithdrawal(ctx context.Context, creatorID uuid.UUID, req *CreateWithdrawalRequest) (*CreateWithdrawalResponse, error) {
	// Validate withdrawal method
	switch req.WithdrawalMethod {
	case models.WithdrawalMethodStripe, models.WithdrawalMethodCrypto, models.WithdrawalMethodBank:
		// Valid methods
	default:
		return nil, ErrInvalidWithdrawalMethod
	}

	// Get earnings info
	earnings, err := s.GetEarningsInfo(ctx, creatorID)
	if err != nil {
		return nil, err
	}

	// Validate amount
	if err := s.ValidateWithdrawalAmount(req.Amount, earnings.AvailableBalance); err != nil {
		return nil, err
	}

	// For crypto withdrawals, check wallet address
	var destinationAddress *string
	if req.WithdrawalMethod == models.WithdrawalMethodCrypto {
		if req.DestinationAddress != nil && *req.DestinationAddress != "" {
			destinationAddress = req.DestinationAddress
		} else {
			// Try to get wallet address from user profile
			var walletAddr *string
			err := s.db.QueryRow(ctx, `
				SELECT wallet_address FROM users WHERE id = $1
			`, creatorID).Scan(&walletAddr)
			if err != nil {
				return nil, fmt.Errorf("failed to get wallet address: %w", err)
			}
			if walletAddr == nil || *walletAddr == "" {
				return nil, ErrNoWalletAddress
			}
			destinationAddress = walletAddr
		}
	} else if req.DestinationAddress != nil {
		destinationAddress = req.DestinationAddress
	}

	// Calculate fee and net amount using method-specific calculation
	platformFee := s.CalculateFeeForMethod(req.Amount, req.WithdrawalMethod)
	netAmount := s.CalculateNetAmountForMethod(req.Amount, req.WithdrawalMethod)

	// Start transaction
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Deduct from pending_earnings
	result, err := tx.Exec(ctx, `
		UPDATE creator_profiles 
		SET pending_earnings = pending_earnings - $1
		WHERE user_id = $2 AND pending_earnings >= $1
	`, req.Amount, creatorID)
	if err != nil {
		return nil, fmt.Errorf("failed to deduct earnings: %w", err)
	}
	if result.RowsAffected() == 0 {
		return nil, ErrInsufficientBalance
	}

	// Create withdrawal record
	withdrawalID := uuid.New()
	_, err = tx.Exec(ctx, `
		INSERT INTO withdrawals (id, creator_id, amount, platform_fee, net_amount, 
		                         withdrawal_method, destination_address, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, withdrawalID, creatorID, req.Amount, platformFee, netAmount,
		req.WithdrawalMethod, destinationAddress, models.WithdrawalStatusPending)
	if err != nil {
		return nil, fmt.Errorf("failed to create withdrawal record: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Fetch the created withdrawal
	withdrawal, err := s.GetWithdrawalByID(ctx, withdrawalID)
	if err != nil {
		return nil, err
	}

	return &CreateWithdrawalResponse{
		Withdrawal: withdrawal,
		Message:    "Withdrawal request created successfully",
	}, nil
}


// GetWithdrawalByID retrieves a withdrawal by ID
func (s *Service) GetWithdrawalByID(ctx context.Context, withdrawalID uuid.UUID) (*models.Withdrawal, error) {
	var w models.Withdrawal
	err := s.db.QueryRow(ctx, `
		SELECT id, creator_id, amount, platform_fee, net_amount, withdrawal_method,
		       destination_address, status, failure_reason, external_tx_id,
		       created_at, processed_at, completed_at, failed_at
		FROM withdrawals WHERE id = $1
	`, withdrawalID).Scan(
		&w.ID, &w.CreatorID, &w.Amount, &w.PlatformFee, &w.NetAmount, &w.WithdrawalMethod,
		&w.DestinationAddress, &w.Status, &w.FailureReason, &w.ExternalTxID,
		&w.CreatedAt, &w.ProcessedAt, &w.CompletedAt, &w.FailedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrWithdrawalNotFound
		}
		return nil, fmt.Errorf("failed to get withdrawal: %w", err)
	}
	return &w, nil
}

// GetWithdrawalHistory retrieves withdrawal history for a creator
func (s *Service) GetWithdrawalHistory(ctx context.Context, creatorID uuid.UUID, page, pageSize int) (*WithdrawalHistoryResponse, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	// Get total count
	var total int
	err := s.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM withdrawals WHERE creator_id = $1
	`, creatorID).Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("failed to count withdrawals: %w", err)
	}

	// Get withdrawals
	rows, err := s.db.Query(ctx, `
		SELECT id, creator_id, amount, platform_fee, net_amount, withdrawal_method,
		       destination_address, status, failure_reason, external_tx_id,
		       created_at, processed_at, completed_at, failed_at
		FROM withdrawals
		WHERE creator_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, creatorID, pageSize, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query withdrawals: %w", err)
	}
	defer rows.Close()

	var withdrawals []models.Withdrawal
	for rows.Next() {
		var w models.Withdrawal
		err := rows.Scan(
			&w.ID, &w.CreatorID, &w.Amount, &w.PlatformFee, &w.NetAmount, &w.WithdrawalMethod,
			&w.DestinationAddress, &w.Status, &w.FailureReason, &w.ExternalTxID,
			&w.CreatedAt, &w.ProcessedAt, &w.CompletedAt, &w.FailedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan withdrawal: %w", err)
		}
		withdrawals = append(withdrawals, w)
	}

	return &WithdrawalHistoryResponse{
		Withdrawals: withdrawals,
		Total:       total,
		Page:        page,
		PageSize:    pageSize,
		TotalPages:  (total + pageSize - 1) / pageSize,
	}, nil
}

// CompleteWithdrawal marks a withdrawal as completed
func (s *Service) CompleteWithdrawal(ctx context.Context, withdrawalID uuid.UUID, externalTxID string) error {
	now := time.Now()
	result, err := s.db.Exec(ctx, `
		UPDATE withdrawals 
		SET status = $1, external_tx_id = $2, completed_at = $3, processed_at = COALESCE(processed_at, $3)
		WHERE id = $4 AND status IN ($5, $6)
	`, models.WithdrawalStatusCompleted, externalTxID, now, withdrawalID,
		models.WithdrawalStatusPending, models.WithdrawalStatusProcessing)
	if err != nil {
		return fmt.Errorf("failed to complete withdrawal: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrWithdrawalAlreadyDone
	}
	return nil
}

// FailureReason represents categorized failure reasons
type FailureReason string

const (
	FailureReasonInsufficientFunds   FailureReason = "insufficient_funds"
	FailureReasonInvalidDestination  FailureReason = "invalid_destination"
	FailureReasonProviderError       FailureReason = "provider_error"
	FailureReasonNetworkError        FailureReason = "network_error"
	FailureReasonTimeout             FailureReason = "timeout"
	FailureReasonRejected            FailureReason = "rejected"
	FailureReasonUnknown             FailureReason = "unknown"
)

// IsRetryable returns whether a failure reason is retryable
func (r FailureReason) IsRetryable() bool {
	switch r {
	case FailureReasonProviderError, FailureReasonNetworkError, FailureReasonTimeout:
		return true
	default:
		return false
	}
}

// FailWithdrawalRequest represents a request to fail a withdrawal
type FailWithdrawalRequest struct {
	Reason       FailureReason `json:"reason"`
	Description  string        `json:"description"`
	RestoreFunds bool          `json:"restore_funds"` // Whether to restore funds to creator
}

// FailWithdrawal marks a withdrawal as failed and optionally restores the funds
func (s *Service) FailWithdrawal(ctx context.Context, withdrawalID uuid.UUID, reason string) error {
	return s.FailWithdrawalWithOptions(ctx, withdrawalID, &FailWithdrawalRequest{
		Reason:       FailureReasonUnknown,
		Description:  reason,
		RestoreFunds: true,
	})
}

// FailWithdrawalWithOptions marks a withdrawal as failed with detailed options
func (s *Service) FailWithdrawalWithOptions(ctx context.Context, withdrawalID uuid.UUID, req *FailWithdrawalRequest) error {
	// Start transaction
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Get withdrawal details
	var creatorID uuid.UUID
	var amount decimal.Decimal
	var status models.WithdrawalStatus
	err = tx.QueryRow(ctx, `
		SELECT creator_id, amount, status FROM withdrawals WHERE id = $1 FOR UPDATE
	`, withdrawalID).Scan(&creatorID, &amount, &status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrWithdrawalNotFound
		}
		return fmt.Errorf("failed to get withdrawal: %w", err)
	}

	// Check if already processed
	if status == models.WithdrawalStatusCompleted || status == models.WithdrawalStatusFailed {
		return ErrWithdrawalAlreadyDone
	}

	// Build failure reason string
	failureReason := fmt.Sprintf("[%s] %s", req.Reason, req.Description)

	// Update withdrawal status
	now := time.Now()
	_, err = tx.Exec(ctx, `
		UPDATE withdrawals 
		SET status = $1, failure_reason = $2, failed_at = $3
		WHERE id = $4
	`, models.WithdrawalStatusFailed, failureReason, now, withdrawalID)
	if err != nil {
		return fmt.Errorf("failed to update withdrawal status: %w", err)
	}

	// Restore funds to pending_earnings if requested
	if req.RestoreFunds {
		_, err = tx.Exec(ctx, `
			UPDATE creator_profiles 
			SET pending_earnings = pending_earnings + $1
			WHERE user_id = $2
		`, amount, creatorID)
		if err != nil {
			return fmt.Errorf("failed to restore earnings: %w", err)
		}
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// RetryWithdrawal creates a new withdrawal request from a failed one
func (s *Service) RetryWithdrawal(ctx context.Context, withdrawalID uuid.UUID, creatorID uuid.UUID) (*CreateWithdrawalResponse, error) {
	// Get the failed withdrawal
	withdrawal, err := s.GetWithdrawalByID(ctx, withdrawalID)
	if err != nil {
		return nil, err
	}

	// Verify ownership
	if withdrawal.CreatorID != creatorID {
		return nil, ErrNotCreator
	}

	// Check if withdrawal is in failed status
	if withdrawal.Status != models.WithdrawalStatusFailed {
		return nil, fmt.Errorf("can only retry failed withdrawals")
	}

	// Check if failure reason is retryable
	if withdrawal.FailureReason != nil {
		reason := extractFailureReason(*withdrawal.FailureReason)
		if !reason.IsRetryable() {
			return nil, fmt.Errorf("withdrawal failure is not retryable: %s", reason)
		}
	}

	// Create a new withdrawal request with the same parameters
	req := &CreateWithdrawalRequest{
		Amount:             withdrawal.Amount,
		WithdrawalMethod:   withdrawal.WithdrawalMethod,
		DestinationAddress: withdrawal.DestinationAddress,
	}

	return s.CreateWithdrawal(ctx, creatorID, req)
}

// extractFailureReason extracts the FailureReason from a failure reason string
func extractFailureReason(reason string) FailureReason {
	// Format: "[reason_type] description"
	if len(reason) > 2 && reason[0] == '[' {
		end := 1
		for end < len(reason) && reason[end] != ']' {
			end++
		}
		if end < len(reason) {
			return FailureReason(reason[1:end])
		}
	}
	return FailureReasonUnknown
}

// GetFailedWithdrawals retrieves all failed withdrawals for a creator
func (s *Service) GetFailedWithdrawals(ctx context.Context, creatorID uuid.UUID, page, pageSize int) (*WithdrawalHistoryResponse, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	// Get total count
	var total int
	err := s.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM withdrawals WHERE creator_id = $1 AND status = $2
	`, creatorID, models.WithdrawalStatusFailed).Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("failed to count failed withdrawals: %w", err)
	}

	// Get withdrawals
	rows, err := s.db.Query(ctx, `
		SELECT id, creator_id, amount, platform_fee, net_amount, withdrawal_method,
		       destination_address, status, failure_reason, external_tx_id,
		       created_at, processed_at, completed_at, failed_at
		FROM withdrawals
		WHERE creator_id = $1 AND status = $2
		ORDER BY failed_at DESC
		LIMIT $3 OFFSET $4
	`, creatorID, models.WithdrawalStatusFailed, pageSize, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query failed withdrawals: %w", err)
	}
	defer rows.Close()

	var withdrawals []models.Withdrawal
	for rows.Next() {
		var w models.Withdrawal
		err := rows.Scan(
			&w.ID, &w.CreatorID, &w.Amount, &w.PlatformFee, &w.NetAmount, &w.WithdrawalMethod,
			&w.DestinationAddress, &w.Status, &w.FailureReason, &w.ExternalTxID,
			&w.CreatedAt, &w.ProcessedAt, &w.CompletedAt, &w.FailedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan withdrawal: %w", err)
		}
		withdrawals = append(withdrawals, w)
	}

	return &WithdrawalHistoryResponse{
		Withdrawals: withdrawals,
		Total:       total,
		Page:        page,
		PageSize:    pageSize,
		TotalPages:  (total + pageSize - 1) / pageSize,
	}, nil
}

// SetWithdrawalProcessing marks a withdrawal as processing
func (s *Service) SetWithdrawalProcessing(ctx context.Context, withdrawalID uuid.UUID) error {
	now := time.Now()
	result, err := s.db.Exec(ctx, `
		UPDATE withdrawals 
		SET status = $1, processed_at = $2
		WHERE id = $3 AND status = $4
	`, models.WithdrawalStatusProcessing, now, withdrawalID, models.WithdrawalStatusPending)
	if err != nil {
		return fmt.Errorf("failed to set withdrawal processing: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrWithdrawalNotPending
	}
	return nil
}

// GetPendingWithdrawals retrieves all pending withdrawals (for admin/processing)
func (s *Service) GetPendingWithdrawals(ctx context.Context, page, pageSize int) (*WithdrawalHistoryResponse, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	// Get total count
	var total int
	err := s.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM withdrawals WHERE status = $1
	`, models.WithdrawalStatusPending).Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("failed to count pending withdrawals: %w", err)
	}

	// Get withdrawals
	rows, err := s.db.Query(ctx, `
		SELECT id, creator_id, amount, platform_fee, net_amount, withdrawal_method,
		       destination_address, status, failure_reason, external_tx_id,
		       created_at, processed_at, completed_at, failed_at
		FROM withdrawals
		WHERE status = $1
		ORDER BY created_at ASC
		LIMIT $2 OFFSET $3
	`, models.WithdrawalStatusPending, pageSize, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query pending withdrawals: %w", err)
	}
	defer rows.Close()

	var withdrawals []models.Withdrawal
	for rows.Next() {
		var w models.Withdrawal
		err := rows.Scan(
			&w.ID, &w.CreatorID, &w.Amount, &w.PlatformFee, &w.NetAmount, &w.WithdrawalMethod,
			&w.DestinationAddress, &w.Status, &w.FailureReason, &w.ExternalTxID,
			&w.CreatedAt, &w.ProcessedAt, &w.CompletedAt, &w.FailedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan withdrawal: %w", err)
		}
		withdrawals = append(withdrawals, w)
	}

	return &WithdrawalHistoryResponse{
		Withdrawals: withdrawals,
		Total:       total,
		Page:        page,
		PageSize:    pageSize,
		TotalPages:  (total + pageSize - 1) / pageSize,
	}, nil
}

// GetConfig returns the withdrawal configuration
func (s *Service) GetConfig() *models.WithdrawalConfig {
	return s.config
}
