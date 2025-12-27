package auth

import "errors"

// Auth-specific errors
var (
	ErrEmailAlreadyExists   = errors.New("email already exists")
	ErrDisplayNameRequired  = errors.New("display name is required for creators")
	ErrInvalidCredentials   = errors.New("invalid email or password")
	ErrInvalidToken         = errors.New("invalid token")
	ErrTokenExpired         = errors.New("token has expired")
	ErrUserNotFound         = errors.New("user not found")
	ErrUnauthorized         = errors.New("unauthorized")
)
