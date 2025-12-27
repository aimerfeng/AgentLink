package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/aimerfeng/AgentLink/internal/config"
	"github.com/aimerfeng/AgentLink/internal/models"
	"github.com/alexedwards/argon2id"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Service handles authentication operations
type Service struct {
	db     *pgxpool.Pool
	config *config.JWTConfig
	quota  *config.QuotaConfig
}

// NewService creates a new auth service
func NewService(db *pgxpool.Pool, jwtCfg *config.JWTConfig, quotaCfg *config.QuotaConfig) *Service {
	return &Service{
		db:     db,
		config: jwtCfg,
		quota:  quotaCfg,
	}
}

// Claims represents JWT claims
type Claims struct {
	UserID   string `json:"user_id"`
	UserType string `json:"user_type"`
	Email    string `json:"email"`
	jwt.RegisteredClaims
}

// TokenPair represents access and refresh tokens
type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	TokenType    string    `json:"token_type"`
}

// RegisterRequest represents a registration request
type RegisterRequest struct {
	Email       string           `json:"email" binding:"required,email"`
	Password    string           `json:"password" binding:"required,min=8"`
	UserType    models.UserType  `json:"user_type" binding:"required,oneof=creator developer"`
	DisplayName string           `json:"display_name"` // Required for creators
}

// LoginRequest represents a login request
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// RefreshRequest represents a token refresh request
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// UserResponse represents a user response (without sensitive data)
type UserResponse struct {
	ID            uuid.UUID        `json:"id"`
	Email         string           `json:"email"`
	UserType      models.UserType  `json:"user_type"`
	WalletAddress *string          `json:"wallet_address,omitempty"`
	EmailVerified bool             `json:"email_verified"`
	CreatedAt     time.Time        `json:"created_at"`
}

// RegisterResponse represents a registration response
type RegisterResponse struct {
	User    UserResponse `json:"user"`
	Tokens  TokenPair    `json:"tokens"`
	Message string       `json:"message"`
}

// LoginResponse represents a login response
type LoginResponse struct {
	User   UserResponse `json:"user"`
	Tokens TokenPair    `json:"tokens"`
}


// Register creates a new user account
func (s *Service) Register(ctx context.Context, req *RegisterRequest) (*RegisterResponse, error) {
	// Check if email already exists
	var exists bool
	err := s.db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)", req.Email).Scan(&exists)
	if err != nil {
		return nil, fmt.Errorf("failed to check email existence: %w", err)
	}
	if exists {
		return nil, ErrEmailAlreadyExists
	}

	// Validate creator has display name
	if req.UserType == models.UserTypeCreator && req.DisplayName == "" {
		return nil, ErrDisplayNameRequired
	}

	// Hash password using Argon2id
	passwordHash, err := argon2id.CreateHash(req.Password, argon2id.DefaultParams)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Start transaction
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Create user
	var user models.User
	err = tx.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, user_type, email_verified)
		VALUES ($1, $2, $3, $4)
		RETURNING id, email, password_hash, user_type, wallet_address, email_verified, created_at, updated_at
	`, req.Email, passwordHash, req.UserType, false).Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.UserType,
		&user.WalletAddress, &user.EmailVerified, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Create quota record with free initial quota
	_, err = tx.Exec(ctx, `
		INSERT INTO quotas (user_id, total_quota, used_quota, free_quota)
		VALUES ($1, $2, 0, $2)
	`, user.ID, s.quota.FreeInitial)
	if err != nil {
		return nil, fmt.Errorf("failed to create quota: %w", err)
	}

	// If creator, create profile
	if req.UserType == models.UserTypeCreator {
		_, err = tx.Exec(ctx, `
			INSERT INTO creator_profiles (user_id, display_name, verified, total_earnings, pending_earnings)
			VALUES ($1, $2, false, 0, 0)
		`, user.ID, req.DisplayName)
		if err != nil {
			return nil, fmt.Errorf("failed to create creator profile: %w", err)
		}
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Generate tokens
	tokens, err := s.generateTokenPair(&user)
	if err != nil {
		return nil, fmt.Errorf("failed to generate tokens: %w", err)
	}

	return &RegisterResponse{
		User:    toUserResponse(&user),
		Tokens:  *tokens,
		Message: "Registration successful. Please verify your email.",
	}, nil
}

// Login authenticates a user and returns tokens
func (s *Service) Login(ctx context.Context, req *LoginRequest) (*LoginResponse, error) {
	// Find user by email
	var user models.User
	err := s.db.QueryRow(ctx, `
		SELECT id, email, password_hash, user_type, wallet_address, email_verified, created_at, updated_at
		FROM users WHERE email = $1
	`, req.Email).Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.UserType,
		&user.WalletAddress, &user.EmailVerified, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Return generic error to not reveal if email exists
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("failed to find user: %w", err)
	}

	// Verify password
	match, err := argon2id.ComparePasswordAndHash(req.Password, user.PasswordHash)
	if err != nil {
		return nil, fmt.Errorf("failed to verify password: %w", err)
	}
	if !match {
		return nil, ErrInvalidCredentials
	}

	// Generate tokens
	tokens, err := s.generateTokenPair(&user)
	if err != nil {
		return nil, fmt.Errorf("failed to generate tokens: %w", err)
	}

	return &LoginResponse{
		User:   toUserResponse(&user),
		Tokens: *tokens,
	}, nil
}


// RefreshTokens generates new tokens from a valid refresh token
func (s *Service) RefreshTokens(ctx context.Context, refreshToken string) (*TokenPair, error) {
	// Parse and validate refresh token
	claims, err := s.validateToken(refreshToken)
	if err != nil {
		return nil, ErrInvalidToken
	}

	// Check if it's a refresh token
	if claims.Subject != "refresh" {
		return nil, ErrInvalidToken
	}

	// Get user from database to ensure they still exist and are active
	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, ErrInvalidToken
	}

	var user models.User
	err = s.db.QueryRow(ctx, `
		SELECT id, email, password_hash, user_type, wallet_address, email_verified, created_at, updated_at
		FROM users WHERE id = $1
	`, userID).Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.UserType,
		&user.WalletAddress, &user.EmailVerified, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to find user: %w", err)
	}

	// Generate new token pair (token rotation)
	return s.generateTokenPair(&user)
}

// ValidateAccessToken validates an access token and returns claims
func (s *Service) ValidateAccessToken(tokenString string) (*Claims, error) {
	claims, err := s.validateToken(tokenString)
	if err != nil {
		return nil, err
	}

	// Check if it's an access token
	if claims.Subject != "access" {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

// GetUserByID retrieves a user by ID
func (s *Service) GetUserByID(ctx context.Context, userID uuid.UUID) (*models.User, error) {
	var user models.User
	err := s.db.QueryRow(ctx, `
		SELECT id, email, password_hash, user_type, wallet_address, email_verified, created_at, updated_at
		FROM users WHERE id = $1
	`, userID).Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.UserType,
		&user.WalletAddress, &user.EmailVerified, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to find user: %w", err)
	}
	return &user, nil
}

// generateTokenPair creates access and refresh tokens
func (s *Service) generateTokenPair(user *models.User) (*TokenPair, error) {
	now := time.Now()
	accessExpiry := now.Add(s.config.AccessTokenExpiry)
	refreshExpiry := now.Add(s.config.RefreshTokenExpiry)

	// Generate access token
	accessClaims := &Claims{
		UserID:   user.ID.String(),
		UserType: string(user.UserType),
		Email:    user.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "access",
			Issuer:    s.config.Issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(accessExpiry),
			ID:        generateJTI(),
		},
	}

	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessTokenString, err := accessToken.SignedString([]byte(s.config.Secret))
	if err != nil {
		return nil, fmt.Errorf("failed to sign access token: %w", err)
	}

	// Generate refresh token
	refreshClaims := &Claims{
		UserID:   user.ID.String(),
		UserType: string(user.UserType),
		Email:    user.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "refresh",
			Issuer:    s.config.Issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(refreshExpiry),
			ID:        generateJTI(),
		},
	}

	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshTokenString, err := refreshToken.SignedString([]byte(s.config.Secret))
	if err != nil {
		return nil, fmt.Errorf("failed to sign refresh token: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessTokenString,
		RefreshToken: refreshTokenString,
		ExpiresAt:    accessExpiry,
		TokenType:    "Bearer",
	}, nil
}

// validateToken parses and validates a JWT token
func (s *Service) validateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.config.Secret), nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

// toUserResponse converts a User to UserResponse
func toUserResponse(user *models.User) UserResponse {
	return UserResponse{
		ID:            user.ID,
		Email:         user.Email,
		UserType:      user.UserType,
		WalletAddress: user.WalletAddress,
		EmailVerified: user.EmailVerified,
		CreatedAt:     user.CreatedAt,
	}
}

// generateJTI generates a unique JWT ID
func generateJTI() string {
	b := make([]byte, 16)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
