package errors

import (
	"net/http"
	"testing"
	"time"

	"pgregory.net/rapid"
)

// TestProperty_ErrorResponse_StandardFormat tests that all error responses follow the standard format
// *For any* API error, the error response SHALL include code, message, timestamp, request_id, and correlation_id.
// **Validates: Requirements A6.6**
func TestProperty_ErrorResponse_StandardFormat(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate random error code
		errorCodes := []ErrorCode{
			ErrInvalidRequest, ErrValidationFailed, ErrInvalidJSON,
			ErrUnauthorized, ErrInvalidCredentials, ErrTokenExpired, ErrInvalidAPIKey,
			ErrForbidden, ErrAgentNotOwned, ErrAgentNotActive,
			ErrNotFound, ErrAgentNotFound, ErrUserNotFound,
			ErrQuotaExhausted, ErrRateLimited,
			ErrInternalServer, ErrUpstreamTimeout, ErrUpstreamUnavailable,
		}
		codeIdx := rapid.IntRange(0, len(errorCodes)-1).Draw(rt, "codeIdx")
		code := errorCodes[codeIdx]

		// Generate random message
		message := rapid.StringMatching(`[a-zA-Z0-9 .,!?]{10,100}`).Draw(rt, "message")

		// Generate random request ID and correlation ID
		requestID := rapid.StringMatching(`[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}`).Draw(rt, "requestID")
		correlationID := rapid.StringMatching(`[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}`).Draw(rt, "correlationID")

		// Generate random path and method
		paths := []string{"/api/v1/agents", "/proxy/v1/agents/123/chat", "/api/v1/users"}
		methods := []string{"GET", "POST", "PUT", "DELETE"}
		pathIdx := rapid.IntRange(0, len(paths)-1).Draw(rt, "pathIdx")
		methodIdx := rapid.IntRange(0, len(methods)-1).Draw(rt, "methodIdx")
		path := paths[pathIdx]
		method := methods[methodIdx]

		// Create API error
		apiErr := &APIError{
			Code:       code,
			Message:    message,
			HTTPStatus: GetHTTPStatusFromCode(code),
		}

		// Create error response
		response := NewErrorResponse(apiErr, requestID, correlationID, path, method)

		// Property 1: Response must have error code
		if response.Error.Code == "" {
			t.Fatal("PROPERTY VIOLATION: Error response must have error code")
		}

		// Property 2: Response must have message
		if response.Error.Message == "" {
			t.Fatal("PROPERTY VIOLATION: Error response must have message")
		}

		// Property 3: Response must have timestamp
		if response.Error.Timestamp == "" {
			t.Fatal("PROPERTY VIOLATION: Error response must have timestamp")
		}

		// Property 4: Timestamp must be valid RFC3339 format
		_, err := time.Parse(time.RFC3339, response.Error.Timestamp)
		if err != nil {
			t.Fatalf("PROPERTY VIOLATION: Timestamp must be valid RFC3339 format: %v", err)
		}

		// Property 5: Response must have request ID
		if response.RequestID == "" {
			t.Fatal("PROPERTY VIOLATION: Error response must have request_id")
		}

		// Property 6: Response must have correlation ID
		if response.CorrelationID == "" {
			t.Fatal("PROPERTY VIOLATION: Error response must have correlation_id")
		}

		// Property 7: Path and method should be included
		if response.Error.Path != path {
			t.Fatalf("PROPERTY VIOLATION: Path should be %s, got %s", path, response.Error.Path)
		}
		if response.Error.Method != method {
			t.Fatalf("PROPERTY VIOLATION: Method should be %s, got %s", method, response.Error.Method)
		}
	})
}

// TestProperty_ErrorResponse_HTTPStatusMapping tests that error codes map to correct HTTP status codes
// *For any* error code, the HTTP status code SHALL be consistent with the error category.
// **Validates: Requirements A6.6**
func TestProperty_ErrorResponse_HTTPStatusMapping(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Test 4xx client errors
		clientErrorCodes := []ErrorCode{
			ErrInvalidRequest, ErrValidationFailed, ErrInvalidJSON, ErrMissingParameter,
			ErrUnauthorized, ErrInvalidCredentials, ErrTokenExpired, ErrInvalidAPIKey, ErrMissingAPIKey,
			ErrForbidden, ErrAgentNotOwned, ErrAgentNotActive, ErrAccessDenied,
			ErrNotFound, ErrAgentNotFound, ErrUserNotFound, ErrAPIKeyNotFound,
			ErrQuotaExhausted, ErrRateLimited,
		}

		codeIdx := rapid.IntRange(0, len(clientErrorCodes)-1).Draw(rt, "clientCodeIdx")
		code := clientErrorCodes[codeIdx]
		status := GetHTTPStatusFromCode(code)

		// Property: Client error codes should map to 4xx status
		if status < 400 || status >= 500 {
			t.Fatalf("PROPERTY VIOLATION: Client error code %s should map to 4xx status, got %d", code, status)
		}
	})
}

// TestProperty_ErrorResponse_ServerErrorMapping tests server error code mapping
func TestProperty_ErrorResponse_ServerErrorMapping(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Test 5xx server errors
		serverErrorCodes := []ErrorCode{
			ErrInternalServer, ErrDatabaseError, ErrCacheError, ErrUpstreamError,
			ErrBadGateway, ErrUpstreamUnavailable, ErrCircuitBreakerOpen, ErrUpstreamTimeout,
		}

		codeIdx := rapid.IntRange(0, len(serverErrorCodes)-1).Draw(rt, "serverCodeIdx")
		code := serverErrorCodes[codeIdx]
		status := GetHTTPStatusFromCode(code)

		// Property: Server error codes should map to 5xx status
		if status < 500 || status >= 600 {
			t.Fatalf("PROPERTY VIOLATION: Server error code %s should map to 5xx status, got %d", code, status)
		}
	})
}

// TestProperty_ErrorResponse_RetryableErrors tests that retryable errors are correctly identified
// *For any* error, the system SHALL correctly identify if it is retryable.
// **Validates: Requirements A6.6**
func TestProperty_ErrorResponse_RetryableErrors(t *testing.T) {
	// Errors that should be retryable
	retryableErrors := []*APIError{
		ErrUpstreamTimeoutError,
		ErrUpstreamUnavailableError,
		ErrCircuitBreakerOpenError,
		ErrRateLimitedError,
	}

	for _, err := range retryableErrors {
		if !IsRetryable(err) {
			t.Fatalf("PROPERTY VIOLATION: Error %s should be retryable", err.Code)
		}
	}

	// Errors that should NOT be retryable
	nonRetryableErrors := []*APIError{
		ErrInvalidCredentialsError,
		ErrInvalidAPIKeyError,
		ErrForbiddenError,
		ErrAgentNotFoundError,
		ErrQuotaExhaustedError,
		ErrInternalServerError,
	}

	for _, err := range nonRetryableErrors {
		if IsRetryable(err) {
			t.Fatalf("PROPERTY VIOLATION: Error %s should NOT be retryable", err.Code)
		}
	}
}

// TestProperty_ErrorResponse_ClientServerClassification tests client/server error classification
// *For any* error, the system SHALL correctly classify it as client or server error.
// **Validates: Requirements A6.6**
func TestProperty_ErrorResponse_ClientServerClassification(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate random HTTP status
		status := rapid.IntRange(400, 599).Draw(rt, "status")

		apiErr := &APIError{
			Code:       ErrInternalServer,
			Message:    "Test error",
			HTTPStatus: status,
		}

		isClient := IsClientError(apiErr)
		isServer := IsServerError(apiErr)

		// Property: Error must be either client or server, not both
		if isClient && isServer {
			t.Fatal("PROPERTY VIOLATION: Error cannot be both client and server error")
		}

		// Property: 4xx errors are client errors
		if status >= 400 && status < 500 {
			if !isClient {
				t.Fatalf("PROPERTY VIOLATION: Status %d should be client error", status)
			}
			if isServer {
				t.Fatalf("PROPERTY VIOLATION: Status %d should not be server error", status)
			}
		}

		// Property: 5xx errors are server errors
		if status >= 500 && status < 600 {
			if !isServer {
				t.Fatalf("PROPERTY VIOLATION: Status %d should be server error", status)
			}
			if isClient {
				t.Fatalf("PROPERTY VIOLATION: Status %d should not be client error", status)
			}
		}
	})
}

// TestProperty_ErrorResponse_WithDetails tests that WithDetails preserves error properties
// *For any* error with details, the original error properties SHALL be preserved.
// **Validates: Requirements A6.6**
func TestProperty_ErrorResponse_WithDetails(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate random error
		message := rapid.StringMatching(`[a-zA-Z0-9 ]{10,50}`).Draw(rt, "message")
		status := rapid.IntRange(400, 599).Draw(rt, "status")

		originalErr := &APIError{
			Code:       ErrInvalidRequest,
			Message:    message,
			HTTPStatus: status,
		}

		// Add details
		details := map[string]string{
			"field": rapid.StringMatching(`[a-z]{5,10}`).Draw(rt, "field"),
			"error": rapid.StringMatching(`[a-z ]{10,30}`).Draw(rt, "error"),
		}

		errWithDetails := originalErr.WithDetails(details)

		// Property: Code should be preserved
		if errWithDetails.Code != originalErr.Code {
			t.Fatal("PROPERTY VIOLATION: Code should be preserved")
		}

		// Property: Message should be preserved
		if errWithDetails.Message != originalErr.Message {
			t.Fatal("PROPERTY VIOLATION: Message should be preserved")
		}

		// Property: HTTP status should be preserved
		if errWithDetails.HTTPStatus != originalErr.HTTPStatus {
			t.Fatal("PROPERTY VIOLATION: HTTP status should be preserved")
		}

		// Property: Details should be set
		if errWithDetails.Details == nil {
			t.Fatal("PROPERTY VIOLATION: Details should be set")
		}

		// Property: Timestamp should be set
		if errWithDetails.Timestamp.IsZero() {
			t.Fatal("PROPERTY VIOLATION: Timestamp should be set")
		}
	})
}

// TestProperty_ErrorResponse_WithMessage tests that WithMessage preserves error properties
// *For any* error with custom message, the original error properties SHALL be preserved except message.
// **Validates: Requirements A6.6**
func TestProperty_ErrorResponse_WithMessage(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate random error
		originalMessage := rapid.StringMatching(`[a-zA-Z0-9 ]{10,50}`).Draw(rt, "originalMessage")
		newMessage := rapid.StringMatching(`[a-zA-Z0-9 ]{10,50}`).Draw(rt, "newMessage")
		status := rapid.IntRange(400, 599).Draw(rt, "status")

		originalErr := &APIError{
			Code:       ErrValidationFailed,
			Message:    originalMessage,
			HTTPStatus: status,
		}

		errWithMessage := originalErr.WithMessage(newMessage)

		// Property: Code should be preserved
		if errWithMessage.Code != originalErr.Code {
			t.Fatal("PROPERTY VIOLATION: Code should be preserved")
		}

		// Property: Message should be updated
		if errWithMessage.Message != newMessage {
			t.Fatal("PROPERTY VIOLATION: Message should be updated")
		}

		// Property: HTTP status should be preserved
		if errWithMessage.HTTPStatus != originalErr.HTTPStatus {
			t.Fatal("PROPERTY VIOLATION: HTTP status should be preserved")
		}

		// Property: Timestamp should be set
		if errWithMessage.Timestamp.IsZero() {
			t.Fatal("PROPERTY VIOLATION: Timestamp should be set")
		}
	})
}

// TestProperty_ErrorResponse_RateLimitError tests rate limit error creation
// *For any* rate limit error, the retry_after_seconds SHALL be included in details.
// **Validates: Requirements A6.2, A6.6**
func TestProperty_ErrorResponse_RateLimitError(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate random retry after value
		retryAfter := rapid.Int64Range(1, 3600).Draw(rt, "retryAfter")

		err := NewRateLimitError(retryAfter)

		// Property: Code should be rate limited
		if err.Code != ErrRateLimited {
			t.Fatal("PROPERTY VIOLATION: Code should be ErrRateLimited")
		}

		// Property: HTTP status should be 429
		if err.HTTPStatus != http.StatusTooManyRequests {
			t.Fatal("PROPERTY VIOLATION: HTTP status should be 429")
		}

		// Property: Details should contain retry_after_seconds
		details, ok := err.Details.(map[string]int64)
		if !ok {
			t.Fatal("PROPERTY VIOLATION: Details should be map[string]int64")
		}

		if details["retry_after_seconds"] != retryAfter {
			t.Fatalf("PROPERTY VIOLATION: retry_after_seconds should be %d, got %d",
				retryAfter, details["retry_after_seconds"])
		}
	})
}

// TestProperty_ErrorResponse_UpstreamError tests upstream error creation
// *For any* upstream error, the provider and status_code SHALL be included in details.
// **Validates: Requirements A6.6**
func TestProperty_ErrorResponse_UpstreamError(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate random provider and status code
		providers := []string{"openai", "anthropic", "google"}
		providerIdx := rapid.IntRange(0, len(providers)-1).Draw(rt, "providerIdx")
		provider := providers[providerIdx]
		statusCode := rapid.IntRange(500, 599).Draw(rt, "statusCode")

		err := NewUpstreamError(provider, statusCode)

		// Property: Code should be upstream error
		if err.Code != ErrUpstreamError {
			t.Fatal("PROPERTY VIOLATION: Code should be ErrUpstreamError")
		}

		// Property: HTTP status should be 502
		if err.HTTPStatus != http.StatusBadGateway {
			t.Fatal("PROPERTY VIOLATION: HTTP status should be 502")
		}

		// Property: Details should contain provider and status_code
		details, ok := err.Details.(map[string]interface{})
		if !ok {
			t.Fatal("PROPERTY VIOLATION: Details should be map[string]interface{}")
		}

		if details["provider"] != provider {
			t.Fatalf("PROPERTY VIOLATION: provider should be %s, got %v", provider, details["provider"])
		}

		if details["status_code"] != statusCode {
			t.Fatalf("PROPERTY VIOLATION: status_code should be %d, got %v", statusCode, details["status_code"])
		}
	})
}
