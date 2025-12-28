package apikey

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/aimerfeng/AgentLink/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Service errors
var (
	ErrAPIKeyNotFound   = errors.New("API key not found")
	ErrAPIKeyRevoked    = errors.New("API key has been revoked")
	ErrAPIKeyNotOwned   = errors.New("API key does not belong to user")
	ErrInvalidAPIKey    = errors.New("invalid API key format")
	ErrMaxKeysReached   = errors.New("maximum number of API keys reached")
)

// MaxAPIKeysPerUser is the maximum number of API keys a user can have
const MaxAPIKeysPerUser = 10

// Service handles API key operations
type Service struct {
	db *pgxpool.Pool
}

// NewService creates a new API key service
func NewService(db *pgxpool.Pool) *Service {
	return &Service{db: db}
}

// CreateAPIKeyRequest represents a request to create an API key
type CreateAPIKeyRequest struct {
	Name        string          `json:"name"`
	Permissions map[string]bool `json:"permissions,omitempty"`
}

// CreateAPIKeyResponse represents the response when creating an API key
// The raw key is only returned once at creation time
type CreateAPIKeyResponse struct {
	ID          uuid.UUID       `json:"id"`
	Key         string          `json:"key"` // Only returned at creation
	KeyPrefix   string          `json:"key_prefix"`
	Name        *string         `json:"name,omitempty"`
	Permissions map[string]bool `json:"permissions"`
	CreatedAt   time.Time       `json:"created_at"`
}

// APIKeyResponse represents an API key in list/get responses (without the raw key)
type APIKeyResponse struct {
	ID          uuid.UUID       `json:"id"`
	KeyPrefix   string          `json:"key_prefix"`
	Name        *string         `json:"name,omitempty"`
	Permissions map[string]bool `json:"permissions"`
	LastUsedAt  *time.Time      `json:"last_used_at,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	RevokedAt   *time.Time      `json:"revoked_at,omitempty"`
}

// ListAPIKeysResponse represents the response for listing API keys
type ListAPIKeysResponse struct {
	Keys  []APIKeyResponse `json:"keys"`
	Total int              `json:"total"`
}

// Create creates a new API key for a user
func (s *Service) Create(ctx context.Context, userID uuid.UUID, req *CreateAPIKeyRequest) (*CreateAPIKeyResponse, error) {
	// Check if user has reached max keys
	var count int
	err := s.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM api_keys 
		WHERE user_id = $1 AND revoked_at IS NULL
	`, userID).Scan(&count)
	if err != nil {
		return nil, fmt.Errorf("failed to count API keys: %w", err)
	}
	if count >= MaxAPIKeysPerUser {
		return nil, ErrMaxKeysReached
	}

	// Generate secure API key
	rawKey, keyHash, keyPrefix, err := generateAPIKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate API key: %w", err)
	}

	// Set default permissions if not provided
	permissions := req.Permissions
	if permissions == nil {
		permissions = map[string]bool{
			"read":  true,
			"write": true,
		}
	}

	// Insert API key
	var apiKey models.APIKey
	var name *string
	if req.Name != "" {
		name = &req.Name
	}

	err = s.db.QueryRow(ctx, `
		INSERT INTO api_keys (user_id, key_hash, key_prefix, name, permissions)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, user_id, key_hash, key_prefix, name, permissions, last_used_at, created_at, revoked_at
	`, userID, keyHash, keyPrefix, name, permissions).Scan(
		&apiKey.ID, &apiKey.UserID, &apiKey.KeyHash, &apiKey.KeyPrefix,
		&apiKey.Name, &apiKey.Permissions, &apiKey.LastUsedAt, &apiKey.CreatedAt, &apiKey.RevokedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create API key: %w", err)
	}

	return &CreateAPIKeyResponse{
		ID:          apiKey.ID,
		Key:         rawKey,
		KeyPrefix:   apiKey.KeyPrefix,
		Name:        apiKey.Name,
		Permissions: apiKey.Permissions,
		CreatedAt:   apiKey.CreatedAt,
	}, nil
}

// List returns all API keys for a user
func (s *Service) List(ctx context.Context, userID uuid.UUID) (*ListAPIKeysResponse, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, key_prefix, name, permissions, last_used_at, created_at, revoked_at
		FROM api_keys
		WHERE user_id = $1
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list API keys: %w", err)
	}
	defer rows.Close()

	var keys []APIKeyResponse
	for rows.Next() {
		var key APIKeyResponse
		err := rows.Scan(
			&key.ID, &key.KeyPrefix, &key.Name, &key.Permissions,
			&key.LastUsedAt, &key.CreatedAt, &key.RevokedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan API key: %w", err)
		}
		keys = append(keys, key)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating API keys: %w", err)
	}

	return &ListAPIKeysResponse{
		Keys:  keys,
		Total: len(keys),
	}, nil
}

// Delete revokes an API key (soft delete)
func (s *Service) Delete(ctx context.Context, keyID uuid.UUID, userID uuid.UUID) error {
	// Check if key exists and belongs to user
	var ownerID uuid.UUID
	var revokedAt *time.Time
	err := s.db.QueryRow(ctx, `
		SELECT user_id, revoked_at FROM api_keys WHERE id = $1
	`, keyID).Scan(&ownerID, &revokedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrAPIKeyNotFound
		}
		return fmt.Errorf("failed to get API key: %w", err)
	}

	// Check ownership
	if ownerID != userID {
		return ErrAPIKeyNotOwned
	}

	// Check if already revoked
	if revokedAt != nil {
		return ErrAPIKeyRevoked
	}

	// Revoke the key
	_, err = s.db.Exec(ctx, `
		UPDATE api_keys 
		SET revoked_at = NOW()
		WHERE id = $1
	`, keyID)
	if err != nil {
		return fmt.Errorf("failed to revoke API key: %w", err)
	}

	return nil
}

// ValidateAPIKey validates an API key and returns the associated user ID
// Returns ErrInvalidAPIKey if the key is invalid or revoked
func (s *Service) ValidateAPIKey(ctx context.Context, rawKey string) (*models.APIKey, error) {
	// Validate key format
	if len(rawKey) < 10 || rawKey[:3] != "ak_" {
		return nil, ErrInvalidAPIKey
	}

	// Hash the key
	keyHash := hashAPIKey(rawKey)

	// Look up the key
	var apiKey models.APIKey
	err := s.db.QueryRow(ctx, `
		SELECT id, user_id, key_hash, key_prefix, name, permissions, last_used_at, created_at, revoked_at
		FROM api_keys
		WHERE key_hash = $1
	`, keyHash).Scan(
		&apiKey.ID, &apiKey.UserID, &apiKey.KeyHash, &apiKey.KeyPrefix,
		&apiKey.Name, &apiKey.Permissions, &apiKey.LastUsedAt, &apiKey.CreatedAt, &apiKey.RevokedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInvalidAPIKey
		}
		return nil, fmt.Errorf("failed to validate API key: %w", err)
	}

	// Check if revoked
	if apiKey.RevokedAt != nil {
		return nil, ErrAPIKeyRevoked
	}

	// Update last used timestamp (async, don't block on this)
	go func() {
		_, _ = s.db.Exec(context.Background(), `
			UPDATE api_keys SET last_used_at = NOW() WHERE id = $1
		`, apiKey.ID)
	}()

	return &apiKey, nil
}

// IsKeyRevoked checks if an API key is revoked by its hash
// This is used for immediate revocation checks
func (s *Service) IsKeyRevoked(ctx context.Context, keyHash string) (bool, error) {
	var revokedAt *time.Time
	err := s.db.QueryRow(ctx, `
		SELECT revoked_at FROM api_keys WHERE key_hash = $1
	`, keyHash).Scan(&revokedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return true, nil // Key doesn't exist, treat as revoked
		}
		return false, fmt.Errorf("failed to check key revocation: %w", err)
	}
	return revokedAt != nil, nil
}

// generateAPIKey generates a secure API key
// Returns: rawKey, keyHash, keyPrefix, error
func generateAPIKey() (string, string, string, error) {
	// Generate 32 random bytes
	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", "", "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// Create the raw key with prefix
	rawKey := "ak_" + hex.EncodeToString(randomBytes)

	// Hash the key for storage
	keyHash := hashAPIKey(rawKey)

	// Create prefix for display (first 8 chars after "ak_")
	keyPrefix := rawKey[:11] // "ak_" + 8 chars

	return rawKey, keyHash, keyPrefix, nil
}

// hashAPIKey creates a SHA-256 hash of an API key
func hashAPIKey(rawKey string) string {
	hash := sha256.Sum256([]byte(rawKey))
	return hex.EncodeToString(hash[:])
}

// HashAPIKey is exported for use in tests and other packages
func HashAPIKey(rawKey string) string {
	return hashAPIKey(rawKey)
}
