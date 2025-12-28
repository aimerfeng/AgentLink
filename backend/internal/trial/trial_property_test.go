package trial

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/aimerfeng/AgentLink/internal/config"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"pgregory.net/rapid"
)

var (
	testDB  *pgxpool.Pool
	testCfg *config.Config
)

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

	// Create test config
	testCfg = &config.Config{
		Quota: config.QuotaConfig{
			FreeInitial:        100,
			TrialCallsPerAgent: 3,
		},
		Encryption: config.EncryptionConfig{
			Key: "test-encryption-key-32-bytes-ok",
		},
	}

	code := m.Run()

	if testDB != nil {
		testDB.Close()
	}

	os.Exit(code)
}


// Helper functions for test setup and cleanup

func createTestUser(t *testing.T, ctx context.Context) uuid.UUID {
	userID := uuid.New()
	_, err := testDB.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, user_type, email_verified)
		VALUES ($1, $2, $3, $4, $5)
	`, userID, fmt.Sprintf("test_%s@example.com", userID.String()[:8]), "hash", "developer", true)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}
	return userID
}

func createTestCreator(t *testing.T, ctx context.Context) uuid.UUID {
	userID := uuid.New()
	_, err := testDB.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, user_type, email_verified)
		VALUES ($1, $2, $3, $4, $5)
	`, userID, fmt.Sprintf("creator_%s@example.com", userID.String()[:8]), "hash", "creator", true)
	if err != nil {
		t.Fatalf("Failed to create test creator: %v", err)
	}
	return userID
}

func createTestAgent(t *testing.T, ctx context.Context, creatorID uuid.UUID, trialEnabled bool) uuid.UUID {
	agentID := uuid.New()
	// Create minimal encrypted config
	configEncrypted := []byte("encrypted_config")
	configIV := []byte("iv_12_bytes!")
	
	_, err := testDB.Exec(ctx, `
		INSERT INTO agents (id, creator_id, name, config_encrypted, config_iv, price_per_call, status, trial_enabled)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, agentID, creatorID, fmt.Sprintf("Test Agent %s", agentID.String()[:8]), 
		configEncrypted, configIV, decimal.NewFromFloat(0.01), "active", trialEnabled)
	if err != nil {
		t.Fatalf("Failed to create test agent: %v", err)
	}
	return agentID
}

func cleanupTestUser(t *testing.T, ctx context.Context, userID uuid.UUID) {
	testDB.Exec(ctx, `DELETE FROM trial_usage WHERE user_id = $1`, userID)
	testDB.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID)
}

func cleanupTestAgent(t *testing.T, ctx context.Context, agentID uuid.UUID) {
	testDB.Exec(ctx, `DELETE FROM trial_usage WHERE agent_id = $1`, agentID)
	testDB.Exec(ctx, `DELETE FROM agents WHERE id = $1`, agentID)
}

func cleanupTestCreator(t *testing.T, ctx context.Context, creatorID uuid.UUID) {
	testDB.Exec(ctx, `DELETE FROM agents WHERE creator_id = $1`, creatorID)
	testDB.Exec(ctx, `DELETE FROM users WHERE id = $1`, creatorID)
}


// TestProperty_TrialQuotaLimit tests that trial calls are limited per agent per user
// *For any* user-agent pair, the system SHALL provide exactly 3 free trial calls.
// **Validates: Requirements D5.1**
func TestProperty_TrialQuotaLimit(t *testing.T) {
	if testDB == nil {
		t.Skip("Test database not available")
	}

	ctx := context.Background()
	svc := NewService(testDB, &testCfg.Quota)

	rapid.Check(t, func(rt *rapid.T) {
		// Create test user and agent
		creatorID := createTestCreator(t, ctx)
		defer cleanupTestCreator(t, ctx, creatorID)
		
		userID := createTestUser(t, ctx)
		defer cleanupTestUser(t, ctx, userID)
		
		agentID := createTestAgent(t, ctx, creatorID, true)
		defer cleanupTestAgent(t, ctx, agentID)

		maxTrials := svc.GetTrialCallsPerAgent()

		// Use all trial calls
		for i := 0; i < maxTrials; i++ {
			info, err := svc.UseTrialCall(ctx, userID, agentID)
			if err != nil {
				t.Fatalf("Failed to use trial call %d: %v", i+1, err)
			}
			
			expectedUsed := i + 1
			expectedRemaining := maxTrials - expectedUsed
			
			if info.UsedTrials != expectedUsed {
				t.Fatalf("PROPERTY VIOLATION: Expected used_trials %d, got %d after call %d",
					expectedUsed, info.UsedTrials, i+1)
			}
			
			if info.RemainingTrials != expectedRemaining {
				t.Fatalf("PROPERTY VIOLATION: Expected remaining_trials %d, got %d after call %d",
					expectedRemaining, info.RemainingTrials, i+1)
			}
		}

		// Verify that additional trial calls are rejected
		_, err := svc.UseTrialCall(ctx, userID, agentID)
		if err != ErrTrialExhausted {
			t.Fatalf("PROPERTY VIOLATION: Expected ErrTrialExhausted after %d calls, got: %v", maxTrials, err)
		}

		// Verify final state
		info, err := svc.GetTrialInfo(ctx, userID, agentID)
		if err != nil {
			t.Fatalf("Failed to get trial info: %v", err)
		}

		if info.UsedTrials != maxTrials {
			t.Fatalf("PROPERTY VIOLATION: Expected final used_trials %d, got %d", maxTrials, info.UsedTrials)
		}

		if info.RemainingTrials != 0 {
			t.Fatalf("PROPERTY VIOLATION: Expected final remaining_trials 0, got %d", info.RemainingTrials)
		}

		if !info.IsExhausted {
			t.Fatalf("PROPERTY VIOLATION: Expected IsExhausted to be true")
		}
	})
}


// TestProperty_TrialQuotaIndependence tests that trial quotas are independent per agent
// *For any* user with multiple agents, trial quota for one agent SHALL NOT affect another.
// **Validates: Requirements D5.1**
func TestProperty_TrialQuotaIndependence(t *testing.T) {
	if testDB == nil {
		t.Skip("Test database not available")
	}

	ctx := context.Background()
	svc := NewService(testDB, &testCfg.Quota)

	rapid.Check(t, func(rt *rapid.T) {
		// Create test user and multiple agents
		creatorID := createTestCreator(t, ctx)
		defer cleanupTestCreator(t, ctx, creatorID)
		
		userID := createTestUser(t, ctx)
		defer cleanupTestUser(t, ctx, userID)
		
		// Create 2-5 agents
		numAgents := rapid.IntRange(2, 5).Draw(rt, "numAgents")
		agentIDs := make([]uuid.UUID, numAgents)
		for i := 0; i < numAgents; i++ {
			agentIDs[i] = createTestAgent(t, ctx, creatorID, true)
			defer cleanupTestAgent(t, ctx, agentIDs[i])
		}

		maxTrials := svc.GetTrialCallsPerAgent()

		// Use all trials on first agent
		for i := 0; i < maxTrials; i++ {
			_, err := svc.UseTrialCall(ctx, userID, agentIDs[0])
			if err != nil {
				t.Fatalf("Failed to use trial call on agent 0: %v", err)
			}
		}

		// Verify first agent is exhausted
		info0, err := svc.GetTrialInfo(ctx, userID, agentIDs[0])
		if err != nil {
			t.Fatalf("Failed to get trial info for agent 0: %v", err)
		}
		if !info0.IsExhausted {
			t.Fatalf("PROPERTY VIOLATION: Agent 0 should be exhausted")
		}

		// Verify other agents still have full trial quota
		for i := 1; i < numAgents; i++ {
			info, err := svc.GetTrialInfo(ctx, userID, agentIDs[i])
			if err != nil {
				t.Fatalf("Failed to get trial info for agent %d: %v", i, err)
			}
			
			if info.UsedTrials != 0 {
				t.Fatalf("PROPERTY VIOLATION: Agent %d should have 0 used trials, got %d", i, info.UsedTrials)
			}
			
			if info.RemainingTrials != maxTrials {
				t.Fatalf("PROPERTY VIOLATION: Agent %d should have %d remaining trials, got %d", 
					i, maxTrials, info.RemainingTrials)
			}
			
			if info.IsExhausted {
				t.Fatalf("PROPERTY VIOLATION: Agent %d should NOT be exhausted", i)
			}
		}
	})
}

// TestProperty_TrialDisabled tests that disabled trials are rejected
// *For any* agent with trial disabled, trial calls SHALL be rejected.
// **Validates: Requirements D5.4**
func TestProperty_TrialDisabled(t *testing.T) {
	if testDB == nil {
		t.Skip("Test database not available")
	}

	ctx := context.Background()
	svc := NewService(testDB, &testCfg.Quota)

	rapid.Check(t, func(rt *rapid.T) {
		// Create test user and agent with trial disabled
		creatorID := createTestCreator(t, ctx)
		defer cleanupTestCreator(t, ctx, creatorID)
		
		userID := createTestUser(t, ctx)
		defer cleanupTestUser(t, ctx, userID)
		
		agentID := createTestAgent(t, ctx, creatorID, false) // trial disabled
		defer cleanupTestAgent(t, ctx, agentID)

		// Verify trial is disabled
		info, err := svc.GetTrialInfo(ctx, userID, agentID)
		if err != nil {
			t.Fatalf("Failed to get trial info: %v", err)
		}
		
		if info.TrialEnabled {
			t.Fatalf("PROPERTY VIOLATION: Trial should be disabled")
		}

		// Verify trial calls are rejected
		available, _, err := svc.CheckTrialAvailable(ctx, userID, agentID)
		if err != ErrTrialDisabled {
			t.Fatalf("PROPERTY VIOLATION: Expected ErrTrialDisabled, got: %v", err)
		}
		if available {
			t.Fatalf("PROPERTY VIOLATION: Trial should not be available when disabled")
		}
	})
}


// TestProperty_TrialInfoAccuracy tests that trial info accurately reflects usage
// *For any* sequence of trial calls, GetTrialInfo SHALL return accurate remaining count.
// **Validates: Requirements D5.2**
func TestProperty_TrialInfoAccuracy(t *testing.T) {
	if testDB == nil {
		t.Skip("Test database not available")
	}

	ctx := context.Background()
	svc := NewService(testDB, &testCfg.Quota)

	rapid.Check(t, func(rt *rapid.T) {
		// Create test user and agent
		creatorID := createTestCreator(t, ctx)
		defer cleanupTestCreator(t, ctx, creatorID)
		
		userID := createTestUser(t, ctx)
		defer cleanupTestUser(t, ctx, userID)
		
		agentID := createTestAgent(t, ctx, creatorID, true)
		defer cleanupTestAgent(t, ctx, agentID)

		maxTrials := svc.GetTrialCallsPerAgent()
		
		// Generate random number of calls to make (0 to maxTrials)
		numCalls := rapid.IntRange(0, maxTrials).Draw(rt, "numCalls")

		// Make the calls
		for i := 0; i < numCalls; i++ {
			_, err := svc.UseTrialCall(ctx, userID, agentID)
			if err != nil {
				t.Fatalf("Failed to use trial call %d: %v", i+1, err)
			}
		}

		// Verify trial info is accurate
		info, err := svc.GetTrialInfo(ctx, userID, agentID)
		if err != nil {
			t.Fatalf("Failed to get trial info: %v", err)
		}

		expectedUsed := numCalls
		expectedRemaining := maxTrials - numCalls
		expectedExhausted := expectedRemaining <= 0

		if info.UsedTrials != expectedUsed {
			t.Fatalf("PROPERTY VIOLATION: Expected used_trials %d, got %d", expectedUsed, info.UsedTrials)
		}

		if info.RemainingTrials != expectedRemaining {
			t.Fatalf("PROPERTY VIOLATION: Expected remaining_trials %d, got %d", expectedRemaining, info.RemainingTrials)
		}

		if info.IsExhausted != expectedExhausted {
			t.Fatalf("PROPERTY VIOLATION: Expected is_exhausted %v, got %v", expectedExhausted, info.IsExhausted)
		}

		if info.MaxTrials != maxTrials {
			t.Fatalf("PROPERTY VIOLATION: Expected max_trials %d, got %d", maxTrials, info.MaxTrials)
		}
	})
}

// TestProperty_TrialToggle tests that creators can enable/disable trial
// *For any* agent, the creator SHALL be able to toggle trial on/off.
// **Validates: Requirements D5.4**
func TestProperty_TrialToggle(t *testing.T) {
	if testDB == nil {
		t.Skip("Test database not available")
	}

	ctx := context.Background()
	svc := NewService(testDB, &testCfg.Quota)

	rapid.Check(t, func(rt *rapid.T) {
		// Create test creator and agent
		creatorID := createTestCreator(t, ctx)
		defer cleanupTestCreator(t, ctx, creatorID)
		
		userID := createTestUser(t, ctx)
		defer cleanupTestUser(t, ctx, userID)
		
		// Start with trial enabled
		agentID := createTestAgent(t, ctx, creatorID, true)
		defer cleanupTestAgent(t, ctx, agentID)

		// Generate random sequence of toggles
		numToggles := rapid.IntRange(1, 5).Draw(rt, "numToggles")
		expectedEnabled := true

		for i := 0; i < numToggles; i++ {
			// Toggle the state
			expectedEnabled = !expectedEnabled
			
			err := svc.SetAgentTrialEnabled(ctx, agentID, creatorID, expectedEnabled)
			if err != nil {
				t.Fatalf("Failed to set trial enabled to %v: %v", expectedEnabled, err)
			}

			// Verify the state
			enabled, err := svc.GetAgentTrialEnabled(ctx, agentID)
			if err != nil {
				t.Fatalf("Failed to get trial enabled: %v", err)
			}

			if enabled != expectedEnabled {
				t.Fatalf("PROPERTY VIOLATION: Expected trial_enabled %v, got %v after toggle %d",
					expectedEnabled, enabled, i+1)
			}

			// Verify trial info reflects the state
			info, err := svc.GetTrialInfo(ctx, userID, agentID)
			if err != nil {
				t.Fatalf("Failed to get trial info: %v", err)
			}

			if info.TrialEnabled != expectedEnabled {
				t.Fatalf("PROPERTY VIOLATION: Trial info shows trial_enabled %v, expected %v",
					info.TrialEnabled, expectedEnabled)
			}
		}
	})
}

// Ensure time package is used (for potential future use)
var _ = time.Now
