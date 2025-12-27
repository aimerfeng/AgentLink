package models

import (
	"time"

	"github.com/google/uuid"
)

// UserType represents the type of user
type UserType string

const (
	UserTypeCreator   UserType = "creator"
	UserTypeDeveloper UserType = "developer"
	UserTypeAdmin     UserType = "admin"
)

// User represents a user in the system
type User struct {
	ID            uuid.UUID  `json:"id" db:"id"`
	Email         string     `json:"email" db:"email"`
	PasswordHash  string     `json:"-" db:"password_hash"`
	UserType      UserType   `json:"user_type" db:"user_type"`
	WalletAddress *string    `json:"wallet_address,omitempty" db:"wallet_address"`
	EmailVerified bool       `json:"email_verified" db:"email_verified"`
	CreatedAt     time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at" db:"updated_at"`
}

// CreatorProfile represents additional creator information
type CreatorProfile struct {
	UserID          uuid.UUID `json:"user_id" db:"user_id"`
	DisplayName     string    `json:"display_name" db:"display_name"`
	Bio             *string   `json:"bio,omitempty" db:"bio"`
	AvatarURL       *string   `json:"avatar_url,omitempty" db:"avatar_url"`
	Verified        bool      `json:"verified" db:"verified"`
	TotalEarnings   string    `json:"total_earnings" db:"total_earnings"`
	PendingEarnings string    `json:"pending_earnings" db:"pending_earnings"`
}
