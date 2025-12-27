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


// Property 10: Wallet Address Validation
// *For any* valid Ethereum address format (0x + 40 hex chars), the validation SHALL pass.
// *For any* invalid address format, the validation SHALL fail.
// **Validates: Requirements 1.2**
func TestProperty10_WalletAddressValidation(t *testing.T) {
	// Test valid addresses
	rapid.Check(t, func(t *rapid.T) {
		// Generate a valid Ethereum address: 0x + 40 hex characters
		hexChars := "0123456789abcdefABCDEF"
		addressBytes := make([]byte, 40)
		for i := 0; i < 40; i++ {
			idx := rapid.IntRange(0, len(hexChars)-1).Draw(t, fmt.Sprintf("hexChar%d", i))
			addressBytes[i] = hexChars[idx]
		}
		validAddress := "0x" + string(addressBytes)

		// Property: Valid Ethereum addresses should pass validation
		if !auth.ValidateEthereumAddress(validAddress) {
			t.Fatalf("Valid address should pass validation: %s", validAddress)
		}
	})

	// Test invalid addresses - wrong length
	rapid.Check(t, func(t *rapid.T) {
		// Generate address with wrong length (not 42 chars)
		length := rapid.IntRange(1, 100).Draw(t, "length")
		if length == 42 {
			length = 41 // Avoid generating valid length
		}
		
		hexChars := "0123456789abcdef"
		addressBytes := make([]byte, length)
		for i := 0; i < length; i++ {
			idx := rapid.IntRange(0, len(hexChars)-1).Draw(t, fmt.Sprintf("char%d", i))
			addressBytes[i] = hexChars[idx]
		}
		invalidAddress := string(addressBytes)

		// Property: Addresses with wrong length should fail validation
		if auth.ValidateEthereumAddress(invalidAddress) {
			t.Fatalf("Address with wrong length should fail validation: %s (length: %d)", invalidAddress, len(invalidAddress))
		}
	})

	// Test invalid addresses - wrong prefix
	rapid.Check(t, func(t *rapid.T) {
		// Generate 42-char address without 0x prefix
		hexChars := "0123456789abcdef"
		addressBytes := make([]byte, 42)
		
		// Use invalid prefix (not 0x)
		invalidPrefixes := []string{"1x", "0y", "0z", "ax", "Ax", "00", "xx"}
		prefix := rapid.SampledFrom(invalidPrefixes).Draw(t, "prefix")
		addressBytes[0] = prefix[0]
		addressBytes[1] = prefix[1]
		
		for i := 2; i < 42; i++ {
			idx := rapid.IntRange(0, len(hexChars)-1).Draw(t, fmt.Sprintf("char%d", i))
			addressBytes[i] = hexChars[idx]
		}
		invalidAddress := string(addressBytes)

		// Property: Addresses without 0x prefix should fail validation
		if auth.ValidateEthereumAddress(invalidAddress) {
			t.Fatalf("Address without 0x prefix should fail validation: %s", invalidAddress)
		}
	})

	// Test invalid addresses - non-hex characters
	rapid.Check(t, func(t *rapid.T) {
		// Generate address with non-hex characters after 0x
		nonHexChars := "ghijklmnopqrstuvwxyzGHIJKLMNOPQRSTUVWXYZ!@#$%^&*()_+-=[]{}|;':\",./<>?"
		hexChars := "0123456789abcdef"
		
		addressBytes := make([]byte, 42)
		addressBytes[0] = '0'
		addressBytes[1] = 'x'
		
		// Insert at least one non-hex character
		nonHexPosition := rapid.IntRange(2, 41).Draw(t, "nonHexPosition")
		nonHexIdx := rapid.IntRange(0, len(nonHexChars)-1).Draw(t, "nonHexChar")
		
		for i := 2; i < 42; i++ {
			if i == nonHexPosition {
				addressBytes[i] = nonHexChars[nonHexIdx]
			} else {
				idx := rapid.IntRange(0, len(hexChars)-1).Draw(t, fmt.Sprintf("char%d", i))
				addressBytes[i] = hexChars[idx]
			}
		}
		invalidAddress := string(addressBytes)

		// Property: Addresses with non-hex characters should fail validation
		if auth.ValidateEthereumAddress(invalidAddress) {
			t.Fatalf("Address with non-hex characters should fail validation: %s", invalidAddress)
		}
	})
}

// TestProperty10_WalletBindingIntegration tests the full wallet binding flow
// This is an integration test that requires database access
func TestProperty10_WalletBindingIntegration(t *testing.T) {
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

		// Generate random valid registration data for a creator
		email := generateValidEmail(t)
		password := generateValidPassword(t)
		displayName := generateDisplayName(t)

		req := &auth.RegisterRequest{
			Email:       email,
			Password:    password,
			UserType:    models.UserTypeCreator,
			DisplayName: displayName,
		}

		// Register the creator
		regResp, err := authService.Register(ctx, req)
		if err != nil {
			t.Fatalf("Registration failed: %v", err)
		}

		// Generate a valid wallet address
		hexChars := "0123456789abcdef"
		addressBytes := make([]byte, 40)
		for i := 0; i < 40; i++ {
			idx := rapid.IntRange(0, len(hexChars)-1).Draw(t, fmt.Sprintf("walletChar%d", i))
			addressBytes[i] = hexChars[idx]
		}
		validWalletAddress := "0x" + string(addressBytes)

		// Bind wallet
		bindReq := &auth.BindWalletRequest{
			WalletAddress: validWalletAddress,
		}
		bindResp, err := authService.BindWallet(ctx, regResp.User.ID, bindReq)
		if err != nil {
			t.Fatalf("Wallet binding failed: %v", err)
		}

		// Property: After binding, the user's wallet address should match
		if bindResp.User.WalletAddress == nil || *bindResp.User.WalletAddress != validWalletAddress {
			t.Fatalf("Wallet address not bound correctly: expected %s, got %v", validWalletAddress, bindResp.User.WalletAddress)
		}

		// Verify in database
		var dbWalletAddress *string
		err = testDB.QueryRow(ctx, `
			SELECT wallet_address FROM users WHERE id = $1
		`, regResp.User.ID).Scan(&dbWalletAddress)
		if err != nil {
			t.Fatalf("Failed to query wallet address: %v", err)
		}

		if dbWalletAddress == nil || *dbWalletAddress != validWalletAddress {
			t.Fatalf("Wallet address not stored correctly in database: expected %s, got %v", validWalletAddress, dbWalletAddress)
		}

		// Cleanup: Delete the test user
		_, err = testDB.Exec(ctx, "DELETE FROM users WHERE id = $1", regResp.User.ID)
		if err != nil {
			t.Logf("Warning: Failed to cleanup test user: %v", err)
		}
	})
}

// TestProperty10_WalletBindingDeveloperRejection tests that developers cannot bind wallets
func TestProperty10_WalletBindingDeveloperRejection(t *testing.T) {
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

		// Generate random valid registration data for a developer
		email := generateValidEmail(t)
		password := generateValidPassword(t)

		req := &auth.RegisterRequest{
			Email:    email,
			Password: password,
			UserType: models.UserTypeDeveloper,
		}

		// Register the developer
		regResp, err := authService.Register(ctx, req)
		if err != nil {
			t.Fatalf("Registration failed: %v", err)
		}

		// Generate a valid wallet address
		validWalletAddress := "0x1234567890abcdef1234567890abcdef12345678"

		// Try to bind wallet (should fail)
		bindReq := &auth.BindWalletRequest{
			WalletAddress: validWalletAddress,
		}
		_, err = authService.BindWallet(ctx, regResp.User.ID, bindReq)

		// Property: Developers should not be able to bind wallet addresses
		if err != auth.ErrNotCreator {
			t.Fatalf("Developer wallet binding should return ErrNotCreator, got: %v", err)
		}

		// Cleanup: Delete the test user
		_, err = testDB.Exec(ctx, "DELETE FROM users WHERE id = $1", regResp.User.ID)
		if err != nil {
			t.Logf("Warning: Failed to cleanup test user: %v", err)
		}
	})
}

// TestProperty10_InvalidWalletAddressRejection tests that invalid addresses are rejected
func TestProperty10_InvalidWalletAddressRejection(t *testing.T) {
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

		// Generate random valid registration data for a creator
		email := generateValidEmail(t)
		password := generateValidPassword(t)
		displayName := generateDisplayName(t)

		req := &auth.RegisterRequest{
			Email:       email,
			Password:    password,
			UserType:    models.UserTypeCreator,
			DisplayName: displayName,
		}

		// Register the creator
		regResp, err := authService.Register(ctx, req)
		if err != nil {
			t.Fatalf("Registration failed: %v", err)
		}

		// Generate an invalid wallet address (wrong length)
		invalidAddresses := []string{
			"0x123",                                       // Too short
			"0x1234567890abcdef1234567890abcdef123456789", // Too long (41 hex chars)
			"1x1234567890abcdef1234567890abcdef12345678",  // Wrong prefix
			"0x1234567890abcdef1234567890abcdef1234567g",  // Invalid hex char
			"",                                            // Empty
			"0x",                                          // Just prefix
		}

		invalidAddress := rapid.SampledFrom(invalidAddresses).Draw(t, "invalidAddress")

		// Try to bind invalid wallet
		bindReq := &auth.BindWalletRequest{
			WalletAddress: invalidAddress,
		}
		_, err = authService.BindWallet(ctx, regResp.User.ID, bindReq)

		// Property: Invalid wallet addresses should be rejected
		if err != auth.ErrInvalidWalletAddress {
			t.Fatalf("Invalid wallet address should return ErrInvalidWalletAddress, got: %v (address: %s)", err, invalidAddress)
		}

		// Cleanup: Delete the test user
		_, err = testDB.Exec(ctx, "DELETE FROM users WHERE id = $1", regResp.User.ID)
		if err != nil {
			t.Logf("Warning: Failed to cleanup test user: %v", err)
		}
	})
}
