package middleware

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/aimerfeng/AgentLink/internal/config"
	apierrors "github.com/aimerfeng/AgentLink/internal/errors"
	"github.com/aimerfeng/AgentLink/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Context keys for storing user information
const (
	ContextKeyUserID   = "user_id"
	ContextKeyUserType = "user_type"
	ContextKeyEmail    = "email"
	ContextKeyClaims   = "claims"
)

// Claims represents JWT claims
type Claims struct {
	UserID   string `json:"user_id"`
	UserType string `json:"user_type"`
	Email    string `json:"email"`
	jwt.RegisteredClaims
}

// JWTAuthenticator handles JWT token validation
type JWTAuthenticator struct {
	config *config.JWTConfig
}

// NewJWTAuthenticator creates a new JWT authenticator
func NewJWTAuthenticator(cfg *config.JWTConfig) *JWTAuthenticator {
	return &JWTAuthenticator{
		config: cfg,
	}
}

// JWTAuth creates a middleware that validates JWT tokens from the Authorization header
// It extracts the Bearer token, validates it, and sets user information in the context
func (j *JWTAuthenticator) JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			respondWithError(c, apierrors.ErrInvalidCredentialsError)
			c.Abort()
			return
		}

		// Extract token from "Bearer <token>"
		tokenString, err := extractBearerToken(authHeader)
		if err != nil {
			respondWithError(c, apierrors.ErrInvalidCredentialsError)
			c.Abort()
			return
		}

		// Validate token
		claims, err := j.ValidateAccessToken(tokenString)
		if err != nil {
			if errors.Is(err, ErrTokenExpired) {
				respondWithError(c, apierrors.ErrTokenExpiredError)
			} else {
				respondWithError(c, apierrors.ErrInvalidCredentialsError)
			}
			c.Abort()
			return
		}

		// Set user info in context
		c.Set(ContextKeyUserID, claims.UserID)
		c.Set(ContextKeyUserType, claims.UserType)
		c.Set(ContextKeyEmail, claims.Email)
		c.Set(ContextKeyClaims, claims)

		c.Next()
	}
}

// ValidateAccessToken validates an access token and returns claims
func (j *JWTAuthenticator) ValidateAccessToken(tokenString string) (*Claims, error) {
	claims, err := j.validateToken(tokenString)
	if err != nil {
		return nil, err
	}

	// Check if it's an access token
	if claims.Subject != "access" {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

// validateToken parses and validates a JWT token
func (j *JWTAuthenticator) validateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Validate signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(j.config.Secret), nil
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

// extractBearerToken extracts the token from a Bearer authorization header
func extractBearerToken(authHeader string) (string, error) {
	const bearerPrefix = "Bearer "
	if len(authHeader) < len(bearerPrefix) {
		return "", ErrInvalidToken
	}
	if !strings.HasPrefix(authHeader, bearerPrefix) {
		return "", ErrInvalidToken
	}
	return authHeader[len(bearerPrefix):], nil
}

// JWT validation errors
var (
	ErrInvalidToken = errors.New("invalid token")
	ErrTokenExpired = errors.New("token expired")
)

// respondWithError sends a standardized error response
func respondWithError(c *gin.Context, err *apierrors.APIError) {
	requestID, _ := c.Get("request_id")
	reqIDStr, _ := requestID.(string)
	correlationID, _ := c.Get("correlation_id")
	corrIDStr, _ := correlationID.(string)
	if corrIDStr == "" {
		corrIDStr = reqIDStr
	}

	response := apierrors.NewErrorResponse(
		err,
		reqIDStr,
		corrIDStr,
		c.Request.URL.Path,
		c.Request.Method,
	)

	c.JSON(err.HTTPStatus, response)
}

// RequireRole creates a middleware that checks if the user has one of the required roles
// This middleware must be used after JWTAuth middleware
func RequireRole(allowedRoles ...models.UserType) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user type from context (set by JWTAuth middleware)
		userTypeStr, exists := c.Get(ContextKeyUserType)
		if !exists {
			respondWithError(c, apierrors.ErrForbiddenError)
			c.Abort()
			return
		}

		userType := models.UserType(userTypeStr.(string))

		// Check if user has one of the allowed roles
		hasRole := false
		for _, role := range allowedRoles {
			if userType == role {
				hasRole = true
				break
			}
		}

		if !hasRole {
			respondWithError(c, &apierrors.APIError{
				Code:       apierrors.ErrForbidden,
				Message:    fmt.Sprintf("Access denied. Required role: %v", allowedRoles),
				HTTPStatus: http.StatusForbidden,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireCreator is a convenience middleware that requires the creator role
func RequireCreator() gin.HandlerFunc {
	return RequireRole(models.UserTypeCreator)
}

// RequireDeveloper is a convenience middleware that requires the developer role
func RequireDeveloper() gin.HandlerFunc {
	return RequireRole(models.UserTypeDeveloper)
}

// RequireAdmin is a convenience middleware that requires the admin role
func RequireAdmin() gin.HandlerFunc {
	return RequireRole(models.UserTypeAdmin)
}

// RequireCreatorOrDeveloper is a convenience middleware that requires either creator or developer role
func RequireCreatorOrDeveloper() gin.HandlerFunc {
	return RequireRole(models.UserTypeCreator, models.UserTypeDeveloper)
}

// GetUserIDFromContext extracts the user ID from the gin context
// Returns empty string if not found
func GetUserIDFromContext(c *gin.Context) string {
	userID, exists := c.Get(ContextKeyUserID)
	if !exists {
		return ""
	}
	return userID.(string)
}

// GetUserTypeFromContext extracts the user type from the gin context
// Returns empty string if not found
func GetUserTypeFromContext(c *gin.Context) models.UserType {
	userType, exists := c.Get(ContextKeyUserType)
	if !exists {
		return ""
	}
	return models.UserType(userType.(string))
}

// GetEmailFromContext extracts the email from the gin context
// Returns empty string if not found
func GetEmailFromContext(c *gin.Context) string {
	email, exists := c.Get(ContextKeyEmail)
	if !exists {
		return ""
	}
	return email.(string)
}

// GetClaimsFromContext extracts the full claims from the gin context
// Returns nil if not found
func GetClaimsFromContext(c *gin.Context) *Claims {
	claims, exists := c.Get(ContextKeyClaims)
	if !exists {
		return nil
	}
	return claims.(*Claims)
}

// RequestID adds a unique request ID to each request
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}
		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

// CorrelationID adds a correlation ID for distributed tracing
// The correlation ID is used to trace requests across multiple services
// It can be passed from upstream services or generated if not present
func CorrelationID() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check for existing correlation ID from upstream
		correlationID := c.GetHeader("X-Correlation-ID")
		if correlationID == "" {
			// Fall back to request ID if no correlation ID provided
			correlationID = c.GetString("request_id")
			if correlationID == "" {
				correlationID = uuid.New().String()
			}
		}
		c.Set("correlation_id", correlationID)
		c.Header("X-Correlation-ID", correlationID)
		c.Next()
	}
}

// GetCorrelationIDFromContext extracts the correlation ID from the gin context
// Returns empty string if not found
func GetCorrelationIDFromContext(c *gin.Context) string {
	correlationID, exists := c.Get("correlation_id")
	if !exists {
		return ""
	}
	return correlationID.(string)
}

// GetRequestIDFromContext extracts the request ID from the gin context
// Returns empty string if not found
func GetRequestIDFromContext(c *gin.Context) string {
	requestID, exists := c.Get("request_id")
	if !exists {
		return ""
	}
	return requestID.(string)
}

// CORS configures CORS headers
func CORS(allowedOrigins []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		
		// Check if origin is allowed
		allowed := false
		for _, o := range allowedOrigins {
			if o == origin || o == "*" {
				allowed = true
				break
			}
		}

		if allowed {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-ID, X-AgentLink-Key")
			c.Header("Access-Control-Expose-Headers", "X-Request-ID, X-RateLimit-Remaining")
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Max-Age", "43200") // 12 hours
		}

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
