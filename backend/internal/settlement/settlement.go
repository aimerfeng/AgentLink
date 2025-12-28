package settlement

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
	ErrSettlementNotFound     = errors.New("settlement not found")
	ErrSettlementAlreadyDone  = errors.New("settlement already completed or failed")
	ErrCreatorNotFound        = errors.New("creator not found")
	ErrNotCreator             = errors.New("user is not a creator")
	ErrNoEarningsToSettle     = errors.New("no earnings to settle")
	ErrInvalidSettlementPeriod = errors.New("invalid settlement period")
	ErrSettlementInProgress   = errors.New("settlement already in progress for this period")
)

// SettlementConfig holds settlement configuration
type SettlementConfig struct {
	PlatformFeeRate    decimal.Decimal `json:"platform_fee_rate"`    // Platform fee percentage (default: 20%)
	MinSettlementAmount decimal.Decimal `json:"min_settlement_amount"` // Minimum amount to trigger settlement
	SettlementPeriodDays int           `json:"settlement_period_days"` // Days between settlements (default: 7)
}

// DefaultSettlementConfig returns the default settlement configuration
func DefaultSettlementConfig() *SettlementConfig {
	return &SettlementConfig{
		PlatformFeeRate:     decimal.NewFromFloat(0.20), // 20% platform fee
		MinSettlementAmount: decimal.NewFromFloat(1.00), // Minimum $1.00 to settle
		SettlementPeriodDays: 7,                         // Weekly settlements
	}
}

// Service handles settlement operations
type Service struct {
	db     *pgxpool.Pool
	config *SettlementConfig
}

// NewService creates a new settlement service
func NewService(db *pgxpool.Pool, config *SettlementConfig) *Service {
	if config == nil {
		config = DefaultSettlementConfig()
	}
	return &Service{
		db:     db,
		config: config,
	}
}

// CreatorEarnings represents earnings data for a creator
type CreatorEarnings struct {
	CreatorID       uuid.UUID       `json:"creator_id"`
	TotalRevenue    decimal.Decimal `json:"total_revenue"`
	TotalCalls      int64           `json:"total_calls"`
	PeriodStart     time.Time       `json:"period_start"`
	PeriodEnd       time.Time       `json:"period_end"`
}

// SettlementCalculation represents the calculated settlement for a creator
type SettlementCalculation struct {
	CreatorID     uuid.UUID       `json:"creator_id"`
	GrossAmount   decimal.Decimal `json:"gross_amount"`
	PlatformFee   decimal.Decimal `json:"platform_fee"`
	NetAmount     decimal.Decimal `json:"net_amount"`
	TotalCalls    int64           `json:"total_calls"`
	AgentBreakdown []AgentEarnings `json:"agent_breakdown"`
	PeriodStart   time.Time       `json:"period_start"`
	PeriodEnd     time.Time       `json:"period_end"`
}

// AgentEarnings represents earnings for a specific agent
type AgentEarnings struct {
	AgentID     uuid.UUID       `json:"agent_id"`
	AgentName   string          `json:"agent_name"`
	TotalCalls  int64           `json:"total_calls"`
	Revenue     decimal.Decimal `json:"revenue"`
}


// SettlementSummary represents a summary of settlements
type SettlementSummary struct {
	TotalSettlements    int             `json:"total_settlements"`
	TotalGrossAmount    decimal.Decimal `json:"total_gross_amount"`
	TotalPlatformFees   decimal.Decimal `json:"total_platform_fees"`
	TotalNetAmount      decimal.Decimal `json:"total_net_amount"`
	PendingSettlements  int             `json:"pending_settlements"`
	CompletedSettlements int            `json:"completed_settlements"`
	FailedSettlements   int             `json:"failed_settlements"`
}

// SettlementHistoryResponse represents the settlement history response
type SettlementHistoryResponse struct {
	Settlements []models.Settlement `json:"settlements"`
	Total       int                 `json:"total"`
	Page        int                 `json:"page"`
	PageSize    int                 `json:"page_size"`
	TotalPages  int                 `json:"total_pages"`
}

// CalculatePlatformFee calculates the platform fee for a given amount
// Implements Requirement A8.3: Settlement calculation
func (s *Service) CalculatePlatformFee(amount decimal.Decimal) decimal.Decimal {
	return amount.Mul(s.config.PlatformFeeRate).Round(8)
}

// CalculateNetAmount calculates the net amount after platform fee
func (s *Service) CalculateNetAmount(amount decimal.Decimal) decimal.Decimal {
	fee := s.CalculatePlatformFee(amount)
	return amount.Sub(fee)
}

// CalculateSettlement calculates the settlement for a creator for a given period
// Implements Requirement A8.3: WHEN settlement period triggers, THE Settlement_Contract SHALL calculate creator's share
func (s *Service) CalculateSettlement(ctx context.Context, creatorID uuid.UUID, periodStart, periodEnd time.Time) (*SettlementCalculation, error) {
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

	// Validate period
	if periodEnd.Before(periodStart) {
		return nil, ErrInvalidSettlementPeriod
	}

	// Get earnings breakdown by agent for the period
	rows, err := s.db.Query(ctx, `
		SELECT 
			a.id,
			a.name,
			COUNT(cl.id) as total_calls,
			COALESCE(SUM(cl.cost_usd), 0) as revenue
		FROM agents a
		LEFT JOIN call_logs cl ON cl.agent_id = a.id 
			AND cl.created_at >= $2 
			AND cl.created_at < $3
			AND cl.status = 'success'
		WHERE a.creator_id = $1
		GROUP BY a.id, a.name
		HAVING COUNT(cl.id) > 0
		ORDER BY revenue DESC
	`, creatorID, periodStart, periodEnd)
	if err != nil {
		return nil, fmt.Errorf("failed to query agent earnings: %w", err)
	}
	defer rows.Close()

	var agentBreakdown []AgentEarnings
	var totalRevenue decimal.Decimal
	var totalCalls int64

	for rows.Next() {
		var ae AgentEarnings
		var revenue decimal.Decimal
		err := rows.Scan(&ae.AgentID, &ae.AgentName, &ae.TotalCalls, &revenue)
		if err != nil {
			return nil, fmt.Errorf("failed to scan agent earnings: %w", err)
		}
		ae.Revenue = revenue
		agentBreakdown = append(agentBreakdown, ae)
		totalRevenue = totalRevenue.Add(revenue)
		totalCalls += ae.TotalCalls
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating agent earnings: %w", err)
	}

	// Calculate fees
	platformFee := s.CalculatePlatformFee(totalRevenue)
	netAmount := s.CalculateNetAmount(totalRevenue)

	return &SettlementCalculation{
		CreatorID:      creatorID,
		GrossAmount:    totalRevenue,
		PlatformFee:    platformFee,
		NetAmount:      netAmount,
		TotalCalls:     totalCalls,
		AgentBreakdown: agentBreakdown,
		PeriodStart:    periodStart,
		PeriodEnd:      periodEnd,
	}, nil
}


// CreateSettlement creates a settlement record for a creator
func (s *Service) CreateSettlement(ctx context.Context, creatorID uuid.UUID, periodStart, periodEnd time.Time) (*models.Settlement, error) {
	// Calculate settlement
	calc, err := s.CalculateSettlement(ctx, creatorID, periodStart, periodEnd)
	if err != nil {
		return nil, err
	}

	// Check minimum amount
	if calc.GrossAmount.LessThan(s.config.MinSettlementAmount) {
		return nil, ErrNoEarningsToSettle
	}

	// Start transaction
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Check for existing settlement in this period
	var existingCount int
	err = tx.QueryRow(ctx, `
		SELECT COUNT(*) FROM settlements 
		WHERE creator_id = $1 
		AND created_at >= $2 
		AND created_at < $3
		AND status != 'failed'
	`, creatorID, periodStart, periodEnd).Scan(&existingCount)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing settlements: %w", err)
	}
	if existingCount > 0 {
		return nil, ErrSettlementInProgress
	}

	// Create settlement record
	settlementID := uuid.New()
	_, err = tx.Exec(ctx, `
		INSERT INTO settlements (id, creator_id, amount, platform_fee, net_amount, status)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, settlementID, creatorID, calc.GrossAmount, calc.PlatformFee, calc.NetAmount, models.SettlementStatusPending)
	if err != nil {
		return nil, fmt.Errorf("failed to create settlement record: %w", err)
	}

	// Update creator's pending earnings (add to pending_earnings for later withdrawal)
	_, err = tx.Exec(ctx, `
		UPDATE creator_profiles 
		SET pending_earnings = pending_earnings + $1,
		    total_earnings = total_earnings + $1
		WHERE user_id = $2
	`, calc.NetAmount, creatorID)
	if err != nil {
		return nil, fmt.Errorf("failed to update creator earnings: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Fetch and return the created settlement
	return s.GetSettlementByID(ctx, settlementID)
}

// GetSettlementByID retrieves a settlement by ID
func (s *Service) GetSettlementByID(ctx context.Context, settlementID uuid.UUID) (*models.Settlement, error) {
	var settlement models.Settlement
	err := s.db.QueryRow(ctx, `
		SELECT id, creator_id, amount, platform_fee, net_amount, tx_hash, status, created_at, settled_at
		FROM settlements WHERE id = $1
	`, settlementID).Scan(
		&settlement.ID, &settlement.CreatorID, &settlement.Amount, &settlement.PlatformFee,
		&settlement.NetAmount, &settlement.TxHash, &settlement.Status, &settlement.CreatedAt, &settlement.SettledAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSettlementNotFound
		}
		return nil, fmt.Errorf("failed to get settlement: %w", err)
	}
	return &settlement, nil
}

// GetSettlementHistory retrieves settlement history for a creator
func (s *Service) GetSettlementHistory(ctx context.Context, creatorID uuid.UUID, page, pageSize int) (*SettlementHistoryResponse, error) {
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
		SELECT COUNT(*) FROM settlements WHERE creator_id = $1
	`, creatorID).Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("failed to count settlements: %w", err)
	}

	// Get settlements
	rows, err := s.db.Query(ctx, `
		SELECT id, creator_id, amount, platform_fee, net_amount, tx_hash, status, created_at, settled_at
		FROM settlements
		WHERE creator_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, creatorID, pageSize, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query settlements: %w", err)
	}
	defer rows.Close()

	var settlements []models.Settlement
	for rows.Next() {
		var s models.Settlement
		err := rows.Scan(
			&s.ID, &s.CreatorID, &s.Amount, &s.PlatformFee,
			&s.NetAmount, &s.TxHash, &s.Status, &s.CreatedAt, &s.SettledAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan settlement: %w", err)
		}
		settlements = append(settlements, s)
	}

	return &SettlementHistoryResponse{
		Settlements: settlements,
		Total:       total,
		Page:        page,
		PageSize:    pageSize,
		TotalPages:  (total + pageSize - 1) / pageSize,
	}, nil
}


// CompleteSettlement marks a settlement as completed with blockchain transaction hash
func (s *Service) CompleteSettlement(ctx context.Context, settlementID uuid.UUID, txHash string) error {
	now := time.Now()
	result, err := s.db.Exec(ctx, `
		UPDATE settlements 
		SET status = $1, tx_hash = $2, settled_at = $3
		WHERE id = $4 AND status = $5
	`, models.SettlementStatusCompleted, txHash, now, settlementID, models.SettlementStatusPending)
	if err != nil {
		return fmt.Errorf("failed to complete settlement: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrSettlementAlreadyDone
	}
	return nil
}

// FailSettlement marks a settlement as failed
func (s *Service) FailSettlement(ctx context.Context, settlementID uuid.UUID) error {
	// Start transaction to restore earnings if needed
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Get settlement details
	var creatorID uuid.UUID
	var netAmount decimal.Decimal
	var status models.SettlementStatus
	err = tx.QueryRow(ctx, `
		SELECT creator_id, net_amount, status FROM settlements WHERE id = $1 FOR UPDATE
	`, settlementID).Scan(&creatorID, &netAmount, &status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrSettlementNotFound
		}
		return fmt.Errorf("failed to get settlement: %w", err)
	}

	if status != models.SettlementStatusPending {
		return ErrSettlementAlreadyDone
	}

	// Update settlement status
	_, err = tx.Exec(ctx, `
		UPDATE settlements SET status = $1 WHERE id = $2
	`, models.SettlementStatusFailed, settlementID)
	if err != nil {
		return fmt.Errorf("failed to update settlement status: %w", err)
	}

	// Note: We don't reverse the pending_earnings here because the earnings
	// are still valid - they just weren't transferred yet. The creator can
	// still withdraw them or they'll be included in the next settlement.

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetPendingSettlements retrieves all pending settlements (for processing)
func (s *Service) GetPendingSettlements(ctx context.Context, page, pageSize int) (*SettlementHistoryResponse, error) {
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
		SELECT COUNT(*) FROM settlements WHERE status = $1
	`, models.SettlementStatusPending).Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("failed to count pending settlements: %w", err)
	}

	// Get settlements
	rows, err := s.db.Query(ctx, `
		SELECT id, creator_id, amount, platform_fee, net_amount, tx_hash, status, created_at, settled_at
		FROM settlements
		WHERE status = $1
		ORDER BY created_at ASC
		LIMIT $2 OFFSET $3
	`, models.SettlementStatusPending, pageSize, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query pending settlements: %w", err)
	}
	defer rows.Close()

	var settlements []models.Settlement
	for rows.Next() {
		var s models.Settlement
		err := rows.Scan(
			&s.ID, &s.CreatorID, &s.Amount, &s.PlatformFee,
			&s.NetAmount, &s.TxHash, &s.Status, &s.CreatedAt, &s.SettledAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan settlement: %w", err)
		}
		settlements = append(settlements, s)
	}

	return &SettlementHistoryResponse{
		Settlements: settlements,
		Total:       total,
		Page:        page,
		PageSize:    pageSize,
		TotalPages:  (total + pageSize - 1) / pageSize,
	}, nil
}


// GetSettlementSummary retrieves a summary of all settlements for a creator
func (s *Service) GetSettlementSummary(ctx context.Context, creatorID uuid.UUID) (*SettlementSummary, error) {
	var summary SettlementSummary

	err := s.db.QueryRow(ctx, `
		SELECT 
			COUNT(*) as total_settlements,
			COALESCE(SUM(amount), 0) as total_gross_amount,
			COALESCE(SUM(platform_fee), 0) as total_platform_fees,
			COALESCE(SUM(net_amount), 0) as total_net_amount,
			COUNT(*) FILTER (WHERE status = 'pending') as pending_settlements,
			COUNT(*) FILTER (WHERE status = 'completed') as completed_settlements,
			COUNT(*) FILTER (WHERE status = 'failed') as failed_settlements
		FROM settlements
		WHERE creator_id = $1
	`, creatorID).Scan(
		&summary.TotalSettlements,
		&summary.TotalGrossAmount,
		&summary.TotalPlatformFees,
		&summary.TotalNetAmount,
		&summary.PendingSettlements,
		&summary.CompletedSettlements,
		&summary.FailedSettlements,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get settlement summary: %w", err)
	}

	return &summary, nil
}

// GetCreatorsForSettlement retrieves all creators who have earnings to settle
func (s *Service) GetCreatorsForSettlement(ctx context.Context, periodStart, periodEnd time.Time) ([]uuid.UUID, error) {
	rows, err := s.db.Query(ctx, `
		SELECT DISTINCT a.creator_id
		FROM agents a
		INNER JOIN call_logs cl ON cl.agent_id = a.id
		WHERE cl.created_at >= $1 
		AND cl.created_at < $2
		AND cl.status = 'success'
		AND cl.cost_usd > 0
	`, periodStart, periodEnd)
	if err != nil {
		return nil, fmt.Errorf("failed to query creators for settlement: %w", err)
	}
	defer rows.Close()

	var creatorIDs []uuid.UUID
	for rows.Next() {
		var creatorID uuid.UUID
		if err := rows.Scan(&creatorID); err != nil {
			return nil, fmt.Errorf("failed to scan creator ID: %w", err)
		}
		creatorIDs = append(creatorIDs, creatorID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating creators: %w", err)
	}

	return creatorIDs, nil
}

// ProcessSettlementBatch processes settlements for all eligible creators in a period
// This is the main entry point for the scheduled settlement task
func (s *Service) ProcessSettlementBatch(ctx context.Context, periodStart, periodEnd time.Time) (*BatchSettlementResult, error) {
	// Get all creators with earnings in this period
	creatorIDs, err := s.GetCreatorsForSettlement(ctx, periodStart, periodEnd)
	if err != nil {
		return nil, err
	}

	result := &BatchSettlementResult{
		PeriodStart:    periodStart,
		PeriodEnd:      periodEnd,
		TotalCreators:  len(creatorIDs),
		Settlements:    make([]SettlementResult, 0, len(creatorIDs)),
	}

	for _, creatorID := range creatorIDs {
		settlementResult := SettlementResult{
			CreatorID: creatorID,
		}

		settlement, err := s.CreateSettlement(ctx, creatorID, periodStart, periodEnd)
		if err != nil {
			settlementResult.Error = err.Error()
			result.FailedCount++
		} else {
			settlementResult.SettlementID = &settlement.ID
			settlementResult.Amount = settlement.NetAmount
			result.SuccessCount++
			result.TotalAmount = result.TotalAmount.Add(settlement.NetAmount)
		}

		result.Settlements = append(result.Settlements, settlementResult)
	}

	return result, nil
}

// BatchSettlementResult represents the result of a batch settlement process
type BatchSettlementResult struct {
	PeriodStart   time.Time          `json:"period_start"`
	PeriodEnd     time.Time          `json:"period_end"`
	TotalCreators int                `json:"total_creators"`
	SuccessCount  int                `json:"success_count"`
	FailedCount   int                `json:"failed_count"`
	TotalAmount   decimal.Decimal    `json:"total_amount"`
	Settlements   []SettlementResult `json:"settlements"`
}

// SettlementResult represents the result of a single settlement
type SettlementResult struct {
	CreatorID    uuid.UUID        `json:"creator_id"`
	SettlementID *uuid.UUID       `json:"settlement_id,omitempty"`
	Amount       decimal.Decimal  `json:"amount"`
	Error        string           `json:"error,omitempty"`
}

// GetConfig returns the settlement configuration
func (s *Service) GetConfig() *SettlementConfig {
	return s.config
}

// GetCurrentSettlementPeriod returns the current settlement period based on configuration
func (s *Service) GetCurrentSettlementPeriod() (time.Time, time.Time) {
	now := time.Now().UTC()
	
	// Calculate the start of the current period
	// Periods start on Sunday at 00:00 UTC
	daysFromSunday := int(now.Weekday())
	periodStart := now.AddDate(0, 0, -daysFromSunday).Truncate(24 * time.Hour)
	
	// If we're past the settlement period, move to the next period
	periodEnd := periodStart.AddDate(0, 0, s.config.SettlementPeriodDays)
	
	return periodStart, periodEnd
}

// GetPreviousSettlementPeriod returns the previous settlement period
func (s *Service) GetPreviousSettlementPeriod() (time.Time, time.Time) {
	currentStart, _ := s.GetCurrentSettlementPeriod()
	
	// Previous period ends at current period start
	periodEnd := currentStart
	periodStart := periodEnd.AddDate(0, 0, -s.config.SettlementPeriodDays)
	
	return periodStart, periodEnd
}
