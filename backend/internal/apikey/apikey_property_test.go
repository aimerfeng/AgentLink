package apikey

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"pgregory.net/rapid"
)

var testDB *pgxpool.Pool

func TestMain(m *testing.M) {
	// Try to connect to test database
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@localhost:5432/agentlink_test?sslmode=disable"
	}

	ctx := context.Background()
	var err error
	testDB, err = pgxpool.New(ctx, dbURL)
	if err != nil {
		fmt.Printf("Warning: Failed to connect to test database: %v\n", err)
		testDB = nil
	} else {
		if err := testDB.Ping(ctx); err != nil {
			fmt.Printf("Warning: Failed to ping test database: %v\n", err)
			testDB.Close()
			testDB = nil
		}
	}

	code := m.Run()

	if testDB != nil {
		testDB.Close()
	}

	os.Exit(code)
}

// TestProperty12_APIKeyRevocationImmediacy tests Property 12:
// *For any* revoked API key, all subsequent requests using that key SHALL be rejected immediately.
// **Validates: Requirements 4.3**
func TestProperty12_APIKeyRevocationImmediacy(t *testing.T) {
	if testDB == nil {
		t.Skip("Test database not available")
	}

	ctx := context.Background()
	service := NewService(testDB)

	rapid.Check(t, func(rt *rapid.T) {
		// Generate random key name
		keyName := rapid.StringMatching(`[a-zA-Z][a-zA-Z0-9]{0,19}`).Draw(rt, "keyName")

		// Create a test user first
		userID := createTestDeveloper(t, ctx)
		defer cleanupTestUser(t, ctx, userID)

		// Create an API key
		createReq := &CreateAPIKeyRequest{
			Name: keyName,
		}
		createResp, err := service.Create(ctx, userID, createReq)
		if err != nil {
			t.Fatalf("Failed to create API key: %v", err)
		}

		rawKey := createResp.Key
		keyID := createResp.ID

		// Verify the key is valid before revocation
		apiKey, err := service.ValidateAPIKey(ctx, rawKey)
		if err != nil {
			t.Fatalf("API key should be valid before revocation: %v", err)
		}
		if apiKey.ID != keyID {
			t.Fatalf("Validated key ID mismatch: got %v, want %v", apiKey.ID, keyID)
		}

		// Revoke the key
		err = service.Delete(ctx, keyID, userID)
		if err != nil {
			t.Fatalf("Failed to revoke API key: %v", err)
		}

		// Property 12: After revocation, the key MUST be rejected immediately
		_, err = service.ValidateAPIKey(ctx, rawKey)
		if err == nil {
			t.Fatal("PROPERTY VIOLATION: Revoked API key was accepted")
		}
		if err != ErrAPIKeyRevoked {
			t.Fatalf("Expected ErrAPIKeyRevoked, got: %v", err)
		}

		// Additional check: IsKeyRevoked should return true
		keyHash := HashAPIKey(rawKey)
		revoked, err := service.IsKeyRevoked(ctx, keyHash)
		if err != nil {
			t.Fatalf("Failed to check key revocation: %v", err)
		}
		if !revoked {
			t.Fatal("PROPERTY VIOLATION: IsKeyRevoked returned false for revoked key")
		}
	})
}

// TestProperty12_APIKeyRevocationImmediacy_MultipleKeys tests that revoking one key
// doesn't affect other keys for the same user
func TestProperty12_APIKeyRevocationImmediacy_MultipleKeys(t *testing.T) {
	if testDB == nil {
		t.Skip("Test database not available")
	}

	ctx := context.Background()
	service := NewService(testDB)

	rapid.Check(t, func(rt *rapid.T) {
		// Create a test user
		userID := createTestDeveloper(t, ctx)
		defer cleanupTestUser(t, ctx, userID)

		// Create multiple API keys (2-5)
		numKeys := rapid.IntRange(2, 5).Draw(rt, "numKeys")
		keys := make([]*CreateAPIKeyResponse, numKeys)

		for i := 0; i < numKeys; i++ {
			createReq := &CreateAPIKeyRequest{
				Name: fmt.Sprintf("key-%d", i),
			}
			resp, err := service.Create(ctx, userID, createReq)
			if err != nil {
				t.Fatalf("Failed to create API key %d: %v", i, err)
			}
			keys[i] = resp
		}

		// Pick a random key to revoke
		revokeIdx := rapid.IntRange(0, numKeys-1).Draw(rt, "revokeIdx")
		keyToRevoke := keys[revokeIdx]

		// Revoke the selected key
		err := service.Delete(ctx, keyToRevoke.ID, userID)
		if err != nil {
			t.Fatalf("Failed to revoke API key: %v", err)
		}

		// Verify the revoked key is rejected
		_, err = service.ValidateAPIKey(ctx, keyToRevoke.Key)
		if err != ErrAPIKeyRevoked {
			t.Fatalf("PROPERTY VIOLATION: Revoked key should return ErrAPIKeyRevoked, got: %v", err)
		}

		// Verify other keys are still valid
		for i, key := range keys {
			if i == revokeIdx {
				continue // Skip the revoked key
			}
			apiKey, err := service.ValidateAPIKey(ctx, key.Key)
			if err != nil {
				t.Fatalf("PROPERTY VIOLATION: Non-revoked key %d should still be valid, got error: %v", i, err)
			}
			if apiKey.ID != key.ID {
				t.Fatalf("Key ID mismatch for key %d", i)
			}
		}
	})
}

// TestProperty12_APIKeyRevocationImmediacy_ConcurrentAccess tests that revocation
// is immediately effective even under concurrent access
func TestProperty12_APIKeyRevocationImmediacy_ConcurrentAccess(t *testing.T) {
	if testDB == nil {
		t.Skip("Test database not available")
	}

	ctx := context.Background()
	service := NewService(testDB)

	// Create a test user
	userID := createTestDeveloper(t, ctx)
	defer cleanupTestUser(t, ctx, userID)

	// Create an API key
	createReq := &CreateAPIKeyRequest{Name: "concurrent-test-key"}
	createResp, err := service.Create(ctx, userID, createReq)
	if err != nil {
		t.Fatalf("Failed to create API key: %v", err)
	}

	rawKey := createResp.Key
	keyID := createResp.ID

	// Start multiple goroutines that will try to validate the key
	done := make(chan bool)
	validationErrors := make(chan error, 100)

	// Start validation goroutines
	for i := 0; i < 10; i++ {
		go func() {
			for {
				select {
				case <-done:
					return
				default:
					_, err := service.ValidateAPIKey(ctx, rawKey)
					if err != nil && err != ErrAPIKeyRevoked {
						validationErrors <- err
					}
					time.Sleep(time.Millisecond)
				}
			}
		}()
	}

	// Give goroutines time to start
	time.Sleep(10 * time.Millisecond)

	// Revoke the key
	err = service.Delete(ctx, keyID, userID)
	if err != nil {
		t.Fatalf("Failed to revoke API key: %v", err)
	}

	// Wait a bit for revocation to propagate
	time.Sleep(50 * time.Millisecond)

	// Stop validation goroutines
	close(done)

	// After revocation, all validations should fail
	for i := 0; i < 5; i++ {
		_, err := service.ValidateAPIKey(ctx, rawKey)
		if err == nil {
			t.Fatal("PROPERTY VIOLATION: Revoked key was accepted after revocation")
		}
		if err != ErrAPIKeyRevoked {
			t.Fatalf("Expected ErrAPIKeyRevoked, got: %v", err)
		}
	}

	// Check for any unexpected errors
	select {
	case err := <-validationErrors:
		t.Fatalf("Unexpected validation error: %v", err)
	default:
		// No unexpected errors
	}
}

// TestAPIKeyGeneration tests that generated API keys have correct format
func TestAPIKeyGeneration(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		rawKey, keyHash, keyPrefix, err := generateAPIKey()
		if err != nil {
			t.Fatalf("Failed to generate API key: %v", err)
		}

		// Check prefix format
		if len(rawKey) < 10 || rawKey[:3] != "ak_" {
			t.Fatalf("Invalid key format: %s", rawKey)
		}

		// Check key prefix matches
		if keyPrefix != rawKey[:11] {
			t.Fatalf("Key prefix mismatch: got %s, want %s", keyPrefix, rawKey[:11])
		}

		// Check hash is consistent
		computedHash := HashAPIKey(rawKey)
		if computedHash != keyHash {
			t.Fatalf("Hash mismatch: got %s, want %s", computedHash, keyHash)
		}

		// Check hash length (SHA-256 = 64 hex chars)
		if len(keyHash) != 64 {
			t.Fatalf("Invalid hash length: got %d, want 64", len(keyHash))
		}
	})
}

// TestAPIKeyHashConsistency tests that hashing is deterministic
func TestAPIKeyHashConsistency(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate a random key-like string
		keyContent := rapid.StringMatching(`ak_[a-f0-9]{64}`).Draw(rt, "keyContent")

		// Hash it multiple times
		hash1 := HashAPIKey(keyContent)
		hash2 := HashAPIKey(keyContent)
		hash3 := HashAPIKey(keyContent)

		// All hashes should be identical
		if hash1 != hash2 || hash2 != hash3 {
			t.Fatalf("Hash inconsistency: %s, %s, %s", hash1, hash2, hash3)
		}
	})
}

// Helper functions for test setup

func createTestDeveloper(t *testing.T, ctx context.Context) uuid.UUID {
	t.Helper()

	userID := uuid.New()
	email := fmt.Sprintf("test-%s@example.com", userID.String()[:8])

	_, err := testDB.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, user_type, email_verified)
		VALUES ($1, $2, 'test-hash', 'developer', true)
	`, userID, email)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	// Create quota record
	_, err = testDB.Exec(ctx, `
		INSERT INTO quotas (user_id, total_quota, used_quota, free_quota)
		VALUES ($1, 100, 0, 100)
	`, userID)
	if err != nil {
		t.Fatalf("Failed to create quota: %v", err)
	}

	return userID
}

func cleanupTestUser(t *testing.T, ctx context.Context, userID uuid.UUID) {
	t.Helper()

	// Delete API keys first (foreign key constraint)
	_, _ = testDB.Exec(ctx, `DELETE FROM api_keys WHERE user_id = $1`, userID)
	// Delete quota
	_, _ = testDB.Exec(ctx, `DELETE FROM quotas WHERE user_id = $1`, userID)
	// Delete user
	_, _ = testDB.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID)
}
