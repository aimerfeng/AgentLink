package trial

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aimerfeng/AgentLink/internal/config"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Service errors
var (
	ErrTrialExhausted     = errors.New("trial quota exhausted for this agent")
	ErrTrialDisabled      = errors.New("trial is disabled for this agent")
	ErrAgentNotFound      = errors.New("agent not found")
	ErrUserNotFound       = errors.New("user not found")
	ErrInvalidTrialConfig = errors.New("invalid trial configuration")
)

// TrialUsage represents trial usage for a user-agent pair
type TrialUsage struct {
	ID         uuid.UUID `json:"id" db:"id"`
	UserID     uuid.UUID `json:"user_id" db:"user_id"`
	AgentID    uuid.UUID `json:"agent_id" db:"agent_id"`
	UsedTrials int       `json:"used_trials" db:"used_trials"`
	MaxTrials  int       `json:"max_trials" db:"max_trials"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
}

// TrialInfo represents trial information for a user-agent pair
type TrialInfo struct {
	AgentID        uuid.UUID `json:"agent_id"`
	UsedTrials     int       `json:"used_trials"`
	MaxTrials      int       `json:"max_trials"`
	RemainingTrials int      `json:"remaining_trials"`
	TrialEnabled   bool      `json:"trial_enabled"`
	IsExhausted    bool      `json:"is_exhausted"`
}

// Service handles trial quota operations
type Service struct {
	db              *pgxpool.Pool
	trialCallsPerAgent int
}

// NewService creates a new trial service
func NewService(db *pgxpool.Pool, quotaCfg *config.QuotaConfig) *Service {
	return &Service{
		db:              db,
		trialCallsPerAgent: quotaCfg.TrialCallsPerAgent,
	}
}


// GetTrialInfo retrieves trial information for a user-agent pair
// D5.2: WHEN a developer uses trial calls, THE System SHALL clearly indicate remaining trial quota
func (s *Service) GetTrialInfo(ctx context.Context, userID, agentID uuid.UUID) (*TrialInfo, error) {
	// First check if trial is enabled for this agent
	var trialEnabled bool
	err := s.db.QueryRow(ctx, `
		SELECT COALESCE(trial_enabled, true) FROM agents WHERE id = $1
	`, agentID).Scan(&trialEnabled)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrAgentNotFound
		}
		return nil, fmt.Errorf("failed to check agent trial status: %w", err)
	}

	// Get or create trial usage record
	var usage TrialUsage
	err = s.db.QueryRow(ctx, `
		SELECT id, user_id, agent_id, used_trials, max_trials, created_at, updated_at
		FROM trial_usage
		WHERE user_id = $1 AND agent_id = $2
	`, userID, agentID).Scan(
		&usage.ID, &usage.UserID, &usage.AgentID,
		&usage.UsedTrials, &usage.MaxTrials,
		&usage.CreatedAt, &usage.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// No usage record exists, return default values
			return &TrialInfo{
				AgentID:         agentID,
				UsedTrials:      0,
				MaxTrials:       s.trialCallsPerAgent,
				RemainingTrials: s.trialCallsPerAgent,
				TrialEnabled:    trialEnabled,
				IsExhausted:     false,
			}, nil
		}
		return nil, fmt.Errorf("failed to get trial usage: %w", err)
	}

	remaining := usage.MaxTrials - usage.UsedTrials
	if remaining < 0 {
		remaining = 0
	}

	return &TrialInfo{
		AgentID:         agentID,
		UsedTrials:      usage.UsedTrials,
		MaxTrials:       usage.MaxTrials,
		RemainingTrials: remaining,
		TrialEnabled:    trialEnabled,
		IsExhausted:     remaining <= 0,
	}, nil
}

// CheckTrialAvailable checks if a user can use trial for an agent
// D5.1: THE System SHALL provide 3 free trial calls per Agent per developer account
func (s *Service) CheckTrialAvailable(ctx context.Context, userID, agentID uuid.UUID) (bool, int, error) {
	info, err := s.GetTrialInfo(ctx, userID, agentID)
	if err != nil {
		return false, 0, err
	}

	// Check if trial is disabled for this agent
	if !info.TrialEnabled {
		return false, 0, ErrTrialDisabled
	}

	// Check if trial is exhausted
	if info.IsExhausted {
		return false, 0, nil
	}

	return true, info.RemainingTrials, nil
}

// UseTrialCall consumes one trial call for a user-agent pair
// D5.1: THE System SHALL provide 3 free trial calls per Agent per developer account
func (s *Service) UseTrialCall(ctx context.Context, userID, agentID uuid.UUID) (*TrialInfo, error) {
	// First check if trial is available
	available, _, err := s.CheckTrialAvailable(ctx, userID, agentID)
	if err != nil {
		return nil, err
	}
	if !available {
		return nil, ErrTrialExhausted
	}

	// Use upsert to atomically increment trial usage
	var usage TrialUsage
	err = s.db.QueryRow(ctx, `
		INSERT INTO trial_usage (user_id, agent_id, used_trials, max_trials)
		VALUES ($1, $2, 1, $3)
		ON CONFLICT (user_id, agent_id) 
		DO UPDATE SET 
			used_trials = trial_usage.used_trials + 1,
			updated_at = NOW()
		WHERE trial_usage.used_trials < trial_usage.max_trials
		RETURNING id, user_id, agent_id, used_trials, max_trials, created_at, updated_at
	`, userID, agentID, s.trialCallsPerAgent).Scan(
		&usage.ID, &usage.UserID, &usage.AgentID,
		&usage.UsedTrials, &usage.MaxTrials,
		&usage.CreatedAt, &usage.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// The WHERE clause prevented the update because trials are exhausted
			return nil, ErrTrialExhausted
		}
		return nil, fmt.Errorf("failed to use trial call: %w", err)
	}

	remaining := usage.MaxTrials - usage.UsedTrials
	if remaining < 0 {
		remaining = 0
	}

	// Check if trial is enabled for this agent
	var trialEnabled bool
	err = s.db.QueryRow(ctx, `
		SELECT COALESCE(trial_enabled, true) FROM agents WHERE id = $1
	`, agentID).Scan(&trialEnabled)
	if err != nil {
		trialEnabled = true // Default to enabled if query fails
	}

	return &TrialInfo{
		AgentID:         agentID,
		UsedTrials:      usage.UsedTrials,
		MaxTrials:       usage.MaxTrials,
		RemainingTrials: remaining,
		TrialEnabled:    trialEnabled,
		IsExhausted:     remaining <= 0,
	}, nil
}


// GetUserTrialUsage retrieves all trial usage for a user
func (s *Service) GetUserTrialUsage(ctx context.Context, userID uuid.UUID) ([]TrialInfo, error) {
	rows, err := s.db.Query(ctx, `
		SELECT tu.agent_id, tu.used_trials, tu.max_trials, COALESCE(a.trial_enabled, true)
		FROM trial_usage tu
		JOIN agents a ON tu.agent_id = a.id
		WHERE tu.user_id = $1
		ORDER BY tu.updated_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user trial usage: %w", err)
	}
	defer rows.Close()

	var infos []TrialInfo
	for rows.Next() {
		var info TrialInfo
		err := rows.Scan(&info.AgentID, &info.UsedTrials, &info.MaxTrials, &info.TrialEnabled)
		if err != nil {
			return nil, fmt.Errorf("failed to scan trial usage: %w", err)
		}
		info.RemainingTrials = info.MaxTrials - info.UsedTrials
		if info.RemainingTrials < 0 {
			info.RemainingTrials = 0
		}
		info.IsExhausted = info.RemainingTrials <= 0
		infos = append(infos, info)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate trial usage: %w", err)
	}

	return infos, nil
}

// SetAgentTrialEnabled sets whether trial is enabled for an agent
// D5.4: THE Creator SHALL have option to disable trial for their Agents
func (s *Service) SetAgentTrialEnabled(ctx context.Context, agentID, creatorID uuid.UUID, enabled bool) error {
	// Verify ownership and update
	result, err := s.db.Exec(ctx, `
		UPDATE agents 
		SET trial_enabled = $1, updated_at = NOW()
		WHERE id = $2 AND creator_id = $3
	`, enabled, agentID, creatorID)
	if err != nil {
		return fmt.Errorf("failed to update agent trial status: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrAgentNotFound
	}

	return nil
}

// GetAgentTrialEnabled checks if trial is enabled for an agent
func (s *Service) GetAgentTrialEnabled(ctx context.Context, agentID uuid.UUID) (bool, error) {
	var enabled bool
	err := s.db.QueryRow(ctx, `
		SELECT COALESCE(trial_enabled, true) FROM agents WHERE id = $1
	`, agentID).Scan(&enabled)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, ErrAgentNotFound
		}
		return false, fmt.Errorf("failed to get agent trial status: %w", err)
	}
	return enabled, nil
}

// ResetTrialUsage resets trial usage for a user-agent pair (admin function)
func (s *Service) ResetTrialUsage(ctx context.Context, userID, agentID uuid.UUID) error {
	_, err := s.db.Exec(ctx, `
		DELETE FROM trial_usage WHERE user_id = $1 AND agent_id = $2
	`, userID, agentID)
	if err != nil {
		return fmt.Errorf("failed to reset trial usage: %w", err)
	}
	return nil
}

// GetTrialCallsPerAgent returns the configured number of trial calls per agent
func (s *Service) GetTrialCallsPerAgent() int {
	return s.trialCallsPerAgent
}

// TrialUsageResponse represents the response for trial usage queries
type TrialUsageResponse struct {
	AgentID         uuid.UUID `json:"agent_id"`
	AgentName       string    `json:"agent_name,omitempty"`
	UsedTrials      int       `json:"used_trials"`
	MaxTrials       int       `json:"max_trials"`
	RemainingTrials int       `json:"remaining_trials"`
	TrialEnabled    bool      `json:"trial_enabled"`
	IsExhausted     bool      `json:"is_exhausted"`
}

// GetUserTrialUsageWithAgentNames retrieves all trial usage for a user with agent names
func (s *Service) GetUserTrialUsageWithAgentNames(ctx context.Context, userID uuid.UUID) ([]TrialUsageResponse, error) {
	rows, err := s.db.Query(ctx, `
		SELECT tu.agent_id, a.name, tu.used_trials, tu.max_trials, COALESCE(a.trial_enabled, true)
		FROM trial_usage tu
		JOIN agents a ON tu.agent_id = a.id
		WHERE tu.user_id = $1
		ORDER BY tu.updated_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user trial usage: %w", err)
	}
	defer rows.Close()

	var responses []TrialUsageResponse
	for rows.Next() {
		var resp TrialUsageResponse
		err := rows.Scan(&resp.AgentID, &resp.AgentName, &resp.UsedTrials, &resp.MaxTrials, &resp.TrialEnabled)
		if err != nil {
			return nil, fmt.Errorf("failed to scan trial usage: %w", err)
		}
		resp.RemainingTrials = resp.MaxTrials - resp.UsedTrials
		if resp.RemainingTrials < 0 {
			resp.RemainingTrials = 0
		}
		resp.IsExhausted = resp.RemainingTrials <= 0
		responses = append(responses, resp)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate trial usage: %w", err)
	}

	return responses, nil
}
