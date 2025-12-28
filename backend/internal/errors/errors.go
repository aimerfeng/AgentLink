package errors

import (
	"fmt"
	"net/http"
	"time"
)

// ErrorCode represents a standardized error code
type ErrorCode string

const (
	// Request errors (400xx)
	ErrInvalidRequest   ErrorCode = "40001"
	ErrValidationFailed ErrorCode = "40002"
	ErrInvalidJSON      ErrorCode = "40003"
	ErrMissingParameter ErrorCode = "40004"

	// Authentication errors (401xx)
	ErrUnauthorized       ErrorCode = "40100"
	ErrInvalidCredentials ErrorCode = "40101"
	ErrTokenExpired       ErrorCode = "40102"
	ErrInvalidAPIKey      ErrorCode = "40103"
	ErrMissingAPIKey      ErrorCode = "40104"

	// Authorization errors (403xx)
	ErrForbidden       ErrorCode = "40301"
	ErrAgentNotOwned   ErrorCode = "40302"
	ErrAgentNotActive  ErrorCode = "40303"
	ErrAccessDenied    ErrorCode = "40304"

	// Resource errors (404xx)
	ErrNotFound      ErrorCode = "40400"
	ErrAgentNotFound ErrorCode = "40401"
	ErrUserNotFound  ErrorCode = "40402"
	ErrAPIKeyNotFound ErrorCode = "40403"

	// Rate limit errors (429xx)
	ErrQuotaExhausted ErrorCode = "42901"
	ErrRateLimited    ErrorCode = "42902"

	// Server errors (500xx)
	ErrInternalServer      ErrorCode = "50001"
	ErrDatabaseError       ErrorCode = "50002"
	ErrCacheError          ErrorCode = "50003"
	ErrUpstreamError       ErrorCode = "50004"

	// Gateway errors (502xx, 503xx, 504xx)
	ErrBadGateway          ErrorCode = "50201"
	ErrUpstreamUnavailable ErrorCode = "50301"
	ErrCircuitBreakerOpen  ErrorCode = "50302"
	ErrUpstreamTimeout     ErrorCode = "50401"
)

// APIError represents a standardized API error
type APIError struct {
	Code       ErrorCode `json:"code"`
	Message    string    `json:"message"`
	Details    any       `json:"details,omitempty"`
	HTTPStatus int       `json:"-"`
	Timestamp  time.Time `json:"timestamp,omitempty"`
}

// Error implements the error interface
func (e *APIError) Error() string {
	return e.Message
}

// WithDetails returns a copy of the error with additional details
func (e *APIError) WithDetails(details any) *APIError {
	return &APIError{
		Code:       e.Code,
		Message:    e.Message,
		Details:    details,
		HTTPStatus: e.HTTPStatus,
		Timestamp:  time.Now().UTC(),
	}
}

// WithMessage returns a copy of the error with a custom message
func (e *APIError) WithMessage(message string) *APIError {
	return &APIError{
		Code:       e.Code,
		Message:    message,
		Details:    e.Details,
		HTTPStatus: e.HTTPStatus,
		Timestamp:  time.Now().UTC(),
	}
}

// ErrorResponse represents the standardized error response format
// This format is consistent across all API endpoints
type ErrorResponse struct {
	Error         ErrorDetail `json:"error"`
	RequestID     string      `json:"request_id,omitempty"`
	CorrelationID string      `json:"correlation_id,omitempty"`
}

// ErrorDetail contains the detailed error information
type ErrorDetail struct {
	Code      ErrorCode `json:"code"`
	Message   string    `json:"message"`
	Details   any       `json:"details,omitempty"`
	Timestamp string    `json:"timestamp"`
	Path      string    `json:"path,omitempty"`
	Method    string    `json:"method,omitempty"`
}

// NewErrorResponse creates a standardized error response
func NewErrorResponse(apiErr *APIError, requestID, correlationID, path, method string) *ErrorResponse {
	return &ErrorResponse{
		Error: ErrorDetail{
			Code:      apiErr.Code,
			Message:   apiErr.Message,
			Details:   apiErr.Details,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Path:      path,
			Method:    method,
		},
		RequestID:     requestID,
		CorrelationID: correlationID,
	}
}

// RateLimitErrorResponse extends ErrorResponse with rate limit specific fields
type RateLimitErrorResponse struct {
	ErrorResponse
	RetryAfter int64 `json:"retry_after_seconds,omitempty"`
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

	ErrMissingAPIKeyError = &APIError{
		Code:       ErrMissingAPIKey,
		Message:    "Missing X-AgentLink-Key header",
		HTTPStatus: http.StatusUnauthorized,
	}

	ErrForbiddenError = &APIError{
		Code:       ErrForbidden,
		Message:    "Access denied",
		HTTPStatus: http.StatusForbidden,
	}

	ErrAgentNotActiveError = &APIError{
		Code:       ErrAgentNotActive,
		Message:    "Agent is not active",
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

	ErrDatabaseErrorError = &APIError{
		Code:       ErrDatabaseError,
		Message:    "Database error",
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

	ErrCircuitBreakerOpenError = &APIError{
		Code:       ErrCircuitBreakerOpen,
		Message:    "Service temporarily unavailable due to circuit breaker",
		HTTPStatus: http.StatusServiceUnavailable,
	}

	ErrBadGatewayError = &APIError{
		Code:       ErrBadGateway,
		Message:    "Bad gateway",
		HTTPStatus: http.StatusBadGateway,
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

// NewRateLimitError creates a rate limit error with retry information
func NewRateLimitError(retryAfterSeconds int64) *APIError {
	return &APIError{
		Code:       ErrRateLimited,
		Message:    fmt.Sprintf("Rate limit exceeded. Retry after %d seconds", retryAfterSeconds),
		Details:    map[string]int64{"retry_after_seconds": retryAfterSeconds},
		HTTPStatus: http.StatusTooManyRequests,
	}
}

// NewUpstreamError creates an upstream error with details
func NewUpstreamError(provider string, statusCode int) *APIError {
	return &APIError{
		Code:       ErrUpstreamError,
		Message:    fmt.Sprintf("Upstream service error from %s", provider),
		Details:    map[string]interface{}{"provider": provider, "status_code": statusCode},
		HTTPStatus: http.StatusBadGateway,
	}
}

// NewNotFoundError creates a not found error for a specific resource
func NewNotFoundError(resource string) *APIError {
	return &APIError{
		Code:       ErrNotFound,
		Message:    fmt.Sprintf("%s not found", resource),
		HTTPStatus: http.StatusNotFound,
	}
}

// IsRetryable returns true if the error is retryable
func IsRetryable(err *APIError) bool {
	switch err.Code {
	case ErrUpstreamTimeout, ErrUpstreamUnavailable, ErrCircuitBreakerOpen, ErrRateLimited:
		return true
	default:
		return false
	}
}

// IsClientError returns true if the error is a client error (4xx)
func IsClientError(err *APIError) bool {
	return err.HTTPStatus >= 400 && err.HTTPStatus < 500
}

// IsServerError returns true if the error is a server error (5xx)
func IsServerError(err *APIError) bool {
	return err.HTTPStatus >= 500
}

// GetHTTPStatusFromCode returns the HTTP status code for an error code
func GetHTTPStatusFromCode(code ErrorCode) int {
	switch code {
	case ErrInvalidRequest, ErrValidationFailed, ErrInvalidJSON, ErrMissingParameter:
		return http.StatusBadRequest
	case ErrUnauthorized, ErrInvalidCredentials, ErrTokenExpired, ErrInvalidAPIKey, ErrMissingAPIKey:
		return http.StatusUnauthorized
	case ErrForbidden, ErrAgentNotOwned, ErrAgentNotActive, ErrAccessDenied:
		return http.StatusForbidden
	case ErrNotFound, ErrAgentNotFound, ErrUserNotFound, ErrAPIKeyNotFound:
		return http.StatusNotFound
	case ErrQuotaExhausted, ErrRateLimited:
		return http.StatusTooManyRequests
	case ErrBadGateway:
		return http.StatusBadGateway
	case ErrUpstreamUnavailable, ErrCircuitBreakerOpen:
		return http.StatusServiceUnavailable
	case ErrUpstreamTimeout:
		return http.StatusGatewayTimeout
	default:
		return http.StatusInternalServerError
	}
}
