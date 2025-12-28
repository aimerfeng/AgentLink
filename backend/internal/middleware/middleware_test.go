package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aimerfeng/AgentLink/internal/config"
	"github.com/aimerfeng/AgentLink/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// Helper function to create a test JWT token
func createTestToken(secret string, userID, userType, email string, subject string, expiry time.Duration) string {
	now := time.Now()
	claims := &Claims{
		UserID:   userID,
		UserType: userType,
		Email:    email,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   subject,
			Issuer:    "agentlink",
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(expiry)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString([]byte(secret))
	return tokenString
}

func TestJWTAuth_ValidToken(t *testing.T) {
	secret := "test-secret-key-for-jwt-testing"
	cfg := &config.JWTConfig{
		Secret:             secret,
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: 7 * 24 * time.Hour,
		Issuer:             "agentlink",
	}

	authenticator := NewJWTAuthenticator(cfg)

	// Create a valid access token
	token := createTestToken(secret, "user-123", "creator", "test@example.com", "access", 15*time.Minute)

	// Create test router
	router := gin.New()
	router.Use(authenticator.JWTAuth())
	router.GET("/protected", func(c *gin.Context) {
		userID := GetUserIDFromContext(c)
		userType := GetUserTypeFromContext(c)
		email := GetEmailFromContext(c)
		c.JSON(http.StatusOK, gin.H{
			"user_id":   userID,
			"user_type": userType,
			"email":     email,
		})
	})

	// Make request with valid token
	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestJWTAuth_MissingToken(t *testing.T) {
	cfg := &config.JWTConfig{
		Secret:             "test-secret",
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: 7 * 24 * time.Hour,
		Issuer:             "agentlink",
	}

	authenticator := NewJWTAuthenticator(cfg)

	router := gin.New()
	router.Use(authenticator.JWTAuth())
	router.GET("/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestJWTAuth_InvalidToken(t *testing.T) {
	cfg := &config.JWTConfig{
		Secret:             "test-secret",
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: 7 * 24 * time.Hour,
		Issuer:             "agentlink",
	}

	authenticator := NewJWTAuthenticator(cfg)

	router := gin.New()
	router.Use(authenticator.JWTAuth())
	router.GET("/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestJWTAuth_ExpiredToken(t *testing.T) {
	secret := "test-secret"
	cfg := &config.JWTConfig{
		Secret:             secret,
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: 7 * 24 * time.Hour,
		Issuer:             "agentlink",
	}

	authenticator := NewJWTAuthenticator(cfg)

	// Create an expired token
	token := createTestToken(secret, "user-123", "creator", "test@example.com", "access", -1*time.Hour)

	router := gin.New()
	router.Use(authenticator.JWTAuth())
	router.GET("/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestJWTAuth_RefreshTokenRejected(t *testing.T) {
	secret := "test-secret"
	cfg := &config.JWTConfig{
		Secret:             secret,
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: 7 * 24 * time.Hour,
		Issuer:             "agentlink",
	}

	authenticator := NewJWTAuthenticator(cfg)

	// Create a refresh token (should be rejected by JWTAuth which expects access tokens)
	token := createTestToken(secret, "user-123", "creator", "test@example.com", "refresh", 7*24*time.Hour)

	router := gin.New()
	router.Use(authenticator.JWTAuth())
	router.GET("/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestRequireRole_AllowedRole(t *testing.T) {
	secret := "test-secret"
	cfg := &config.JWTConfig{
		Secret:             secret,
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: 7 * 24 * time.Hour,
		Issuer:             "agentlink",
	}

	authenticator := NewJWTAuthenticator(cfg)
	token := createTestToken(secret, "user-123", "creator", "test@example.com", "access", 15*time.Minute)

	router := gin.New()
	router.Use(authenticator.JWTAuth())
	router.Use(RequireRole(models.UserTypeCreator))
	router.GET("/creator-only", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/creator-only", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestRequireRole_DeniedRole(t *testing.T) {
	secret := "test-secret"
	cfg := &config.JWTConfig{
		Secret:             secret,
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: 7 * 24 * time.Hour,
		Issuer:             "agentlink",
	}

	authenticator := NewJWTAuthenticator(cfg)
	// Create a developer token
	token := createTestToken(secret, "user-123", "developer", "test@example.com", "access", 15*time.Minute)

	router := gin.New()
	router.Use(authenticator.JWTAuth())
	router.Use(RequireRole(models.UserTypeCreator)) // Requires creator, but user is developer
	router.GET("/creator-only", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/creator-only", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", w.Code)
	}
}

func TestRequireRole_MultipleAllowedRoles(t *testing.T) {
	secret := "test-secret"
	cfg := &config.JWTConfig{
		Secret:             secret,
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: 7 * 24 * time.Hour,
		Issuer:             "agentlink",
	}

	authenticator := NewJWTAuthenticator(cfg)

	// Test with developer token
	developerToken := createTestToken(secret, "user-123", "developer", "test@example.com", "access", 15*time.Minute)

	router := gin.New()
	router.Use(authenticator.JWTAuth())
	router.Use(RequireRole(models.UserTypeCreator, models.UserTypeDeveloper)) // Allow both
	router.GET("/multi-role", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/multi-role", nil)
	req.Header.Set("Authorization", "Bearer "+developerToken)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestRequireAdmin(t *testing.T) {
	secret := "test-secret"
	cfg := &config.JWTConfig{
		Secret:             secret,
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: 7 * 24 * time.Hour,
		Issuer:             "agentlink",
	}

	authenticator := NewJWTAuthenticator(cfg)

	// Test with admin token
	adminToken := createTestToken(secret, "admin-123", "admin", "admin@example.com", "access", 15*time.Minute)

	router := gin.New()
	router.Use(authenticator.JWTAuth())
	router.Use(RequireAdmin())
	router.GET("/admin-only", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/admin-only", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Test with non-admin token
	creatorToken := createTestToken(secret, "user-123", "creator", "test@example.com", "access", 15*time.Minute)

	req2 := httptest.NewRequest("GET", "/admin-only", nil)
	req2.Header.Set("Authorization", "Bearer "+creatorToken)
	w2 := httptest.NewRecorder()

	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", w2.Code)
	}
}

func TestExtractBearerToken(t *testing.T) {
	tests := []struct {
		name       string
		authHeader string
		wantToken  string
		wantErr    bool
	}{
		{
			name:       "valid bearer token",
			authHeader: "Bearer abc123",
			wantToken:  "abc123",
			wantErr:    false,
		},
		{
			name:       "missing bearer prefix",
			authHeader: "abc123",
			wantToken:  "",
			wantErr:    true,
		},
		{
			name:       "empty header",
			authHeader: "",
			wantToken:  "",
			wantErr:    true,
		},
		{
			name:       "only bearer prefix",
			authHeader: "Bearer ",
			wantToken:  "",
			wantErr:    false,
		},
		{
			name:       "wrong prefix",
			authHeader: "Basic abc123",
			wantToken:  "",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := extractBearerToken(tt.authHeader)
			if (err != nil) != tt.wantErr {
				t.Errorf("extractBearerToken() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if token != tt.wantToken {
				t.Errorf("extractBearerToken() = %v, want %v", token, tt.wantToken)
			}
		})
	}
}

func TestContextHelpers(t *testing.T) {
	secret := "test-secret"
	cfg := &config.JWTConfig{
		Secret:             secret,
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: 7 * 24 * time.Hour,
		Issuer:             "agentlink",
	}

	authenticator := NewJWTAuthenticator(cfg)
	token := createTestToken(secret, "user-456", "developer", "dev@example.com", "access", 15*time.Minute)

	router := gin.New()
	router.Use(authenticator.JWTAuth())
	router.GET("/test", func(c *gin.Context) {
		userID := GetUserIDFromContext(c)
		userType := GetUserTypeFromContext(c)
		email := GetEmailFromContext(c)
		claims := GetClaimsFromContext(c)

		if userID != "user-456" {
			t.Errorf("Expected userID 'user-456', got '%s'", userID)
		}
		if userType != models.UserTypeDeveloper {
			t.Errorf("Expected userType 'developer', got '%s'", userType)
		}
		if email != "dev@example.com" {
			t.Errorf("Expected email 'dev@example.com', got '%s'", email)
		}
		if claims == nil {
			t.Error("Expected claims to be set")
		}

		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}


// Property tests for Correlation ID middleware
// **Validates: Requirements A6.6**

func TestProperty_CorrelationID_GeneratedWhenMissing(t *testing.T) {
	router := gin.New()
	router.Use(RequestID())
	router.Use(CorrelationID())
	router.GET("/test", func(c *gin.Context) {
		correlationID := GetCorrelationIDFromContext(c)
		c.JSON(http.StatusOK, gin.H{"correlation_id": correlationID})
	})

	// Make request without correlation ID header
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Property: Correlation ID should be generated
	correlationID := w.Header().Get("X-Correlation-ID")
	if correlationID == "" {
		t.Fatal("PROPERTY VIOLATION: Correlation ID should be generated when not provided")
	}

	// Property: Correlation ID should be a valid UUID format
	if len(correlationID) != 36 {
		t.Fatalf("PROPERTY VIOLATION: Correlation ID should be UUID format, got length %d", len(correlationID))
	}
}

func TestProperty_CorrelationID_PropagatedFromHeader(t *testing.T) {
	router := gin.New()
	router.Use(RequestID())
	router.Use(CorrelationID())
	router.GET("/test", func(c *gin.Context) {
		correlationID := GetCorrelationIDFromContext(c)
		c.JSON(http.StatusOK, gin.H{"correlation_id": correlationID})
	})

	// Make request with correlation ID header
	expectedCorrelationID := "test-correlation-id-12345"
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Correlation-ID", expectedCorrelationID)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Property: Correlation ID should be propagated from header
	correlationID := w.Header().Get("X-Correlation-ID")
	if correlationID != expectedCorrelationID {
		t.Fatalf("PROPERTY VIOLATION: Correlation ID should be propagated, expected %s, got %s",
			expectedCorrelationID, correlationID)
	}
}

func TestProperty_CorrelationID_FallsBackToRequestID(t *testing.T) {
	router := gin.New()
	router.Use(RequestID())
	router.Use(CorrelationID())

	var capturedRequestID string
	var capturedCorrelationID string

	router.GET("/test", func(c *gin.Context) {
		capturedRequestID = GetRequestIDFromContext(c)
		capturedCorrelationID = GetCorrelationIDFromContext(c)
		c.JSON(http.StatusOK, gin.H{})
	})

	// Make request without correlation ID but with request ID
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Property: When no correlation ID is provided, it should fall back to request ID
	if capturedCorrelationID != capturedRequestID {
		t.Fatalf("PROPERTY VIOLATION: Correlation ID should fall back to request ID, got correlation=%s, request=%s",
			capturedCorrelationID, capturedRequestID)
	}
}

func TestProperty_CorrelationID_SetInResponseHeader(t *testing.T) {
	router := gin.New()
	router.Use(RequestID())
	router.Use(CorrelationID())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{})
	})

	// Test with provided correlation ID
	providedID := "provided-correlation-id"
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Correlation-ID", providedID)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Property: Response should include X-Correlation-ID header
	responseCorrelationID := w.Header().Get("X-Correlation-ID")
	if responseCorrelationID == "" {
		t.Fatal("PROPERTY VIOLATION: Response should include X-Correlation-ID header")
	}
	if responseCorrelationID != providedID {
		t.Fatalf("PROPERTY VIOLATION: Response correlation ID should match provided, expected %s, got %s",
			providedID, responseCorrelationID)
	}
}

func TestProperty_CorrelationID_UniquePerRequest(t *testing.T) {
	router := gin.New()
	router.Use(RequestID())
	router.Use(CorrelationID())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{})
	})

	// Make multiple requests without correlation ID
	correlationIDs := make(map[string]bool)
	numRequests := 10

	for i := 0; i < numRequests; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		correlationID := w.Header().Get("X-Correlation-ID")
		if correlationID == "" {
			t.Fatal("PROPERTY VIOLATION: Correlation ID should be generated")
		}

		if correlationIDs[correlationID] {
			t.Fatalf("PROPERTY VIOLATION: Correlation ID should be unique, got duplicate: %s", correlationID)
		}
		correlationIDs[correlationID] = true
	}

	// Property: All correlation IDs should be unique
	if len(correlationIDs) != numRequests {
		t.Fatalf("PROPERTY VIOLATION: Expected %d unique correlation IDs, got %d",
			numRequests, len(correlationIDs))
	}
}

func TestProperty_RequestID_GeneratedWhenMissing(t *testing.T) {
	router := gin.New()
	router.Use(RequestID())
	router.GET("/test", func(c *gin.Context) {
		requestID := GetRequestIDFromContext(c)
		c.JSON(http.StatusOK, gin.H{"request_id": requestID})
	})

	// Make request without request ID header
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Property: Request ID should be generated
	requestID := w.Header().Get("X-Request-ID")
	if requestID == "" {
		t.Fatal("PROPERTY VIOLATION: Request ID should be generated when not provided")
	}

	// Property: Request ID should be a valid UUID format
	if len(requestID) != 36 {
		t.Fatalf("PROPERTY VIOLATION: Request ID should be UUID format, got length %d", len(requestID))
	}
}

func TestProperty_RequestID_PropagatedFromHeader(t *testing.T) {
	router := gin.New()
	router.Use(RequestID())
	router.GET("/test", func(c *gin.Context) {
		requestID := GetRequestIDFromContext(c)
		c.JSON(http.StatusOK, gin.H{"request_id": requestID})
	})

	// Make request with request ID header
	expectedRequestID := "test-request-id-12345"
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Request-ID", expectedRequestID)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Property: Request ID should be propagated from header
	requestID := w.Header().Get("X-Request-ID")
	if requestID != expectedRequestID {
		t.Fatalf("PROPERTY VIOLATION: Request ID should be propagated, expected %s, got %s",
			expectedRequestID, requestID)
	}
}
