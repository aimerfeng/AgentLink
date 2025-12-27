package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aimerfeng/AgentLink/internal/config"
	"github.com/aimerfeng/AgentLink/internal/middleware"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// Helper function to create a test JWT token
func createTestJWTToken(secret string, userID, userType, email string, subject string, expiry time.Duration) string {
	now := time.Now()
	claims := &middleware.Claims{
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

// TestAuthEndpoints_Checkpoint verifies the authentication system endpoints
// This is a checkpoint test to ensure the auth system is properly configured
func TestAuthEndpoints_Checkpoint(t *testing.T) {
	secret := "test-secret-key-for-jwt-testing-32chars"
	cfg := &config.JWTConfig{
		Secret:             secret,
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: 7 * 24 * time.Hour,
		Issuer:             "agentlink",
	}

	authenticator := middleware.NewJWTAuthenticator(cfg)

	// Create test router with auth middleware
	router := gin.New()
	router.Use(middleware.RequestID())

	// Public routes
	router.POST("/api/v1/auth/register", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "register endpoint accessible"})
	})
	router.POST("/api/v1/auth/login", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "login endpoint accessible"})
	})
	router.POST("/api/v1/auth/refresh", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "refresh endpoint accessible"})
	})

	// Protected routes
	protected := router.Group("/api/v1")
	protected.Use(authenticator.JWTAuth())
	{
		protected.GET("/creators/me", func(c *gin.Context) {
			userID := middleware.GetUserIDFromContext(c)
			c.JSON(http.StatusOK, gin.H{"user_id": userID})
		})
		protected.GET("/developers/me", func(c *gin.Context) {
			userID := middleware.GetUserIDFromContext(c)
			c.JSON(http.StatusOK, gin.H{"user_id": userID})
		})
	}

	// Test 1: Public endpoints should be accessible without auth
	t.Run("PublicEndpoints_Accessible", func(t *testing.T) {
		endpoints := []string{
			"/api/v1/auth/register",
			"/api/v1/auth/login",
			"/api/v1/auth/refresh",
		}

		for _, endpoint := range endpoints {
			req := httptest.NewRequest("POST", endpoint, bytes.NewBuffer([]byte("{}")))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Public endpoint %s should be accessible, got status %d", endpoint, w.Code)
			}
		}
	})

	// Test 2: Protected endpoints should reject requests without auth
	t.Run("ProtectedEndpoints_RejectWithoutAuth", func(t *testing.T) {
		endpoints := []string{
			"/api/v1/creators/me",
			"/api/v1/developers/me",
		}

		for _, endpoint := range endpoints {
			req := httptest.NewRequest("GET", endpoint, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != http.StatusUnauthorized {
				t.Errorf("Protected endpoint %s should return 401 without auth, got %d", endpoint, w.Code)
			}
		}
	})

	// Test 3: Protected endpoints should accept valid tokens
	t.Run("ProtectedEndpoints_AcceptValidToken", func(t *testing.T) {
		token := createTestJWTToken(secret, "user-123", "creator", "test@example.com", "access", 15*time.Minute)

		req := httptest.NewRequest("GET", "/api/v1/creators/me", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Protected endpoint should accept valid token, got status %d", w.Code)
		}

		// Verify user ID is extracted correctly
		var response map[string]string
		json.Unmarshal(w.Body.Bytes(), &response)
		if response["user_id"] != "user-123" {
			t.Errorf("Expected user_id 'user-123', got '%s'", response["user_id"])
		}
	})

	// Test 4: Protected endpoints should reject expired tokens
	t.Run("ProtectedEndpoints_RejectExpiredToken", func(t *testing.T) {
		token := createTestJWTToken(secret, "user-123", "creator", "test@example.com", "access", -1*time.Hour)

		req := httptest.NewRequest("GET", "/api/v1/creators/me", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Protected endpoint should reject expired token, got status %d", w.Code)
		}
	})

	// Test 5: Protected endpoints should reject invalid tokens
	t.Run("ProtectedEndpoints_RejectInvalidToken", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/creators/me", nil)
		req.Header.Set("Authorization", "Bearer invalid.token.here")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Protected endpoint should reject invalid token, got status %d", w.Code)
		}
	})

	// Test 6: Protected endpoints should reject refresh tokens used as access tokens
	t.Run("ProtectedEndpoints_RejectRefreshToken", func(t *testing.T) {
		token := createTestJWTToken(secret, "user-123", "creator", "test@example.com", "refresh", 7*24*time.Hour)

		req := httptest.NewRequest("GET", "/api/v1/creators/me", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Protected endpoint should reject refresh token, got status %d", w.Code)
		}
	})

	// Test 7: Request ID should be present in responses
	t.Run("RequestID_Present", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewBuffer([]byte("{}")))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		requestID := w.Header().Get("X-Request-ID")
		if requestID == "" {
			t.Error("X-Request-ID header should be present in response")
		}
	})

	// Test 8: Custom Request ID should be preserved
	t.Run("RequestID_Preserved", func(t *testing.T) {
		customRequestID := "custom-request-id-123"
		req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewBuffer([]byte("{}")))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Request-ID", customRequestID)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		requestID := w.Header().Get("X-Request-ID")
		if requestID != customRequestID {
			t.Errorf("X-Request-ID should be preserved, expected '%s', got '%s'", customRequestID, requestID)
		}
	})
}

// TestRoleBasedAccess_Checkpoint verifies role-based access control
func TestRoleBasedAccess_Checkpoint(t *testing.T) {
	secret := "test-secret-key-for-jwt-testing-32chars"
	cfg := &config.JWTConfig{
		Secret:             secret,
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: 7 * 24 * time.Hour,
		Issuer:             "agentlink",
	}

	authenticator := middleware.NewJWTAuthenticator(cfg)

	// Create test router with role-based middleware
	router := gin.New()

	// Creator-only routes
	creatorRoutes := router.Group("/api/v1/creators")
	creatorRoutes.Use(authenticator.JWTAuth())
	creatorRoutes.Use(middleware.RequireCreator())
	{
		creatorRoutes.GET("/me", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"role": "creator"})
		})
	}

	// Developer-only routes
	developerRoutes := router.Group("/api/v1/developers")
	developerRoutes.Use(authenticator.JWTAuth())
	developerRoutes.Use(middleware.RequireDeveloper())
	{
		developerRoutes.GET("/me", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"role": "developer"})
		})
	}

	// Admin-only routes
	adminRoutes := router.Group("/api/v1/admin")
	adminRoutes.Use(authenticator.JWTAuth())
	adminRoutes.Use(middleware.RequireAdmin())
	{
		adminRoutes.GET("/stats", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"role": "admin"})
		})
	}

	// Test 1: Creator can access creator routes
	t.Run("Creator_CanAccessCreatorRoutes", func(t *testing.T) {
		token := createTestJWTToken(secret, "creator-123", "creator", "creator@example.com", "access", 15*time.Minute)

		req := httptest.NewRequest("GET", "/api/v1/creators/me", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Creator should access creator routes, got status %d", w.Code)
		}
	})

	// Test 2: Creator cannot access developer routes
	t.Run("Creator_CannotAccessDeveloperRoutes", func(t *testing.T) {
		token := createTestJWTToken(secret, "creator-123", "creator", "creator@example.com", "access", 15*time.Minute)

		req := httptest.NewRequest("GET", "/api/v1/developers/me", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("Creator should not access developer routes, got status %d", w.Code)
		}
	})

	// Test 3: Developer can access developer routes
	t.Run("Developer_CanAccessDeveloperRoutes", func(t *testing.T) {
		token := createTestJWTToken(secret, "developer-123", "developer", "developer@example.com", "access", 15*time.Minute)

		req := httptest.NewRequest("GET", "/api/v1/developers/me", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Developer should access developer routes, got status %d", w.Code)
		}
	})

	// Test 4: Developer cannot access creator routes
	t.Run("Developer_CannotAccessCreatorRoutes", func(t *testing.T) {
		token := createTestJWTToken(secret, "developer-123", "developer", "developer@example.com", "access", 15*time.Minute)

		req := httptest.NewRequest("GET", "/api/v1/creators/me", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("Developer should not access creator routes, got status %d", w.Code)
		}
	})

	// Test 5: Admin can access admin routes
	t.Run("Admin_CanAccessAdminRoutes", func(t *testing.T) {
		token := createTestJWTToken(secret, "admin-123", "admin", "admin@example.com", "access", 15*time.Minute)

		req := httptest.NewRequest("GET", "/api/v1/admin/stats", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Admin should access admin routes, got status %d", w.Code)
		}
	})

	// Test 6: Non-admin cannot access admin routes
	t.Run("NonAdmin_CannotAccessAdminRoutes", func(t *testing.T) {
		token := createTestJWTToken(secret, "creator-123", "creator", "creator@example.com", "access", 15*time.Minute)

		req := httptest.NewRequest("GET", "/api/v1/admin/stats", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("Non-admin should not access admin routes, got status %d", w.Code)
		}
	})
}

// TestTokenValidation_Checkpoint verifies token validation behavior
func TestTokenValidation_Checkpoint(t *testing.T) {
	secret := "test-secret-key-for-jwt-testing-32chars"
	cfg := &config.JWTConfig{
		Secret:             secret,
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: 7 * 24 * time.Hour,
		Issuer:             "agentlink",
	}

	authenticator := middleware.NewJWTAuthenticator(cfg)

	// Test 1: Valid access token should be validated
	t.Run("ValidAccessToken_Validated", func(t *testing.T) {
		token := createTestJWTToken(secret, "user-123", "creator", "test@example.com", "access", 15*time.Minute)

		claims, err := authenticator.ValidateAccessToken(token)
		if err != nil {
			t.Errorf("Valid access token should be validated, got error: %v", err)
		}
		if claims.UserID != "user-123" {
			t.Errorf("Expected UserID 'user-123', got '%s'", claims.UserID)
		}
		if claims.UserType != "creator" {
			t.Errorf("Expected UserType 'creator', got '%s'", claims.UserType)
		}
		if claims.Email != "test@example.com" {
			t.Errorf("Expected Email 'test@example.com', got '%s'", claims.Email)
		}
	})

	// Test 2: Expired token should be rejected
	t.Run("ExpiredToken_Rejected", func(t *testing.T) {
		token := createTestJWTToken(secret, "user-123", "creator", "test@example.com", "access", -1*time.Hour)

		_, err := authenticator.ValidateAccessToken(token)
		if err == nil {
			t.Error("Expired token should be rejected")
		}
	})

	// Test 3: Token with wrong secret should be rejected
	t.Run("WrongSecret_Rejected", func(t *testing.T) {
		token := createTestJWTToken("wrong-secret", "user-123", "creator", "test@example.com", "access", 15*time.Minute)

		_, err := authenticator.ValidateAccessToken(token)
		if err == nil {
			t.Error("Token with wrong secret should be rejected")
		}
	})

	// Test 4: Refresh token should be rejected as access token
	t.Run("RefreshToken_RejectedAsAccess", func(t *testing.T) {
		token := createTestJWTToken(secret, "user-123", "creator", "test@example.com", "refresh", 7*24*time.Hour)

		_, err := authenticator.ValidateAccessToken(token)
		if err == nil {
			t.Error("Refresh token should be rejected as access token")
		}
	})

	// Test 5: Malformed token should be rejected
	t.Run("MalformedToken_Rejected", func(t *testing.T) {
		_, err := authenticator.ValidateAccessToken("not.a.valid.token")
		if err == nil {
			t.Error("Malformed token should be rejected")
		}
	})

	// Test 6: Empty token should be rejected
	t.Run("EmptyToken_Rejected", func(t *testing.T) {
		_, err := authenticator.ValidateAccessToken("")
		if err == nil {
			t.Error("Empty token should be rejected")
		}
	})
}
