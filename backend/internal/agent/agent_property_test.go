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


// Property 8: Publish State Transition
// *For any* Agent, publishing SHALL change status to active, and the Agent SHALL become accessible via API.
// **Validates: Requirements 3.1**
func TestProperty8_PublishStateTransition(t *testing.T) {
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

		// Create an agent (starts in draft status)
		req := &agent.CreateAgentRequest{
			Name:         generateAgentName(t),
			Config:       generateValidAgentConfig(t),
			PricePerCall: generateValidPrice(t),
		}

		createResp, err := agentService.Create(ctx, creatorID, req)
		if err != nil {
			t.Fatalf("Failed to create agent: %v", err)
		}
		defer func() {
			_, _ = testDB.Exec(ctx, "DELETE FROM agents WHERE id = $1", createResp.ID)
		}()

		// Property: New agent should be in draft status
		if createResp.Status != agentmodels.AgentStatusDraft {
			t.Fatalf("New agent should be in draft status, got: %s", createResp.Status)
		}

		// Property: PublishedAt should be nil for draft agent
		if createResp.PublishedAt != nil {
			t.Fatal("Draft agent should not have PublishedAt set")
		}

		// Publish the agent
		publishResp, err := agentService.Publish(ctx, createResp.ID, creatorID)
		if err != nil {
			t.Fatalf("Failed to publish agent: %v", err)
		}

		// Property: After publishing, status should be active
		if publishResp.Status != agentmodels.AgentStatusActive {
			t.Fatalf("Published agent should have active status, got: %s", publishResp.Status)
		}

		// Property: PublishedAt should be set after publishing
		if publishResp.PublishedAt == nil {
			t.Fatal("Published agent should have PublishedAt set")
		}

		// Property: PublishedAt should be recent (within last minute)
		if time.Since(*publishResp.PublishedAt) > time.Minute {
			t.Fatal("PublishedAt should be recent")
		}

		// Property: Publishing an already active agent should return error
		_, err = agentService.Publish(ctx, createResp.ID, creatorID)
		if err != agent.ErrAgentAlreadyActive {
			t.Fatalf("Publishing already active agent should return ErrAgentAlreadyActive, got: %v", err)
		}

		// Verify agent is retrievable and has correct status
		retrievedAgent, err := agentService.GetByIDForOwner(ctx, createResp.ID, creatorID)
		if err != nil {
			t.Fatalf("Failed to retrieve published agent: %v", err)
		}

		// Property: Retrieved agent should have active status
		if retrievedAgent.Status != agentmodels.AgentStatusActive {
			t.Fatalf("Retrieved agent should have active status, got: %s", retrievedAgent.Status)
		}
	})
}

// TestProperty8_UnpublishStateTransition tests unpublishing an agent
func TestProperty8_UnpublishStateTransition(t *testing.T) {
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

		// Create and publish an agent
		req := &agent.CreateAgentRequest{
			Name:         generateAgentName(t),
			Config:       generateValidAgentConfig(t),
			PricePerCall: generateValidPrice(t),
		}

		createResp, err := agentService.Create(ctx, creatorID, req)
		if err != nil {
			t.Fatalf("Failed to create agent: %v", err)
		}
		defer func() {
			_, _ = testDB.Exec(ctx, "DELETE FROM agents WHERE id = $1", createResp.ID)
		}()

		// Publish the agent first
		_, err = agentService.Publish(ctx, createResp.ID, creatorID)
		if err != nil {
			t.Fatalf("Failed to publish agent: %v", err)
		}

		// Unpublish the agent
		unpublishResp, err := agentService.Unpublish(ctx, createResp.ID, creatorID)
		if err != nil {
			t.Fatalf("Failed to unpublish agent: %v", err)
		}

		// Property: After unpublishing, status should be inactive
		if unpublishResp.Status != agentmodels.AgentStatusInactive {
			t.Fatalf("Unpublished agent should have inactive status, got: %s", unpublishResp.Status)
		}

		// Verify agent is retrievable and has correct status
		retrievedAgent, err := agentService.GetByIDForOwner(ctx, createResp.ID, creatorID)
		if err != nil {
			t.Fatalf("Failed to retrieve unpublished agent: %v", err)
		}

		// Property: Retrieved agent should have inactive status
		if retrievedAgent.Status != agentmodels.AgentStatusInactive {
			t.Fatalf("Retrieved agent should have inactive status, got: %s", retrievedAgent.Status)
		}
	})
}

// TestProperty8_PublishOwnershipValidation tests that only owners can publish/unpublish
func TestProperty8_PublishOwnershipValidation(t *testing.T) {
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

		// Create two test creators
		creatorID1, cleanup1 := createTestCreator(ctx, t, authService)
		defer cleanup1()

		creatorID2, cleanup2 := createTestCreator(ctx, t, authService)
		defer cleanup2()

		// Create an agent owned by creator 1
		req := &agent.CreateAgentRequest{
			Name:         generateAgentName(t),
			Config:       generateValidAgentConfig(t),
			PricePerCall: generateValidPrice(t),
		}

		createResp, err := agentService.Create(ctx, creatorID1, req)
		if err != nil {
			t.Fatalf("Failed to create agent: %v", err)
		}
		defer func() {
			_, _ = testDB.Exec(ctx, "DELETE FROM agents WHERE id = $1", createResp.ID)
		}()

		// Property: Non-owner should not be able to publish
		_, err = agentService.Publish(ctx, createResp.ID, creatorID2)
		if err != agent.ErrAgentNotOwned {
			t.Fatalf("Non-owner publishing should return ErrAgentNotOwned, got: %v", err)
		}

		// Owner publishes the agent
		_, err = agentService.Publish(ctx, createResp.ID, creatorID1)
		if err != nil {
			t.Fatalf("Owner should be able to publish: %v", err)
		}

		// Property: Non-owner should not be able to unpublish
		_, err = agentService.Unpublish(ctx, createResp.ID, creatorID2)
		if err != agent.ErrAgentNotOwned {
			t.Fatalf("Non-owner unpublishing should return ErrAgentNotOwned, got: %v", err)
		}

		// Owner unpublishes the agent
		_, err = agentService.Unpublish(ctx, createResp.ID, creatorID1)
		if err != nil {
			t.Fatalf("Owner should be able to unpublish: %v", err)
		}
	})
}


// Property 9: Version Preservation
// *For any* Agent update, the previous version SHALL be preserved and retrievable, and the version number SHALL increment.
// **Validates: Requirements 3.2**
func TestProperty9_VersionPreservation(t *testing.T) {
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

		// Create an agent
		originalConfig := generateValidAgentConfig(t)
		originalName := generateAgentName(t)

		req := &agent.CreateAgentRequest{
			Name:         originalName,
			Config:       originalConfig,
			PricePerCall: generateValidPrice(t),
		}

		createResp, err := agentService.Create(ctx, creatorID, req)
		if err != nil {
			t.Fatalf("Failed to create agent: %v", err)
		}
		defer func() {
			// Cleanup agent and versions
			_, _ = testDB.Exec(ctx, "DELETE FROM agent_versions WHERE agent_id = $1", createResp.ID)
			_, _ = testDB.Exec(ctx, "DELETE FROM agents WHERE id = $1", createResp.ID)
		}()

		// Property: Initial version should be 1
		if createResp.Version != 1 {
			t.Fatalf("Initial version should be 1, got: %d", createResp.Version)
		}

		// Generate new config for update
		newConfig := generateValidAgentConfig(t)
		newName := generateAgentName(t) + "_updated"

		// Update the agent
		updateReq := &agent.UpdateAgentRequest{
			Name:   &newName,
			Config: &newConfig,
		}

		updateResp, err := agentService.Update(ctx, createResp.ID, creatorID, updateReq)
		if err != nil {
			t.Fatalf("Failed to update agent: %v", err)
		}

		// Property: Version should increment after update
		if updateResp.Version != 2 {
			t.Fatalf("Version should be 2 after first update, got: %d", updateResp.Version)
		}

		// Property: Updated agent should have new values
		if updateResp.Name != newName {
			t.Fatalf("Updated name mismatch: expected %q, got %q", newName, updateResp.Name)
		}
		if updateResp.Config.SystemPrompt != newConfig.SystemPrompt {
			t.Fatalf("Updated config mismatch")
		}

		// Get historical versions
		versionsResp, err := agentService.GetVersions(ctx, createResp.ID, creatorID)
		if err != nil {
			t.Fatalf("Failed to get versions: %v", err)
		}

		// Property: There should be exactly 1 historical version (version 1)
		if versionsResp.Total != 1 {
			t.Fatalf("Should have 1 historical version, got: %d", versionsResp.Total)
		}

		// Property: Historical version should have version number 1
		if versionsResp.Versions[0].Version != 1 {
			t.Fatalf("Historical version should be 1, got: %d", versionsResp.Versions[0].Version)
		}

		// Property: Historical version should preserve original config
		if versionsResp.Versions[0].Config.SystemPrompt != originalConfig.SystemPrompt {
			t.Fatalf("Historical version config mismatch: expected %q, got %q",
				originalConfig.SystemPrompt, versionsResp.Versions[0].Config.SystemPrompt)
		}

		// Get specific version
		versionResp, err := agentService.GetVersion(ctx, createResp.ID, creatorID, 1)
		if err != nil {
			t.Fatalf("Failed to get specific version: %v", err)
		}

		// Property: Retrieved version should match
		if versionResp.Version != 1 {
			t.Fatalf("Retrieved version should be 1, got: %d", versionResp.Version)
		}
		if versionResp.Config.SystemPrompt != originalConfig.SystemPrompt {
			t.Fatalf("Retrieved version config mismatch")
		}
	})
}

// TestProperty9_MultipleVersions tests that multiple updates preserve all versions
func TestProperty9_MultipleVersions(t *testing.T) {
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

		// Create an agent
		configs := []agentmodels.AgentConfig{generateValidAgentConfig(t)}

		req := &agent.CreateAgentRequest{
			Name:         generateAgentName(t),
			Config:       configs[0],
			PricePerCall: generateValidPrice(t),
		}

		createResp, err := agentService.Create(ctx, creatorID, req)
		if err != nil {
			t.Fatalf("Failed to create agent: %v", err)
		}
		defer func() {
			// Cleanup agent and versions
			_, _ = testDB.Exec(ctx, "DELETE FROM agent_versions WHERE agent_id = $1", createResp.ID)
			_, _ = testDB.Exec(ctx, "DELETE FROM agents WHERE id = $1", createResp.ID)
		}()

		// Perform multiple updates (2-5 updates)
		numUpdates := rapid.IntRange(2, 5).Draw(t, "numUpdates")

		for i := 0; i < numUpdates; i++ {
			newConfig := generateValidAgentConfig(t)
			configs = append(configs, newConfig)

			updateReq := &agent.UpdateAgentRequest{
				Config: &newConfig,
			}

			updateResp, err := agentService.Update(ctx, createResp.ID, creatorID, updateReq)
			if err != nil {
				t.Fatalf("Failed to update agent (update %d): %v", i+1, err)
			}

			// Property: Version should increment correctly
			expectedVersion := i + 2 // starts at 1, first update makes it 2
			if updateResp.Version != expectedVersion {
				t.Fatalf("Version should be %d after update %d, got: %d", expectedVersion, i+1, updateResp.Version)
			}
		}

		// Get all historical versions
		versionsResp, err := agentService.GetVersions(ctx, createResp.ID, creatorID)
		if err != nil {
			t.Fatalf("Failed to get versions: %v", err)
		}

		// Property: Number of historical versions should equal number of updates
		if versionsResp.Total != numUpdates {
			t.Fatalf("Should have %d historical versions, got: %d", numUpdates, versionsResp.Total)
		}

		// Property: Each historical version should be retrievable and have correct config
		for i := 0; i < numUpdates; i++ {
			versionNum := i + 1 // versions 1 through numUpdates
			versionResp, err := agentService.GetVersion(ctx, createResp.ID, creatorID, versionNum)
			if err != nil {
				t.Fatalf("Failed to get version %d: %v", versionNum, err)
			}

			// Property: Version number should match
			if versionResp.Version != versionNum {
				t.Fatalf("Retrieved version should be %d, got: %d", versionNum, versionResp.Version)
			}

			// Property: Config should match the original config at that version
			if versionResp.Config.SystemPrompt != configs[i].SystemPrompt {
				t.Fatalf("Version %d config mismatch: expected %q, got %q",
					versionNum, configs[i].SystemPrompt, versionResp.Config.SystemPrompt)
			}
		}
	})
}

// TestProperty9_VersionOwnershipValidation tests that only owners can access versions
func TestProperty9_VersionOwnershipValidation(t *testing.T) {
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

		// Create two test creators
		creatorID1, cleanup1 := createTestCreator(ctx, t, authService)
		defer cleanup1()

		creatorID2, cleanup2 := createTestCreator(ctx, t, authService)
		defer cleanup2()

		// Create an agent owned by creator 1
		req := &agent.CreateAgentRequest{
			Name:         generateAgentName(t),
			Config:       generateValidAgentConfig(t),
			PricePerCall: generateValidPrice(t),
		}

		createResp, err := agentService.Create(ctx, creatorID1, req)
		if err != nil {
			t.Fatalf("Failed to create agent: %v", err)
		}
		defer func() {
			_, _ = testDB.Exec(ctx, "DELETE FROM agent_versions WHERE agent_id = $1", createResp.ID)
			_, _ = testDB.Exec(ctx, "DELETE FROM agents WHERE id = $1", createResp.ID)
		}()

		// Update the agent to create a version
		newConfig := generateValidAgentConfig(t)
		updateReq := &agent.UpdateAgentRequest{
			Config: &newConfig,
		}
		_, err = agentService.Update(ctx, createResp.ID, creatorID1, updateReq)
		if err != nil {
			t.Fatalf("Failed to update agent: %v", err)
		}

		// Property: Non-owner should not be able to list versions
		_, err = agentService.GetVersions(ctx, createResp.ID, creatorID2)
		if err != agent.ErrAgentNotOwned {
			t.Fatalf("Non-owner listing versions should return ErrAgentNotOwned, got: %v", err)
		}

		// Property: Non-owner should not be able to get specific version
		_, err = agentService.GetVersion(ctx, createResp.ID, creatorID2, 1)
		if err != agent.ErrAgentNotOwned {
			t.Fatalf("Non-owner getting version should return ErrAgentNotOwned, got: %v", err)
		}

		// Property: Owner should be able to access versions
		versionsResp, err := agentService.GetVersions(ctx, createResp.ID, creatorID1)
		if err != nil {
			t.Fatalf("Owner should be able to list versions: %v", err)
		}
		if versionsResp.Total != 1 {
			t.Fatalf("Should have 1 version, got: %d", versionsResp.Total)
		}

		versionResp, err := agentService.GetVersion(ctx, createResp.ID, creatorID1, 1)
		if err != nil {
			t.Fatalf("Owner should be able to get version: %v", err)
		}
		if versionResp.Version != 1 {
			t.Fatalf("Version should be 1, got: %d", versionResp.Version)
		}
	})
}
