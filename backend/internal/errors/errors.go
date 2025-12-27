package errors

import (
	"net/http"
)

// ErrorCode represents a standardized error code
type ErrorCode string

const (
	// Authentication errors (401xx)
	ErrInvalidCredentials ErrorCode = "40101"
	ErrTokenExpired       ErrorCode = "40102"
	ErrInvalidAPIKey      ErrorCode = "40103"

	// Authorization errors (403xx)
	ErrForbidden     ErrorCode = "40301"
	ErrAgentNotOwned ErrorCode = "40302"

	// Resource errors (404xx)
	ErrAgentNotFound ErrorCode = "40401"
	ErrUserNotFound  ErrorCode = "40402"

	// Request errors (400xx)
	ErrInvalidRequest   ErrorCode = "40001"
	ErrValidationFailed ErrorCode = "40002"

	// Rate limit errors (429xx)
	ErrQuotaExhausted ErrorCode = "42901"
	ErrRateLimited    ErrorCode = "42902"

	// Server errors (500xx)
	ErrInternalServer      ErrorCode = "50001"
	ErrUpstreamTimeout     ErrorCode = "50401"
	ErrUpstreamUnavailable ErrorCode = "50301"
)

// APIError represents a standardized API error
type APIError struct {
	Code       ErrorCode `json:"code"`
	Message    string    `json:"message"`
	Details    any       `json:"details,omitempty"`
	HTTPStatus int       `json:"-"`
}

// Error implements the error interface
func (e *APIError) Error() string {
	return e.Message
}

// ErrorResponse represents the error response format
type ErrorResponse struct {
	Error     APIError `json:"error"`
	RequestID string   `json:"request_id"`
}

// Common errors
var (
	ErrInvalidCredentialsError = &APIError{
		Code:       ErrInvalidCredentials,
		Message:    "Invalid email or password",
		HTTPStatus: http.StatusUnauthorized,
	}

	ErrTokenExpiredError = &APIError{
		Code:       ErrTokenExpired,
		Message:    "Token has expired",
		HTTPStatus: http.StatusUnauthorized,
	}

	ErrInvalidAPIKeyError = &APIError{
		Code:       ErrInvalidAPIKey,
		Message:    "Invalid or revoked API key",
		HTTPStatus: http.StatusUnauthorized,
	}

	ErrForbiddenError = &APIError{
		Code:       ErrForbidden,
		Message:    "Access denied",
		HTTPStatus: http.StatusForbidden,
	}

	ErrAgentNotFoundError = &APIError{
		Code:       ErrAgentNotFound,
		Message:    "Agent not found",
		HTTPStatus: http.StatusNotFound,
	}

	ErrUserNotFoundError = &APIError{
		Code:       ErrUserNotFound,
		Message:    "User not found",
		HTTPStatus: http.StatusNotFound,
	}

	ErrQuotaExhaustedError = &APIError{
		Code:       ErrQuotaExhausted,
		Message:    "API quota exhausted",
		HTTPStatus: http.StatusTooManyRequests,
	}

	ErrRateLimitedError = &APIError{
		Code:       ErrRateLimited,
		Message:    "Rate limit exceeded",
		HTTPStatus: http.StatusTooManyRequests,
	}

	ErrInternalServerError = &APIError{
		Code:       ErrInternalServer,
		Message:    "Internal server error",
		HTTPStatus: http.StatusInternalServerError,
	}

	ErrUpstreamTimeoutError = &APIError{
		Code:       ErrUpstreamTimeout,
		Message:    "Upstream service timeout",
		HTTPStatus: http.StatusGatewayTimeout,
	}

	ErrUpstreamUnavailableError = &APIError{
		Code:       ErrUpstreamUnavailable,
		Message:    "Upstream service unavailable",
		HTTPStatus: http.StatusServiceUnavailable,
	}
)

// NewValidationError creates a validation error with details
func NewValidationError(details any) *APIError {
	return &APIError{
		Code:       ErrValidationFailed,
		Message:    "Validation failed",
		Details:    details,
		HTTPStatus: http.StatusBadRequest,
	}
}

// NewInvalidRequestError creates an invalid request error
func NewInvalidRequestError(message string) *APIError {
	return &APIError{
		Code:       ErrInvalidRequest,
		Message:    message,
		HTTPStatus: http.StatusBadRequest,
	}
}
