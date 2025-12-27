package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// AgentStatus represents the status of an agent
type AgentStatus string

const (
	AgentStatusDraft    AgentStatus = "draft"
	AgentStatusActive   AgentStatus = "active"
	AgentStatusInactive AgentStatus = "inactive"
)

// Agent represents an AI agent
type Agent struct {
	ID              uuid.UUID       `json:"id" db:"id"`
	CreatorID       uuid.UUID       `json:"creator_id" db:"creator_id"`
	Name            string          `json:"name" db:"name"`
	Description     *string         `json:"description,omitempty" db:"description"`
	Category        *string         `json:"category,omitempty" db:"category"`
	Status          AgentStatus     `json:"status" db:"status"`
	ConfigEncrypted []byte          `json:"-" db:"config_encrypted"`
	ConfigIV        []byte          `json:"-" db:"config_iv"`
	PricePerCall    decimal.Decimal `json:"price_per_call" db:"price_per_call"`
	TotalCalls      int64           `json:"total_calls" db:"total_calls"`
	TotalRevenue    decimal.Decimal `json:"total_revenue" db:"total_revenue"`
	AverageRating   decimal.Decimal `json:"average_rating" db:"average_rating"`
	ReviewCount     int             `json:"review_count" db:"review_count"`
	TokenID         *int64          `json:"token_id,omitempty" db:"token_id"`
	TokenTxHash     *string         `json:"token_tx_hash,omitempty" db:"token_tx_hash"`
	Version         int             `json:"version" db:"version"`
	CreatedAt       time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at" db:"updated_at"`
	PublishedAt     *time.Time      `json:"published_at,omitempty" db:"published_at"`
}

// AgentConfig represents the decrypted agent configuration
type AgentConfig struct {
	SystemPrompt  string           `json:"system_prompt"`
	Model         string           `json:"model"`
	Provider      string           `json:"provider"`
	Temperature   float64          `json:"temperature"`
	MaxTokens     int              `json:"max_tokens"`
	TopP          float64          `json:"top_p"`
	KnowledgeBase *KnowledgeConfig `json:"knowledge_base,omitempty"`
}

// KnowledgeConfig represents knowledge base configuration
type KnowledgeConfig struct {
	Enabled      bool     `json:"enabled"`
	FileIDs      []string `json:"file_ids"`
	ChunkSize    int      `json:"chunk_size"`
	ChunkOverlap int      `json:"chunk_overlap"`
	TopK         int      `json:"top_k"`
}

// AgentVersion represents a historical version of an agent
type AgentVersion struct {
	ID              uuid.UUID `json:"id" db:"id"`
	AgentID         uuid.UUID `json:"agent_id" db:"agent_id"`
	Version         int       `json:"version" db:"version"`
	ConfigEncrypted []byte    `json:"-" db:"config_encrypted"`
	ConfigIV        []byte    `json:"-" db:"config_iv"`
	CreatedAt       time.Time `json:"created_at" db:"created_at"`
}
