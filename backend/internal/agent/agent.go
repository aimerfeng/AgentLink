package agent

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/aimerfeng/AgentLink/internal/config"
	"github.com/aimerfeng/AgentLink/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// Service errors
var (
	ErrAgentNotFound     = errors.New("agent not found")
	ErrAgentNotOwned     = errors.New("agent not owned by user")
	ErrInvalidPrice      = errors.New("invalid price: must be between $0.001 and $100")
	ErrInvalidConfig     = errors.New("invalid agent configuration")
	ErrEncryptionFailed  = errors.New("encryption failed")
	ErrDecryptionFailed  = errors.New("decryption failed")
	ErrAgentDraft        = errors.New("agent is in draft status")
	ErrAgentNotDraft     = errors.New("agent is not in draft status")
	ErrAgentAlreadyActive = errors.New("agent is already active")
)

// Price validation constants
var (
	MinPricePerCall = decimal.NewFromFloat(0.001) // $0.001 minimum
	MaxPricePerCall = decimal.NewFromFloat(100.0) // $100 maximum
)

// Service handles agent operations
type Service struct {
	db            *pgxpool.Pool
	encryptionKey []byte
}

// NewService creates a new agent service
func NewService(db *pgxpool.Pool, encCfg *config.EncryptionConfig) (*Service, error) {
	// Parse encryption key from hex string
	key, err := hex.DecodeString(encCfg.Key)
	if err != nil {
		// If not hex, use raw bytes (for development)
		key = []byte(encCfg.Key)
	}

	// Ensure key is 32 bytes for AES-256
	if len(key) < 32 {
		// Pad key if too short (for development only)
		padded := make([]byte, 32)
		copy(padded, key)
		key = padded
	} else if len(key) > 32 {
		key = key[:32]
	}

	return &Service{
		db:            db,
		encryptionKey: key,
	}, nil
}


// CreateAgentRequest represents a request to create an agent
type CreateAgentRequest struct {
	Name         string                `json:"name" binding:"required,min=1,max=100"`
	Description  *string               `json:"description,omitempty"`
	Category     *string               `json:"category,omitempty"`
	Config       models.AgentConfig    `json:"config" binding:"required"`
	PricePerCall decimal.Decimal       `json:"price_per_call" binding:"required"`
}

// UpdateAgentRequest represents a request to update an agent
type UpdateAgentRequest struct {
	Name         *string               `json:"name,omitempty"`
	Description  *string               `json:"description,omitempty"`
	Category     *string               `json:"category,omitempty"`
	Config       *models.AgentConfig   `json:"config,omitempty"`
	PricePerCall *decimal.Decimal      `json:"price_per_call,omitempty"`
}

// AgentResponse represents an agent response (with decrypted config for owner)
type AgentResponse struct {
	ID            uuid.UUID           `json:"id"`
	CreatorID     uuid.UUID           `json:"creator_id"`
	Name          string              `json:"name"`
	Description   *string             `json:"description,omitempty"`
	Category      *string             `json:"category,omitempty"`
	Status        models.AgentStatus  `json:"status"`
	Config        *models.AgentConfig `json:"config,omitempty"` // Only included for owner
	PricePerCall  decimal.Decimal     `json:"price_per_call"`
	TotalCalls    int64               `json:"total_calls"`
	TotalRevenue  decimal.Decimal     `json:"total_revenue"`
	AverageRating decimal.Decimal     `json:"average_rating"`
	ReviewCount   int                 `json:"review_count"`
	TokenID       *int64              `json:"token_id,omitempty"`
	TokenTxHash   *string             `json:"token_tx_hash,omitempty"`
	Version       int                 `json:"version"`
	CreatedAt     time.Time           `json:"created_at"`
	UpdatedAt     time.Time           `json:"updated_at"`
	PublishedAt   *time.Time          `json:"published_at,omitempty"`
}

// ListAgentsResponse represents a paginated list of agents
type ListAgentsResponse struct {
	Agents     []AgentResponse `json:"agents"`
	Total      int64           `json:"total"`
	Page       int             `json:"page"`
	PageSize   int             `json:"page_size"`
	TotalPages int             `json:"total_pages"`
}

// ValidatePrice validates that the price is within acceptable range
func ValidatePrice(price decimal.Decimal) error {
	if price.LessThan(MinPricePerCall) || price.GreaterThan(MaxPricePerCall) {
		return ErrInvalidPrice
	}
	return nil
}

// ValidateConfig validates the agent configuration
func ValidateConfig(cfg *models.AgentConfig) error {
	if cfg.SystemPrompt == "" {
		return fmt.Errorf("%w: system_prompt is required", ErrInvalidConfig)
	}
	if cfg.Model == "" {
		return fmt.Errorf("%w: model is required", ErrInvalidConfig)
	}
	if cfg.Provider == "" {
		return fmt.Errorf("%w: provider is required", ErrInvalidConfig)
	}
	if cfg.Temperature < 0 || cfg.Temperature > 2 {
		return fmt.Errorf("%w: temperature must be between 0 and 2", ErrInvalidConfig)
	}
	if cfg.MaxTokens < 1 || cfg.MaxTokens > 128000 {
		return fmt.Errorf("%w: max_tokens must be between 1 and 128000", ErrInvalidConfig)
	}
	if cfg.TopP < 0 || cfg.TopP > 1 {
		return fmt.Errorf("%w: top_p must be between 0 and 1", ErrInvalidConfig)
	}
	return nil
}


// Encrypt encrypts data using AES-256-GCM
func (s *Service) Encrypt(plaintext []byte) (ciphertext, nonce []byte, err error) {
	block, err := aes.NewCipher(s.encryptionKey)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrEncryptionFailed, err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrEncryptionFailed, err)
	}

	nonce = make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrEncryptionFailed, err)
	}

	ciphertext = gcm.Seal(nil, nonce, plaintext, nil)
	return ciphertext, nonce, nil
}

// Decrypt decrypts data using AES-256-GCM
func (s *Service) Decrypt(ciphertext, nonce []byte) ([]byte, error) {
	block, err := aes.NewCipher(s.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDecryptionFailed, err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDecryptionFailed, err)
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDecryptionFailed, err)
	}

	return plaintext, nil
}

// encryptConfig encrypts an agent configuration
func (s *Service) encryptConfig(cfg *models.AgentConfig) ([]byte, []byte, error) {
	plaintext, err := json.Marshal(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal config: %w", err)
	}
	return s.Encrypt(plaintext)
}

// decryptConfig decrypts an agent configuration
func (s *Service) decryptConfig(ciphertext, nonce []byte) (*models.AgentConfig, error) {
	plaintext, err := s.Decrypt(ciphertext, nonce)
	if err != nil {
		return nil, err
	}

	var cfg models.AgentConfig
	if err := json.Unmarshal(plaintext, &cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}


// Create creates a new agent
func (s *Service) Create(ctx context.Context, creatorID uuid.UUID, req *CreateAgentRequest) (*AgentResponse, error) {
	// Validate price
	if err := ValidatePrice(req.PricePerCall); err != nil {
		return nil, err
	}

	// Validate config
	if err := ValidateConfig(&req.Config); err != nil {
		return nil, err
	}

	// Encrypt config
	configEncrypted, configIV, err := s.encryptConfig(&req.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt config: %w", err)
	}

	// Create agent in database
	var agent models.Agent
	err = s.db.QueryRow(ctx, `
		INSERT INTO agents (
			creator_id, name, description, category, status,
			config_encrypted, config_iv, price_per_call, version
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 1)
		RETURNING id, creator_id, name, description, category, status,
			config_encrypted, config_iv, price_per_call, total_calls,
			total_revenue, average_rating, review_count, token_id,
			token_tx_hash, version, created_at, updated_at, published_at
	`, creatorID, req.Name, req.Description, req.Category, models.AgentStatusDraft,
		configEncrypted, configIV, req.PricePerCall,
	).Scan(
		&agent.ID, &agent.CreatorID, &agent.Name, &agent.Description,
		&agent.Category, &agent.Status, &agent.ConfigEncrypted, &agent.ConfigIV,
		&agent.PricePerCall, &agent.TotalCalls, &agent.TotalRevenue,
		&agent.AverageRating, &agent.ReviewCount, &agent.TokenID,
		&agent.TokenTxHash, &agent.Version, &agent.CreatedAt, &agent.UpdatedAt,
		&agent.PublishedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create agent: %w", err)
	}

	return s.toAgentResponse(&agent, &req.Config), nil
}

// GetByID retrieves an agent by ID
func (s *Service) GetByID(ctx context.Context, agentID uuid.UUID) (*models.Agent, error) {
	var agent models.Agent
	err := s.db.QueryRow(ctx, `
		SELECT id, creator_id, name, description, category, status,
			config_encrypted, config_iv, price_per_call, total_calls,
			total_revenue, average_rating, review_count, token_id,
			token_tx_hash, version, created_at, updated_at, published_at
		FROM agents WHERE id = $1
	`, agentID).Scan(
		&agent.ID, &agent.CreatorID, &agent.Name, &agent.Description,
		&agent.Category, &agent.Status, &agent.ConfigEncrypted, &agent.ConfigIV,
		&agent.PricePerCall, &agent.TotalCalls, &agent.TotalRevenue,
		&agent.AverageRating, &agent.ReviewCount, &agent.TokenID,
		&agent.TokenTxHash, &agent.Version, &agent.CreatedAt, &agent.UpdatedAt,
		&agent.PublishedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrAgentNotFound
		}
		return nil, fmt.Errorf("failed to get agent: %w", err)
	}
	return &agent, nil
}

// GetByIDForOwner retrieves an agent by ID with decrypted config (for owner only)
func (s *Service) GetByIDForOwner(ctx context.Context, agentID, creatorID uuid.UUID) (*AgentResponse, error) {
	agent, err := s.GetByID(ctx, agentID)
	if err != nil {
		return nil, err
	}

	// Verify ownership
	if agent.CreatorID != creatorID {
		return nil, ErrAgentNotOwned
	}

	// Decrypt config
	cfg, err := s.decryptConfig(agent.ConfigEncrypted, agent.ConfigIV)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt config: %w", err)
	}

	return s.toAgentResponse(agent, cfg), nil
}


// List retrieves agents for a creator with pagination
func (s *Service) List(ctx context.Context, creatorID uuid.UUID, page, pageSize int) (*ListAgentsResponse, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	// Get total count
	var total int64
	err := s.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM agents WHERE creator_id = $1
	`, creatorID).Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("failed to count agents: %w", err)
	}

	// Get agents
	rows, err := s.db.Query(ctx, `
		SELECT id, creator_id, name, description, category, status,
			config_encrypted, config_iv, price_per_call, total_calls,
			total_revenue, average_rating, review_count, token_id,
			token_tx_hash, version, created_at, updated_at, published_at
		FROM agents
		WHERE creator_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, creatorID, pageSize, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list agents: %w", err)
	}
	defer rows.Close()

	var agents []AgentResponse
	for rows.Next() {
		var agent models.Agent
		err := rows.Scan(
			&agent.ID, &agent.CreatorID, &agent.Name, &agent.Description,
			&agent.Category, &agent.Status, &agent.ConfigEncrypted, &agent.ConfigIV,
			&agent.PricePerCall, &agent.TotalCalls, &agent.TotalRevenue,
			&agent.AverageRating, &agent.ReviewCount, &agent.TokenID,
			&agent.TokenTxHash, &agent.Version, &agent.CreatedAt, &agent.UpdatedAt,
			&agent.PublishedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan agent: %w", err)
		}

		// Decrypt config for owner
		cfg, err := s.decryptConfig(agent.ConfigEncrypted, agent.ConfigIV)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt config: %w", err)
		}

		agents = append(agents, *s.toAgentResponse(&agent, cfg))
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate agents: %w", err)
	}

	totalPages := int(total) / pageSize
	if int(total)%pageSize > 0 {
		totalPages++
	}

	return &ListAgentsResponse{
		Agents:     agents,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}


// Update updates an agent
func (s *Service) Update(ctx context.Context, agentID, creatorID uuid.UUID, req *UpdateAgentRequest) (*AgentResponse, error) {
	// Get existing agent
	agent, err := s.GetByID(ctx, agentID)
	if err != nil {
		return nil, err
	}

	// Verify ownership
	if agent.CreatorID != creatorID {
		return nil, ErrAgentNotOwned
	}

	// Get current config
	currentConfig, err := s.decryptConfig(agent.ConfigEncrypted, agent.ConfigIV)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt current config: %w", err)
	}

	// Start transaction
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Save current version to history
	_, err = tx.Exec(ctx, `
		INSERT INTO agent_versions (agent_id, version, config_encrypted, config_iv)
		VALUES ($1, $2, $3, $4)
	`, agentID, agent.Version, agent.ConfigEncrypted, agent.ConfigIV)
	if err != nil {
		return nil, fmt.Errorf("failed to save version history: %w", err)
	}

	// Apply updates
	if req.Name != nil {
		agent.Name = *req.Name
	}
	if req.Description != nil {
		agent.Description = req.Description
	}
	if req.Category != nil {
		agent.Category = req.Category
	}
	if req.PricePerCall != nil {
		if err := ValidatePrice(*req.PricePerCall); err != nil {
			return nil, err
		}
		agent.PricePerCall = *req.PricePerCall
	}
	if req.Config != nil {
		if err := ValidateConfig(req.Config); err != nil {
			return nil, err
		}
		currentConfig = req.Config
	}

	// Encrypt updated config
	configEncrypted, configIV, err := s.encryptConfig(currentConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt config: %w", err)
	}

	// Update agent
	newVersion := agent.Version + 1
	err = tx.QueryRow(ctx, `
		UPDATE agents SET
			name = $1, description = $2, category = $3,
			config_encrypted = $4, config_iv = $5, price_per_call = $6,
			version = $7, updated_at = NOW()
		WHERE id = $8
		RETURNING id, creator_id, name, description, category, status,
			config_encrypted, config_iv, price_per_call, total_calls,
			total_revenue, average_rating, review_count, token_id,
			token_tx_hash, version, created_at, updated_at, published_at
	`, agent.Name, agent.Description, agent.Category,
		configEncrypted, configIV, agent.PricePerCall, newVersion, agentID,
	).Scan(
		&agent.ID, &agent.CreatorID, &agent.Name, &agent.Description,
		&agent.Category, &agent.Status, &agent.ConfigEncrypted, &agent.ConfigIV,
		&agent.PricePerCall, &agent.TotalCalls, &agent.TotalRevenue,
		&agent.AverageRating, &agent.ReviewCount, &agent.TokenID,
		&agent.TokenTxHash, &agent.Version, &agent.CreatedAt, &agent.UpdatedAt,
		&agent.PublishedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update agent: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return s.toAgentResponse(agent, currentConfig), nil
}


// Publish publishes an agent (changes status to active)
func (s *Service) Publish(ctx context.Context, agentID, creatorID uuid.UUID) (*AgentResponse, error) {
	agent, err := s.GetByID(ctx, agentID)
	if err != nil {
		return nil, err
	}

	// Verify ownership
	if agent.CreatorID != creatorID {
		return nil, ErrAgentNotOwned
	}

	// Check current status
	if agent.Status == models.AgentStatusActive {
		return nil, ErrAgentAlreadyActive
	}

	// Update status
	now := time.Now()
	err = s.db.QueryRow(ctx, `
		UPDATE agents SET
			status = $1, published_at = $2, updated_at = NOW()
		WHERE id = $3
		RETURNING id, creator_id, name, description, category, status,
			config_encrypted, config_iv, price_per_call, total_calls,
			total_revenue, average_rating, review_count, token_id,
			token_tx_hash, version, created_at, updated_at, published_at
	`, models.AgentStatusActive, now, agentID).Scan(
		&agent.ID, &agent.CreatorID, &agent.Name, &agent.Description,
		&agent.Category, &agent.Status, &agent.ConfigEncrypted, &agent.ConfigIV,
		&agent.PricePerCall, &agent.TotalCalls, &agent.TotalRevenue,
		&agent.AverageRating, &agent.ReviewCount, &agent.TokenID,
		&agent.TokenTxHash, &agent.Version, &agent.CreatedAt, &agent.UpdatedAt,
		&agent.PublishedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to publish agent: %w", err)
	}

	// Decrypt config for response
	cfg, err := s.decryptConfig(agent.ConfigEncrypted, agent.ConfigIV)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt config: %w", err)
	}

	return s.toAgentResponse(agent, cfg), nil
}

// Unpublish unpublishes an agent (changes status to inactive)
func (s *Service) Unpublish(ctx context.Context, agentID, creatorID uuid.UUID) (*AgentResponse, error) {
	agent, err := s.GetByID(ctx, agentID)
	if err != nil {
		return nil, err
	}

	// Verify ownership
	if agent.CreatorID != creatorID {
		return nil, ErrAgentNotOwned
	}

	// Update status
	err = s.db.QueryRow(ctx, `
		UPDATE agents SET
			status = $1, updated_at = NOW()
		WHERE id = $2
		RETURNING id, creator_id, name, description, category, status,
			config_encrypted, config_iv, price_per_call, total_calls,
			total_revenue, average_rating, review_count, token_id,
			token_tx_hash, version, created_at, updated_at, published_at
	`, models.AgentStatusInactive, agentID).Scan(
		&agent.ID, &agent.CreatorID, &agent.Name, &agent.Description,
		&agent.Category, &agent.Status, &agent.ConfigEncrypted, &agent.ConfigIV,
		&agent.PricePerCall, &agent.TotalCalls, &agent.TotalRevenue,
		&agent.AverageRating, &agent.ReviewCount, &agent.TokenID,
		&agent.TokenTxHash, &agent.Version, &agent.CreatedAt, &agent.UpdatedAt,
		&agent.PublishedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to unpublish agent: %w", err)
	}

	// Decrypt config for response
	cfg, err := s.decryptConfig(agent.ConfigEncrypted, agent.ConfigIV)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt config: %w", err)
	}

	return s.toAgentResponse(agent, cfg), nil
}

// GetConfig retrieves the decrypted config for an agent (internal use)
func (s *Service) GetConfig(ctx context.Context, agentID uuid.UUID) (*models.AgentConfig, error) {
	agent, err := s.GetByID(ctx, agentID)
	if err != nil {
		return nil, err
	}

	return s.decryptConfig(agent.ConfigEncrypted, agent.ConfigIV)
}

// toAgentResponse converts an Agent model to AgentResponse
func (s *Service) toAgentResponse(agent *models.Agent, cfg *models.AgentConfig) *AgentResponse {
	return &AgentResponse{
		ID:            agent.ID,
		CreatorID:     agent.CreatorID,
		Name:          agent.Name,
		Description:   agent.Description,
		Category:      agent.Category,
		Status:        agent.Status,
		Config:        cfg,
		PricePerCall:  agent.PricePerCall,
		TotalCalls:    agent.TotalCalls,
		TotalRevenue:  agent.TotalRevenue,
		AverageRating: agent.AverageRating,
		ReviewCount:   agent.ReviewCount,
		TokenID:       agent.TokenID,
		TokenTxHash:   agent.TokenTxHash,
		Version:       agent.Version,
		CreatedAt:     agent.CreatedAt,
		UpdatedAt:     agent.UpdatedAt,
		PublishedAt:   agent.PublishedAt,
	}
}
