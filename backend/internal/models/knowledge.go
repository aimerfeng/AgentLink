package models

import (
	"time"

	"github.com/google/uuid"
)

// FileStatus represents the processing status of a knowledge file
type FileStatus string

const (
	FileStatusProcessing FileStatus = "processing"
	FileStatusCompleted  FileStatus = "completed"
	FileStatusFailed     FileStatus = "failed"
)

// KnowledgeFile represents an uploaded knowledge base file
type KnowledgeFile struct {
	ID          uuid.UUID  `json:"id" db:"id"`
	AgentID     uuid.UUID  `json:"agent_id" db:"agent_id"`
	Filename    string     `json:"filename" db:"filename"`
	FileType    string     `json:"file_type" db:"file_type"`
	FileSize    int64      `json:"file_size" db:"file_size"`
	S3Key       string     `json:"-" db:"s3_key"`
	ChunkCount  int        `json:"chunk_count" db:"chunk_count"`
	Status      FileStatus `json:"status" db:"status"`
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
	ProcessedAt *time.Time `json:"processed_at,omitempty" db:"processed_at"`
}

// KnowledgeEmbedding represents a vector embedding for knowledge retrieval
type KnowledgeEmbedding struct {
	ID         uuid.UUID         `json:"id" db:"id"`
	AgentID    uuid.UUID         `json:"agent_id" db:"agent_id"`
	FileID     uuid.UUID         `json:"file_id" db:"file_id"`
	ChunkIndex int               `json:"chunk_index" db:"chunk_index"`
	Content    string            `json:"content" db:"content"`
	Embedding  []float32         `json:"-" db:"embedding"`
	Metadata   map[string]string `json:"metadata,omitempty" db:"metadata"`
	CreatedAt  time.Time         `json:"created_at" db:"created_at"`
}
