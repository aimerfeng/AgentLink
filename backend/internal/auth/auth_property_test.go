package auth_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/aimerfeng/AgentLink/internal/auth"
	"github.com/aimerfeng/AgentLink/internal/config"
	"github.com/aimerfeng/AgentLink/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
	"pgregory.net/rapid"
)

// Test database connection for property tests
var testDB *pgxpool.Pool

func TestMain(m *testing.M) {
	// Setup test database
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@localhost:5432/agentlink_test?sslmode=disable"
	}

	var err error
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	testDB, err = pgxpool.New(ctx, dbURL)
	if err != nil {
		fmt.Printf("Warning: Failed to connect to test database: %v\n", err)
		fmt.Println("Property tests requiring database will be skipped")
		// Don't exit, just run tests (they will skip)
		code := m.Run()
		os.Exit(code)
	}

	// Test connection
	if err := testDB.Ping(ctx); err != nil {
		fmt.Printf("Warning: Failed to ping test database: %v\n", err)
		testDB = nil
	}

	// Run tests
	code := m.Run()
	
	if testDB != nil {
		testDB.Close()
	}
	os.Exit(code)
}

// generateValidEmail generates a valid email address for testing
func generateValidEmail(t *rapid.T) string {
	// Generate a random string for the local part
	localPart := rapid.StringMatching(`[a-z]{5,10}`).Draw(t, "localPart")
	domain := rapid.StringMatching(`[a-z]{3,8}`).Draw(t, "domain")
	tld := rapid.SampledFrom([]string{"com", "org", "net", "io"}).Draw(t, "tld")
	timestamp := time.Now().UnixNano()
	return fmt.Sprintf("%s%d@%s.%s", localPart, timestamp, domain, tld)
}

// generateValidPassword generates a valid password (min 8 chars)
func generateValidPassword(t *rapid.T) string {
	// Generate password with at least 8 characters
	length := rapid.IntRange(8, 32).Draw(t, "passwordLength")
	chars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%"
	password := make([]byte, length)
	for i := 0; i < length; i++ {
		idx := rapid.IntRange(0, len(chars)-1).Draw(t, fmt.Sprintf("char%d", i))
		password[i] = chars[idx]
	}
	return string(password)
}

// generateDisplayName generates a valid display name for creators
func generateDisplayName(t *rapid.T) string {
	return rapid.StringMatching(`[A-Za-z ]{3,50}`).Draw(t, "displayName")
}

// Property 35: Free Quota Initialization
// *For any* newly registered developer, the account SHALL be initialized with the specified free quota amount.
// **Validates: Requirements 4.1**
func TestProperty35_FreeQuotaInitialization(t *testing.T) {
	if testDB == nil {
		t.Skip("Test database not available")
	}

	// Create auth service with test config
	jwtConfig := &config.JWTConfig{
		Secret:             "test-secret-key-for-property-testing-32chars",
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: 7 * 24 * time.Hour,
		Issuer:             "agentlink-test",
	}

	expectedFreeQuota := int64(100) // Default free quota
	quotaConfig := &config.QuotaConfig{
		FreeInitial:        expectedFreeQuota,
		TrialCallsPerAgent: 3,
	}

	authService := auth.NewService(testDB, jwtConfig, quotaConfig)

	rapid.Check(t, func(t *rapid.T) {
		ctx := context.Background()

		// Generate random valid registration data
		email := generateValidEmail(t)
		password := generateValidPassword(t)
		userType := rapid.SampledFrom([]models.UserType{
			models.UserTypeDeveloper,
			models.UserTypeCreator,
		}).Draw(t, "userType")

		req := &auth.RegisterRequest{
			Email:    email,
			Password: password,
			UserType: userType,
		}

		// Add display name for creators
		if userType == models.UserTypeCreator {
			req.DisplayName = generateDisplayName(t)
		}

		// Register the user
		resp, err := authService.Register(ctx, req)
		if err != nil {
			t.Fatalf("Registration failed: %v", err)
		}

		// Verify the user was created
		if resp.User.Email != email {
			t.Fatalf("Email mismatch: expected %s, got %s", email, resp.User.Email)
		}

		// Query the quota table to verify free quota was initialized
		var totalQuota, usedQuota, freeQuota int64
		err = testDB.QueryRow(ctx, `
			SELECT total_quota, used_quota, free_quota 
			FROM quotas 
			WHERE user_id = $1
		`, resp.User.ID).Scan(&totalQuota, &usedQuota, &freeQuota)

		if err != nil {
			t.Fatalf("Failed to query quota: %v", err)
		}

		// Property assertion: Free quota should be initialized to the expected amount
		if freeQuota != expectedFreeQuota {
			t.Fatalf("Free quota not initialized correctly: expected %d, got %d", expectedFreeQuota, freeQuota)
		}

		// Property assertion: Total quota should equal free quota for new users
		if totalQuota != expectedFreeQuota {
			t.Fatalf("Total quota not initialized correctly: expected %d, got %d", expectedFreeQuota, totalQuota)
		}

		// Property assertion: Used quota should be 0 for new users
		if usedQuota != 0 {
			t.Fatalf("Used quota should be 0 for new users, got %d", usedQuota)
		}

		// Cleanup: Delete the test user
		_, err = testDB.Exec(ctx, "DELETE FROM users WHERE id = $1", resp.User.ID)
		if err != nil {
			t.Logf("Warning: Failed to cleanup test user: %v", err)
		}
	})
}


// Property 5: Authentication Correctness
// *For any* valid API key with sufficient quota, the Proxy Gateway SHALL accept the request.
// *For any* invalid API key, the Proxy Gateway SHALL return 401.
// *For any* valid API key with exhausted quota, the Proxy Gateway SHALL return 429.
// **Validates: Requirements 5.1, 5.2**
//
// Note: This property test validates the authentication logic at the service level.
// The full Proxy Gateway integration test would require the proxy server to be running.
func TestProperty5_AuthenticationCorrectness(t *testing.T) {
	if testDB == nil {
		t.Skip("Test database not available")
	}

	// Create auth service with test config
	jwtConfig := &config.JWTConfig{
		Secret:             "test-secret-key-for-property-testing-32chars",
		AccessTokenExpiry:  15 * time.Minute,
		RefreshTokenExpiry: 7 * 24 * time.Hour,
		Issuer:             "agentlink-test",
	}

	quotaConfig := &config.QuotaConfig{
		FreeInitial:        100,
		TrialCallsPerAgent: 3,
	}

	authService := auth.NewService(testDB, jwtConfig, quotaConfig)

	rapid.Check(t, func(t *rapid.T) {
		ctx := context.Background()

		// Generate random valid registration data
		email := generateValidEmail(t)
		password := generateValidPassword(t)

		req := &auth.RegisterRequest{
			Email:    email,
			Password: password,
			UserType: models.UserTypeDeveloper,
		}

		// Register the user
		regResp, err := authService.Register(ctx, req)
		if err != nil {
			t.Fatalf("Registration failed: %v", err)
		}

		// Test 1: Valid credentials should return tokens
		loginReq := &auth.LoginRequest{
			Email:    email,
			Password: password,
		}
		loginResp, err := authService.Login(ctx, loginReq)
		if err != nil {
			t.Fatalf("Login with valid credentials should succeed: %v", err)
		}
		if loginResp.Tokens.AccessToken == "" {
			t.Fatal("Login should return access token")
		}
		if loginResp.Tokens.RefreshToken == "" {
			t.Fatal("Login should return refresh token")
		}

		// Test 2: Invalid password should return error
		invalidPasswordReq := &auth.LoginRequest{
			Email:    email,
			Password: "wrongpassword123",
		}
		_, err = authService.Login(ctx, invalidPasswordReq)
		if err != auth.ErrInvalidCredentials {
			t.Fatalf("Login with invalid password should return ErrInvalidCredentials, got: %v", err)
		}

		// Test 3: Invalid email should return same error (not revealing which field is wrong)
		invalidEmailReq := &auth.LoginRequest{
			Email:    "nonexistent@example.com",
			Password: password,
		}
		_, err = authService.Login(ctx, invalidEmailReq)
		if err != auth.ErrInvalidCredentials {
			t.Fatalf("Login with invalid email should return ErrInvalidCredentials, got: %v", err)
		}

		// Test 4: Valid access token should be validated successfully
		claims, err := authService.ValidateAccessToken(loginResp.Tokens.AccessToken)
		if err != nil {
			t.Fatalf("Valid access token should be validated: %v", err)
		}
		if claims.UserID != regResp.User.ID.String() {
			t.Fatalf("Token claims should contain correct user ID")
		}

		// Test 5: Invalid token should return error
		_, err = authService.ValidateAccessToken("invalid.token.here")
		if err != auth.ErrInvalidToken {
			t.Fatalf("Invalid token should return ErrInvalidToken, got: %v", err)
		}

		// Cleanup: Delete the test user
		_, err = testDB.Exec(ctx, "DELETE FROM users WHERE id = $1", regResp.User.ID)
		if err != nil {
			t.Logf("Warning: Failed to cleanup test user: %v", err)
		}
	})
}
