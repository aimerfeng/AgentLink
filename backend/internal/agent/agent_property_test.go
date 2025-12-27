package agent_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/aimerfeng/AgentLink/internal/agent"
	"github.com/aimerfeng/AgentLink/internal/auth"
	"github.com/aimerfeng/AgentLink/internal/config"
	agentmodels "github.com/aimerfeng/AgentLink/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
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

// Helper functions for generating test data

// generateValidEmail generates a valid email address for testing
func generateValidEmail(t *rapid.T) string {
	localPart := rapid.StringMatching(`[a-z]{5,10}`).Draw(t, "localPart")
	domain := rapid.StringMatching(`[a-z]{3,8}`).Draw(t, "domain")
	tld := rapid.SampledFrom([]string{"com", "org", "net", "io"}).Draw(t, "tld")
	timestamp := time.Now().UnixNano()
	return fmt.Sprintf("%s%d@%s.%s", localPart, timestamp, domain, tld)
}

// generateValidPassword generates a valid password (min 8 chars)
func generateValidPassword(t *rapid.T) string {
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

// generateAgentName generates a valid agent name
func generateAgentName(t *rapid.T) string {
	return rapid.StringMatching(`[A-Za-z0-9 ]{3,50}`).Draw(t, "agentName")
}

// generateSystemPrompt generates a valid system prompt
func generateSystemPrompt(t *rapid.T) string {
	return rapid.StringMatching(`[A-Za-z0-9 .,!?]{10,200}`).Draw(t, "systemPrompt")
}

// generateValidPrice generates a valid price between $0.001 and $100
func generateValidPrice(t *rapid.T) decimal.Decimal {
	// Generate price in millicents to avoid floating point issues
	millicents := rapid.Int64Range(1, 100000).Draw(t, "priceMillicents")
	return decimal.NewFromInt(millicents).Div(decimal.NewFromInt(1000))
}

// generateValidAgentConfig generates a valid agent configuration
func generateValidAgentConfig(t *rapid.T) agentmodels.AgentConfig {
	providers := []string{"openai", "anthropic", "google"}
	models := map[string][]string{
		"openai":    {"gpt-4", "gpt-4-turbo", "gpt-3.5-turbo"},
		"anthropic": {"claude-3-opus", "claude-3-sonnet", "claude-3-haiku"},
		"google":    {"gemini-pro", "gemini-ultra"},
	}

	provider := rapid.SampledFrom(providers).Draw(t, "provider")
	model := rapid.SampledFrom(models[provider]).Draw(t, "model")

	// Temperature: 0.0 - 2.0
	tempInt := rapid.IntRange(0, 200).Draw(t, "temperature")
	temperature := float64(tempInt) / 100.0

	// MaxTokens: 1 - 128000
	maxTokens := rapid.IntRange(100, 4096).Draw(t, "maxTokens")

	// TopP: 0.0 - 1.0
	topPInt := rapid.IntRange(0, 100).Draw(t, "topP")
	topP := float64(topPInt) / 100.0

	return agentmodels.AgentConfig{
		SystemPrompt: generateSystemPrompt(t),
		Model:        model,
		Provider:     provider,
		Temperature:  temperature,
		MaxTokens:    maxTokens,
		TopP:         topP,
	}
}

// createTestCreator creates a test creator and returns the user ID
func createTestCreator(ctx context.Context, t *rapid.T, authService *auth.Service) (uuid.UUID, func()) {
	email := generateValidEmail(t)
	password := generateValidPassword(t)
	displayName := generateDisplayName(t)

	req := &auth.RegisterRequest{
		Email:       email,
		Password:    password,
		UserType:    agentmodels.UserTypeCreator,
		DisplayName: displayName,
	}

	resp, err := authService.Register(ctx, req)
	if err != nil {
		panic(fmt.Sprintf("Failed to create test creator: %v", err))
	}

	cleanup := func() {
		_, _ = testDB.Exec(ctx, "DELETE FROM users WHERE id = $1", resp.User.ID)
	}

	return resp.User.ID, cleanup
}

// Property 2: ID Uniqueness
// *For any* two agents created, their IDs SHALL be unique.
// **Validates: Requirements 2.3, 4.2**
func TestProperty2_IDUniqueness(t *testing.T) {
	if testDB == nil {
		t.Skip("Test database not available")
	}

	// Create services
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

	encConfig := &config.EncryptionConfig{
		Key: "test-encryption-key-32-bytes-ok!",
	}

	authService := auth.NewService(testDB, jwtConfig, quotaConfig)
	agentService, err := agent.NewService(testDB, encConfig)
	if err != nil {
		t.Fatalf("Failed to create agent service: %v", err)
	}

	rapid.Check(t, func(t *rapid.T) {
		ctx := context.Background()

		// Create a test creator
		creatorID, cleanup := createTestCreator(ctx, t, authService)
		defer cleanup()

		// Track all created agent IDs
		agentIDs := make(map[uuid.UUID]bool)
		var agentsToCleanup []uuid.UUID

		// Create multiple agents
		numAgents := rapid.IntRange(2, 10).Draw(t, "numAgents")

		for i := 0; i < numAgents; i++ {
			req := &agent.CreateAgentRequest{
				Name:         generateAgentName(t) + fmt.Sprintf("_%d", i),
				Config:       generateValidAgentConfig(t),
				PricePerCall: generateValidPrice(t),
			}

			resp, err := agentService.Create(ctx, creatorID, req)
			if err != nil {
				t.Fatalf("Failed to create agent %d: %v", i, err)
			}

			agentsToCleanup = append(agentsToCleanup, resp.ID)

			// Property assertion: Each agent ID should be unique
			if agentIDs[resp.ID] {
				t.Fatalf("Duplicate agent ID detected: %s", resp.ID)
			}
			agentIDs[resp.ID] = true

			// Property assertion: Agent ID should be a valid UUID
			if resp.ID == uuid.Nil {
				t.Fatal("Agent ID should not be nil UUID")
			}
		}

		// Cleanup agents
		for _, agentID := range agentsToCleanup {
			_, _ = testDB.Exec(ctx, "DELETE FROM agents WHERE id = $1", agentID)
		}
	})
}


// Property 11: Price Validation
// *For any* price within the valid range ($0.001 - $100), the validation SHALL pass.
// *For any* price outside the valid range, the validation SHALL fail.
// **Validates: Requirements 2.4**
func TestProperty11_PriceValidation(t *testing.T) {
	// Test valid prices - prices within range should pass
	rapid.Check(t, func(t *rapid.T) {
		// Generate price in millicents to avoid floating point issues
		// Range: 1 millicent ($0.001) to 100000 millicents ($100)
		millicents := rapid.Int64Range(1, 100000).Draw(t, "priceMillicents")
		price := decimal.NewFromInt(millicents).Div(decimal.NewFromInt(1000))

		// Property: Valid prices should pass validation
		err := agent.ValidatePrice(price)
		if err != nil {
			t.Fatalf("Valid price %s should pass validation, got error: %v", price.String(), err)
		}
	})

	// Test boundary values
	t.Run("BoundaryValues", func(t *testing.T) {
		// Minimum valid price: $0.001
		minPrice := decimal.NewFromFloat(0.001)
		if err := agent.ValidatePrice(minPrice); err != nil {
			t.Fatalf("Minimum price $0.001 should be valid, got error: %v", err)
		}

		// Maximum valid price: $100
		maxPrice := decimal.NewFromFloat(100.0)
		if err := agent.ValidatePrice(maxPrice); err != nil {
			t.Fatalf("Maximum price $100 should be valid, got error: %v", err)
		}

		// Just below minimum: $0.0009
		belowMin := decimal.NewFromFloat(0.0009)
		if err := agent.ValidatePrice(belowMin); err != agent.ErrInvalidPrice {
			t.Fatalf("Price below minimum should return ErrInvalidPrice, got: %v", err)
		}

		// Just above maximum: $100.01
		aboveMax := decimal.NewFromFloat(100.01)
		if err := agent.ValidatePrice(aboveMax); err != agent.ErrInvalidPrice {
			t.Fatalf("Price above maximum should return ErrInvalidPrice, got: %v", err)
		}

		// Zero price
		zeroPrice := decimal.NewFromFloat(0)
		if err := agent.ValidatePrice(zeroPrice); err != agent.ErrInvalidPrice {
			t.Fatalf("Zero price should return ErrInvalidPrice, got: %v", err)
		}

		// Negative price
		negativePrice := decimal.NewFromFloat(-1.0)
		if err := agent.ValidatePrice(negativePrice); err != agent.ErrInvalidPrice {
			t.Fatalf("Negative price should return ErrInvalidPrice, got: %v", err)
		}
	})

	// Test invalid prices - prices below minimum
	rapid.Check(t, func(t *rapid.T) {
		// Generate price below minimum (0 to 0.0009)
		// Using microcents: 0 to 999 microcents = $0 to $0.000999
		microcents := rapid.Int64Range(0, 999).Draw(t, "priceMicrocents")
		price := decimal.NewFromInt(microcents).Div(decimal.NewFromInt(1000000))

		// Property: Prices below minimum should fail validation
		err := agent.ValidatePrice(price)
		if err != agent.ErrInvalidPrice {
			t.Fatalf("Price %s below minimum should return ErrInvalidPrice, got: %v", price.String(), err)
		}
	})

	// Test invalid prices - prices above maximum
	rapid.Check(t, func(t *rapid.T) {
		// Generate price above maximum ($100.01 to $10000)
		// Using cents: 10001 to 1000000 cents = $100.01 to $10000
		cents := rapid.Int64Range(10001, 1000000).Draw(t, "priceCents")
		price := decimal.NewFromInt(cents).Div(decimal.NewFromInt(100))

		// Property: Prices above maximum should fail validation
		err := agent.ValidatePrice(price)
		if err != agent.ErrInvalidPrice {
			t.Fatalf("Price %s above maximum should return ErrInvalidPrice, got: %v", price.String(), err)
		}
	})
}

// TestProperty11_PriceValidationIntegration tests price validation in agent creation
func TestProperty11_PriceValidationIntegration(t *testing.T) {
	if testDB == nil {
		t.Skip("Test database not available")
	}

	// Create services
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

	encConfig := &config.EncryptionConfig{
		Key: "test-encryption-key-32-bytes-ok!",
	}

	authService := auth.NewService(testDB, jwtConfig, quotaConfig)
	agentService, err := agent.NewService(testDB, encConfig)
	if err != nil {
		t.Fatalf("Failed to create agent service: %v", err)
	}

	rapid.Check(t, func(t *rapid.T) {
		ctx := context.Background()

		// Create a test creator
		creatorID, cleanup := createTestCreator(ctx, t, authService)
		defer cleanup()

		// Test with valid price
		validPrice := generateValidPrice(t)
		validReq := &agent.CreateAgentRequest{
			Name:         generateAgentName(t),
			Config:       generateValidAgentConfig(t),
			PricePerCall: validPrice,
		}

		resp, err := agentService.Create(ctx, creatorID, validReq)
		if err != nil {
			t.Fatalf("Agent creation with valid price %s should succeed: %v", validPrice.String(), err)
		}

		// Property: Created agent should have the correct price
		if !resp.PricePerCall.Equal(validPrice) {
			t.Fatalf("Agent price mismatch: expected %s, got %s", validPrice.String(), resp.PricePerCall.String())
		}

		// Cleanup
		_, _ = testDB.Exec(ctx, "DELETE FROM agents WHERE id = $1", resp.ID)
	})

	// Test with invalid prices
	rapid.Check(t, func(t *rapid.T) {
		ctx := context.Background()

		// Create a test creator
		creatorID, cleanup := createTestCreator(ctx, t, authService)
		defer cleanup()

		// Generate invalid price (either too low or too high)
		var invalidPrice decimal.Decimal
		if rapid.Bool().Draw(t, "tooLow") {
			// Too low: 0 to 0.0009
			microcents := rapid.Int64Range(0, 999).Draw(t, "priceMicrocents")
			invalidPrice = decimal.NewFromInt(microcents).Div(decimal.NewFromInt(1000000))
		} else {
			// Too high: $100.01 to $10000
			cents := rapid.Int64Range(10001, 1000000).Draw(t, "priceCents")
			invalidPrice = decimal.NewFromInt(cents).Div(decimal.NewFromInt(100))
		}

		invalidReq := &agent.CreateAgentRequest{
			Name:         generateAgentName(t),
			Config:       generateValidAgentConfig(t),
			PricePerCall: invalidPrice,
		}

		_, err := agentService.Create(ctx, creatorID, invalidReq)

		// Property: Agent creation with invalid price should fail
		if err != agent.ErrInvalidPrice {
			t.Fatalf("Agent creation with invalid price %s should return ErrInvalidPrice, got: %v", invalidPrice.String(), err)
		}
	})
}


// Property 19: Encryption Round-Trip
// *For any* valid agent configuration, encrypting then decrypting SHALL produce the original configuration.
// **Validates: Requirements 10.1**
func TestProperty19_EncryptionRoundTrip(t *testing.T) {
	// Create agent service with test encryption key
	encConfig := &config.EncryptionConfig{
		Key: "test-encryption-key-32-bytes-ok!",
	}

	// We need a mock DB pool for the service, but we only test encryption
	// So we'll create a minimal service just for encryption testing
	agentService, err := agent.NewService(nil, encConfig)
	if err != nil {
		t.Fatalf("Failed to create agent service: %v", err)
	}

	rapid.Check(t, func(t *rapid.T) {
		// Generate random plaintext data
		dataLength := rapid.IntRange(1, 10000).Draw(t, "dataLength")
		plaintext := make([]byte, dataLength)
		for i := 0; i < dataLength; i++ {
			plaintext[i] = byte(rapid.IntRange(0, 255).Draw(t, fmt.Sprintf("byte%d", i)))
		}

		// Encrypt
		ciphertext, nonce, err := agentService.Encrypt(plaintext)
		if err != nil {
			t.Fatalf("Encryption failed: %v", err)
		}

		// Property: Ciphertext should be different from plaintext
		if len(ciphertext) > 0 && len(plaintext) > 0 {
			// Check that at least some bytes are different (encryption happened)
			different := false
			minLen := len(plaintext)
			if len(ciphertext) < minLen {
				minLen = len(ciphertext)
			}
			for i := 0; i < minLen; i++ {
				if ciphertext[i] != plaintext[i] {
					different = true
					break
				}
			}
			if !different && len(ciphertext) == len(plaintext) {
				// This is extremely unlikely for random data, but check anyway
				t.Log("Warning: Ciphertext appears identical to plaintext")
			}
		}

		// Decrypt
		decrypted, err := agentService.Decrypt(ciphertext, nonce)
		if err != nil {
			t.Fatalf("Decryption failed: %v", err)
		}

		// Property: Decrypted data should match original plaintext
		if len(decrypted) != len(plaintext) {
			t.Fatalf("Decrypted length mismatch: expected %d, got %d", len(plaintext), len(decrypted))
		}
		for i := 0; i < len(plaintext); i++ {
			if decrypted[i] != plaintext[i] {
				t.Fatalf("Decrypted data mismatch at byte %d: expected %d, got %d", i, plaintext[i], decrypted[i])
			}
		}
	})
}

// TestProperty19_EncryptionRoundTripWithAgentConfig tests round-trip with actual agent configs
func TestProperty19_EncryptionRoundTripWithAgentConfig(t *testing.T) {
	if testDB == nil {
		t.Skip("Test database not available")
	}

	// Create services
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

	encConfig := &config.EncryptionConfig{
		Key: "test-encryption-key-32-bytes-ok!",
	}

	authService := auth.NewService(testDB, jwtConfig, quotaConfig)
	agentService, err := agent.NewService(testDB, encConfig)
	if err != nil {
		t.Fatalf("Failed to create agent service: %v", err)
	}

	rapid.Check(t, func(t *rapid.T) {
		ctx := context.Background()

		// Create a test creator
		creatorID, cleanup := createTestCreator(ctx, t, authService)
		defer cleanup()

		// Generate a random agent config
		originalConfig := generateValidAgentConfig(t)

		// Create agent with the config
		req := &agent.CreateAgentRequest{
			Name:         generateAgentName(t),
			Config:       originalConfig,
			PricePerCall: generateValidPrice(t),
		}

		resp, err := agentService.Create(ctx, creatorID, req)
		if err != nil {
			t.Fatalf("Agent creation failed: %v", err)
		}

		// Retrieve the agent and verify config is preserved
		retrievedResp, err := agentService.GetByIDForOwner(ctx, resp.ID, creatorID)
		if err != nil {
			t.Fatalf("Agent retrieval failed: %v", err)
		}

		// Property: Retrieved config should match original config
		if retrievedResp.Config == nil {
			t.Fatal("Retrieved config should not be nil")
		}

		if retrievedResp.Config.SystemPrompt != originalConfig.SystemPrompt {
			t.Fatalf("SystemPrompt mismatch: expected %q, got %q", originalConfig.SystemPrompt, retrievedResp.Config.SystemPrompt)
		}
		if retrievedResp.Config.Model != originalConfig.Model {
			t.Fatalf("Model mismatch: expected %q, got %q", originalConfig.Model, retrievedResp.Config.Model)
		}
		if retrievedResp.Config.Provider != originalConfig.Provider {
			t.Fatalf("Provider mismatch: expected %q, got %q", originalConfig.Provider, retrievedResp.Config.Provider)
		}
		if retrievedResp.Config.Temperature != originalConfig.Temperature {
			t.Fatalf("Temperature mismatch: expected %f, got %f", originalConfig.Temperature, retrievedResp.Config.Temperature)
		}
		if retrievedResp.Config.MaxTokens != originalConfig.MaxTokens {
			t.Fatalf("MaxTokens mismatch: expected %d, got %d", originalConfig.MaxTokens, retrievedResp.Config.MaxTokens)
		}
		if retrievedResp.Config.TopP != originalConfig.TopP {
			t.Fatalf("TopP mismatch: expected %f, got %f", originalConfig.TopP, retrievedResp.Config.TopP)
		}

		// Cleanup
		_, _ = testDB.Exec(ctx, "DELETE FROM agents WHERE id = $1", resp.ID)
	})
}

// TestProperty19_EncryptionDifferentNonces tests that different encryptions produce different ciphertexts
func TestProperty19_EncryptionDifferentNonces(t *testing.T) {
	encConfig := &config.EncryptionConfig{
		Key: "test-encryption-key-32-bytes-ok!",
	}

	agentService, err := agent.NewService(nil, encConfig)
	if err != nil {
		t.Fatalf("Failed to create agent service: %v", err)
	}

	rapid.Check(t, func(t *rapid.T) {
		// Generate random plaintext
		dataLength := rapid.IntRange(10, 1000).Draw(t, "dataLength")
		plaintext := make([]byte, dataLength)
		for i := 0; i < dataLength; i++ {
			plaintext[i] = byte(rapid.IntRange(0, 255).Draw(t, fmt.Sprintf("byte%d", i)))
		}

		// Encrypt twice
		ciphertext1, nonce1, err := agentService.Encrypt(plaintext)
		if err != nil {
			t.Fatalf("First encryption failed: %v", err)
		}

		ciphertext2, nonce2, err := agentService.Encrypt(plaintext)
		if err != nil {
			t.Fatalf("Second encryption failed: %v", err)
		}

		// Property: Different encryptions should produce different nonces
		noncesDifferent := false
		for i := 0; i < len(nonce1) && i < len(nonce2); i++ {
			if nonce1[i] != nonce2[i] {
				noncesDifferent = true
				break
			}
		}
		if !noncesDifferent && len(nonce1) == len(nonce2) {
			t.Fatal("Different encryptions should produce different nonces (extremely unlikely to be the same)")
		}

		// Property: Different nonces should produce different ciphertexts
		ciphertextsDifferent := false
		for i := 0; i < len(ciphertext1) && i < len(ciphertext2); i++ {
			if ciphertext1[i] != ciphertext2[i] {
				ciphertextsDifferent = true
				break
			}
		}
		if !ciphertextsDifferent && len(ciphertext1) == len(ciphertext2) {
			t.Fatal("Different encryptions should produce different ciphertexts")
		}

		// Property: Both should decrypt to the same plaintext
		decrypted1, err := agentService.Decrypt(ciphertext1, nonce1)
		if err != nil {
			t.Fatalf("First decryption failed: %v", err)
		}

		decrypted2, err := agentService.Decrypt(ciphertext2, nonce2)
		if err != nil {
			t.Fatalf("Second decryption failed: %v", err)
		}

		if len(decrypted1) != len(decrypted2) {
			t.Fatalf("Decrypted lengths should match: %d vs %d", len(decrypted1), len(decrypted2))
		}

		for i := 0; i < len(decrypted1); i++ {
			if decrypted1[i] != decrypted2[i] {
				t.Fatalf("Decrypted data should match at byte %d", i)
			}
		}
	})
}

// TestProperty19_EncryptionTamperDetection tests that tampered ciphertext is detected
func TestProperty19_EncryptionTamperDetection(t *testing.T) {
	encConfig := &config.EncryptionConfig{
		Key: "test-encryption-key-32-bytes-ok!",
	}

	agentService, err := agent.NewService(nil, encConfig)
	if err != nil {
		t.Fatalf("Failed to create agent service: %v", err)
	}

	rapid.Check(t, func(t *rapid.T) {
		// Generate random plaintext
		dataLength := rapid.IntRange(10, 1000).Draw(t, "dataLength")
		plaintext := make([]byte, dataLength)
		for i := 0; i < dataLength; i++ {
			plaintext[i] = byte(rapid.IntRange(0, 255).Draw(t, fmt.Sprintf("byte%d", i)))
		}

		// Encrypt
		ciphertext, nonce, err := agentService.Encrypt(plaintext)
		if err != nil {
			t.Fatalf("Encryption failed: %v", err)
		}

		// Tamper with ciphertext
		if len(ciphertext) > 0 {
			tamperedCiphertext := make([]byte, len(ciphertext))
			copy(tamperedCiphertext, ciphertext)
			
			// Flip a random bit
			tamperPos := rapid.IntRange(0, len(tamperedCiphertext)-1).Draw(t, "tamperPos")
			tamperedCiphertext[tamperPos] ^= 0x01

			// Property: Decryption of tampered ciphertext should fail
			_, err := agentService.Decrypt(tamperedCiphertext, nonce)
			if err == nil {
				t.Fatal("Decryption of tampered ciphertext should fail")
			}
		}
	})
}
