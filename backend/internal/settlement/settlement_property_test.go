package settlement

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/aimerfeng/AgentLink/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"pgregory.net/rapid"
)

var (
	testDB *pgxpool.Pool
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

	code := m.Run()

	if testDB != nil {
		testDB.Close()
	}

	os.Exit(code)
}

// ============================================
// Property Tests for Settlement Fee Calculation
// ============================================

// TestProperty_SettlementFee_NonNegative tests that platform fees are never negative
// *For any* settlement amount, the calculated platform fee SHALL be non-negative.
// **Validates: Requirements A8.3**
func TestProperty_SettlementFee_NonNegative(t *testing.T) {
	config := DefaultSettlementConfig()
	svc := NewService(nil, config)

	rapid.Check(t, func(rt *rapid.T) {
		// Generate positive amount
		amountFloat := rapid.Float64Range(0.01, 100000.0).Draw(rt, "amountFloat")
		amount := decimal.NewFromFloat(amountFloat).Round(8)

		fee := svc.CalculatePlatformFee(amount)

		if fee.LessThan(decimal.Zero) {
			t.Fatalf("PROPERTY VIOLATION: Platform fee for $%s should be non-negative, got $%s",
				amount.String(), fee.String())
		}
	})
}


// TestProperty_SettlementFee_LessThanAmount tests that fee is always less than the settlement amount
// *For any* settlement amount, the calculated platform fee SHALL be less than the settlement amount.
// **Validates: Requirements A8.3**
func TestProperty_SettlementFee_LessThanAmount(t *testing.T) {
	config := DefaultSettlementConfig()
	svc := NewService(nil, config)

	rapid.Check(t, func(rt *rapid.T) {
		// Generate positive amount
		amountFloat := rapid.Float64Range(0.01, 100000.0).Draw(rt, "amountFloat")
		amount := decimal.NewFromFloat(amountFloat).Round(8)

		fee := svc.CalculatePlatformFee(amount)

		if fee.GreaterThanOrEqual(amount) {
			t.Fatalf("PROPERTY VIOLATION: Platform fee $%s should be less than amount $%s",
				fee.String(), amount.String())
		}
	})
}

// TestProperty_SettlementFee_NetAmountPositive tests that net amount is always positive
// *For any* positive settlement amount, the net amount after fees SHALL be positive.
// **Validates: Requirements A8.3**
func TestProperty_SettlementFee_NetAmountPositive(t *testing.T) {
	config := DefaultSettlementConfig()
	svc := NewService(nil, config)

	rapid.Check(t, func(rt *rapid.T) {
		// Generate positive amount
		amountFloat := rapid.Float64Range(0.01, 100000.0).Draw(rt, "amountFloat")
		amount := decimal.NewFromFloat(amountFloat).Round(8)

		netAmount := svc.CalculateNetAmount(amount)

		if netAmount.LessThanOrEqual(decimal.Zero) {
			t.Fatalf("PROPERTY VIOLATION: Net amount for $%s should be positive, got $%s",
				amount.String(), netAmount.String())
		}
	})
}

// TestProperty_SettlementFee_FeeEqualsAmountMinusNet tests fee + net = amount
// *For any* settlement amount, the sum of platform fee and net amount SHALL equal the original amount.
// **Validates: Requirements A8.3**
func TestProperty_SettlementFee_FeeEqualsAmountMinusNet(t *testing.T) {
	config := DefaultSettlementConfig()
	svc := NewService(nil, config)

	rapid.Check(t, func(rt *rapid.T) {
		amountFloat := rapid.Float64Range(0.01, 100000.0).Draw(rt, "amountFloat")
		amount := decimal.NewFromFloat(amountFloat).Round(8)

		fee := svc.CalculatePlatformFee(amount)
		netAmount := svc.CalculateNetAmount(amount)

		// fee + netAmount should equal amount
		sum := fee.Add(netAmount)
		
		// Allow small rounding difference (8 decimal places)
		diff := sum.Sub(amount).Abs()
		tolerance := decimal.NewFromFloat(0.00000001)
		
		if diff.GreaterThan(tolerance) {
			t.Fatalf("PROPERTY VIOLATION: Fee ($%s) + Net ($%s) = $%s should equal Amount ($%s)",
				fee.String(), netAmount.String(), sum.String(), amount.String())
		}
	})
}

// TestProperty_SettlementFee_ProportionalToAmount tests that fee is proportional to amount
// *For any* two settlement amounts where A > B, the fee for A SHALL be greater than the fee for B.
// **Validates: Requirements A8.3**
func TestProperty_SettlementFee_ProportionalToAmount(t *testing.T) {
	config := DefaultSettlementConfig()
	svc := NewService(nil, config)

	rapid.Check(t, func(rt *rapid.T) {
		// Generate two different amounts
		amount1Float := rapid.Float64Range(0.01, 50000.0).Draw(rt, "amount1Float")
		amount2Float := rapid.Float64Range(amount1Float+0.01, 100000.0).Draw(rt, "amount2Float")
		
		amount1 := decimal.NewFromFloat(amount1Float).Round(8)
		amount2 := decimal.NewFromFloat(amount2Float).Round(8)

		fee1 := svc.CalculatePlatformFee(amount1)
		fee2 := svc.CalculatePlatformFee(amount2)

		// Since amount2 > amount1, fee2 should be > fee1
		if !fee2.GreaterThan(fee1) {
			t.Fatalf("PROPERTY VIOLATION: Fee for larger amount ($%s -> $%s) should be greater than fee for smaller amount ($%s -> $%s)",
				amount2.String(), fee2.String(), amount1.String(), fee1.String())
		}
	})
}

// TestProperty_SettlementFee_ConsistentRate tests that fee rate is consistent
// *For any* settlement amount, the fee rate (fee/amount) SHALL equal the configured platform fee rate.
// **Validates: Requirements A8.3**
func TestProperty_SettlementFee_ConsistentRate(t *testing.T) {
	config := DefaultSettlementConfig()
	svc := NewService(nil, config)

	rapid.Check(t, func(rt *rapid.T) {
		amountFloat := rapid.Float64Range(1.0, 100000.0).Draw(rt, "amountFloat")
		amount := decimal.NewFromFloat(amountFloat).Round(8)

		fee := svc.CalculatePlatformFee(amount)
		
		// Calculate actual rate
		actualRate := fee.Div(amount)
		
		// Allow small rounding difference
		diff := actualRate.Sub(config.PlatformFeeRate).Abs()
		tolerance := decimal.NewFromFloat(0.0001) // 0.01% tolerance
		
		if diff.GreaterThan(tolerance) {
			t.Fatalf("PROPERTY VIOLATION: Fee rate (%s) should equal configured rate (%s) for amount $%s",
				actualRate.String(), config.PlatformFeeRate.String(), amount.String())
		}
	})
}


// ============================================
// Property Tests for Settlement Period
// ============================================

// TestProperty_SettlementPeriod_ValidRange tests that settlement periods have valid ranges
// *For any* settlement period, the end date SHALL be after the start date.
// **Validates: Requirements A8.3**
func TestProperty_SettlementPeriod_ValidRange(t *testing.T) {
	config := DefaultSettlementConfig()
	svc := NewService(nil, config)

	// Test current period
	currentStart, currentEnd := svc.GetCurrentSettlementPeriod()
	if !currentEnd.After(currentStart) {
		t.Fatalf("PROPERTY VIOLATION: Current period end (%v) should be after start (%v)",
			currentEnd, currentStart)
	}

	// Test previous period
	prevStart, prevEnd := svc.GetPreviousSettlementPeriod()
	if !prevEnd.After(prevStart) {
		t.Fatalf("PROPERTY VIOLATION: Previous period end (%v) should be after start (%v)",
			prevEnd, prevStart)
	}
}

// TestProperty_SettlementPeriod_ConsecutivePeriods tests that periods are consecutive
// *For any* two consecutive settlement periods, the end of the first SHALL equal the start of the second.
// **Validates: Requirements A8.3**
func TestProperty_SettlementPeriod_ConsecutivePeriods(t *testing.T) {
	config := DefaultSettlementConfig()
	svc := NewService(nil, config)

	// Get current and previous periods
	currentStart, _ := svc.GetCurrentSettlementPeriod()
	_, prevEnd := svc.GetPreviousSettlementPeriod()

	// Previous period end should equal current period start
	if !prevEnd.Equal(currentStart) {
		t.Fatalf("PROPERTY VIOLATION: Previous period end (%v) should equal current period start (%v)",
			prevEnd, currentStart)
	}
}

// TestProperty_SettlementPeriod_CorrectDuration tests that periods have correct duration
// *For any* settlement period, the duration SHALL equal the configured settlement period days.
// **Validates: Requirements A8.3**
func TestProperty_SettlementPeriod_CorrectDuration(t *testing.T) {
	config := DefaultSettlementConfig()
	svc := NewService(nil, config)

	// Test current period
	currentStart, currentEnd := svc.GetCurrentSettlementPeriod()
	expectedDuration := time.Duration(config.SettlementPeriodDays) * 24 * time.Hour
	actualDuration := currentEnd.Sub(currentStart)

	if actualDuration != expectedDuration {
		t.Fatalf("PROPERTY VIOLATION: Period duration (%v) should equal configured duration (%v)",
			actualDuration, expectedDuration)
	}

	// Test previous period
	prevStart, prevEnd := svc.GetPreviousSettlementPeriod()
	actualDuration = prevEnd.Sub(prevStart)

	if actualDuration != expectedDuration {
		t.Fatalf("PROPERTY VIOLATION: Previous period duration (%v) should equal configured duration (%v)",
			actualDuration, expectedDuration)
	}
}

// ============================================
// Property Tests for Settlement Calculation with Database
// ============================================

// TestProperty_Settlement_CreatorShareCalculation tests that creator share is correctly calculated
// *For any* settlement, the creator's share SHALL equal gross amount minus platform fee.
// **Validates: Requirements A8.3**
func TestProperty_Settlement_CreatorShareCalculation(t *testing.T) {
	if testDB == nil {
		t.Skip("Test database not available")
	}

	ctx := context.Background()
	
	// Check if required tables exist
	var exists bool
	err := testDB.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_name = 'settlements'
		)
	`).Scan(&exists)
	if err != nil || !exists {
		t.Skip("Settlements table not available - run migrations first")
	}

	config := DefaultSettlementConfig()
	svc := NewService(testDB, config)

	rapid.Check(t, func(rt *rapid.T) {
		// Generate test data
		grossAmountFloat := rapid.Float64Range(10.0, 10000.0).Draw(rt, "grossAmountFloat")
		grossAmount := decimal.NewFromFloat(grossAmountFloat).Round(8)

		// Calculate expected values
		expectedFee := svc.CalculatePlatformFee(grossAmount)
		expectedNet := svc.CalculateNetAmount(grossAmount)

		// Verify the relationship: gross = fee + net
		calculatedGross := expectedFee.Add(expectedNet)
		diff := calculatedGross.Sub(grossAmount).Abs()
		tolerance := decimal.NewFromFloat(0.00000001)

		if diff.GreaterThan(tolerance) {
			t.Fatalf("PROPERTY VIOLATION: Fee ($%s) + Net ($%s) = $%s should equal Gross ($%s)",
				expectedFee.String(), expectedNet.String(), calculatedGross.String(), grossAmount.String())
		}
	})
}


// ============================================
// Helper Functions
// ============================================

func createTestCreator(t *testing.T, ctx context.Context) uuid.UUID {
	t.Helper()

	userID := uuid.New()
	email := fmt.Sprintf("test-settlement-%s@example.com", userID.String()[:8])

	// Create user
	_, err := testDB.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, user_type, email_verified)
		VALUES ($1, $2, 'test-hash', 'creator', true)
	`, userID, email)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	// Create creator profile
	_, err = testDB.Exec(ctx, `
		INSERT INTO creator_profiles (user_id, display_name, verified, total_earnings, pending_earnings)
		VALUES ($1, $2, false, 0, 0)
	`, userID, "Test Creator")
	if err != nil {
		t.Fatalf("Failed to create creator profile: %v", err)
	}

	return userID
}

func createTestAgent(t *testing.T, ctx context.Context, creatorID uuid.UUID, pricePerCall decimal.Decimal) uuid.UUID {
	t.Helper()

	agentID := uuid.New()
	
	// Create agent with minimal required fields
	_, err := testDB.Exec(ctx, `
		INSERT INTO agents (id, creator_id, name, config_encrypted, config_iv, price_per_call, status)
		VALUES ($1, $2, $3, $4, $5, $6, 'active')
	`, agentID, creatorID, "Test Agent", []byte("encrypted"), []byte("iv"), pricePerCall)
	if err != nil {
		t.Fatalf("Failed to create test agent: %v", err)
	}

	return agentID
}

func createTestCallLog(t *testing.T, ctx context.Context, agentID, userID, apiKeyID uuid.UUID, cost decimal.Decimal, createdAt time.Time) {
	t.Helper()

	// Ensure partition exists for the date
	partitionName := fmt.Sprintf("call_logs_%s", createdAt.Format("2006_01"))
	nextMonth := createdAt.AddDate(0, 1, 0)
	
	// Try to create partition if it doesn't exist (ignore error if it already exists)
	_, _ = testDB.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s PARTITION OF call_logs
		FOR VALUES FROM ('%s') TO ('%s')
	`, partitionName, createdAt.Format("2006-01-01"), nextMonth.Format("2006-01-01")))

	_, err := testDB.Exec(ctx, `
		INSERT INTO call_logs (id, agent_id, api_key_id, user_id, request_id, status, cost_usd, created_at)
		VALUES ($1, $2, $3, $4, $5, 'success', $6, $7)
	`, uuid.New(), agentID, apiKeyID, userID, uuid.New().String(), cost, createdAt)
	if err != nil {
		t.Fatalf("Failed to create test call log: %v", err)
	}
}

func createTestAPIKey(t *testing.T, ctx context.Context, userID uuid.UUID) uuid.UUID {
	t.Helper()

	apiKeyID := uuid.New()
	keyHash := fmt.Sprintf("test-key-hash-%s", apiKeyID.String()[:8])
	
	_, err := testDB.Exec(ctx, `
		INSERT INTO api_keys (id, user_id, key_hash, key_prefix, name)
		VALUES ($1, $2, $3, 'ak_test', 'Test Key')
	`, apiKeyID, userID, keyHash)
	if err != nil {
		t.Fatalf("Failed to create test API key: %v", err)
	}

	return apiKeyID
}

func cleanupTestCreator(t *testing.T, ctx context.Context, userID uuid.UUID) {
	t.Helper()

	// Delete settlements
	_, _ = testDB.Exec(ctx, `DELETE FROM settlements WHERE creator_id = $1`, userID)
	// Delete call logs for agents owned by this creator
	_, _ = testDB.Exec(ctx, `DELETE FROM call_logs WHERE agent_id IN (SELECT id FROM agents WHERE creator_id = $1)`, userID)
	// Delete agents
	_, _ = testDB.Exec(ctx, `DELETE FROM agents WHERE creator_id = $1`, userID)
	// Delete API keys
	_, _ = testDB.Exec(ctx, `DELETE FROM api_keys WHERE user_id = $1`, userID)
	// Delete creator profile
	_, _ = testDB.Exec(ctx, `DELETE FROM creator_profiles WHERE user_id = $1`, userID)
	// Delete user
	_, _ = testDB.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID)
}

// ============================================
// Integration Property Tests
// ============================================

// TestProperty_Settlement_CalculationMatchesCallLogs tests that settlement calculation matches call logs
// *For any* creator with call logs, the settlement calculation SHALL accurately reflect the total revenue from call logs.
// **Validates: Requirements A8.3**
func TestProperty_Settlement_CalculationMatchesCallLogs(t *testing.T) {
	if testDB == nil {
		t.Skip("Test database not available")
	}

	ctx := context.Background()
	
	// Check if required tables exist
	var exists bool
	err := testDB.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_name = 'call_logs'
		)
	`).Scan(&exists)
	if err != nil || !exists {
		t.Skip("Call logs table not available - run migrations first")
	}

	config := DefaultSettlementConfig()
	svc := NewService(testDB, config)

	rapid.Check(t, func(rt *rapid.T) {
		// Create test creator
		creatorID := createTestCreator(t, ctx)
		defer cleanupTestCreator(t, ctx, creatorID)

		// Create test developer (for API key)
		developerID := uuid.New()
		devEmail := fmt.Sprintf("test-dev-%s@example.com", developerID.String()[:8])
		_, err := testDB.Exec(ctx, `
			INSERT INTO users (id, email, password_hash, user_type, email_verified)
			VALUES ($1, $2, 'test-hash', 'developer', true)
		`, developerID, devEmail)
		if err != nil {
			t.Fatalf("Failed to create test developer: %v", err)
		}
		defer func() {
			_, _ = testDB.Exec(ctx, `DELETE FROM api_keys WHERE user_id = $1`, developerID)
			_, _ = testDB.Exec(ctx, `DELETE FROM users WHERE id = $1`, developerID)
		}()

		// Create API key for developer
		apiKeyID := createTestAPIKey(t, ctx, developerID)

		// Generate random number of agents and calls
		numAgents := rapid.IntRange(1, 3).Draw(rt, "numAgents")
		
		var expectedTotalRevenue decimal.Decimal
		periodStart := time.Now().AddDate(0, 0, -7).Truncate(24 * time.Hour)
		periodEnd := time.Now().Truncate(24 * time.Hour)

		for i := 0; i < numAgents; i++ {
			// Create agent with random price
			priceFloat := rapid.Float64Range(0.001, 1.0).Draw(rt, fmt.Sprintf("price%d", i))
			price := decimal.NewFromFloat(priceFloat).Round(6)
			agentID := createTestAgent(t, ctx, creatorID, price)

			// Create random number of calls
			numCalls := rapid.IntRange(1, 5).Draw(rt, fmt.Sprintf("numCalls%d", i))
			for j := 0; j < numCalls; j++ {
				// Random cost per call
				costFloat := rapid.Float64Range(0.001, 0.5).Draw(rt, fmt.Sprintf("cost%d_%d", i, j))
				cost := decimal.NewFromFloat(costFloat).Round(6)
				
				// Random time within period
				callTime := periodStart.Add(time.Duration(rapid.IntRange(0, int(periodEnd.Sub(periodStart).Hours())).Draw(rt, fmt.Sprintf("hours%d_%d", i, j))) * time.Hour)
				
				createTestCallLog(t, ctx, agentID, developerID, apiKeyID, cost, callTime)
				expectedTotalRevenue = expectedTotalRevenue.Add(cost)
			}
		}

		// Calculate settlement
		calc, err := svc.CalculateSettlement(ctx, creatorID, periodStart, periodEnd)
		if err != nil {
			t.Fatalf("Failed to calculate settlement: %v", err)
		}

		// Verify gross amount matches expected total revenue
		diff := calc.GrossAmount.Sub(expectedTotalRevenue).Abs()
		tolerance := decimal.NewFromFloat(0.000001)
		
		if diff.GreaterThan(tolerance) {
			t.Fatalf("PROPERTY VIOLATION: Calculated gross amount ($%s) should match expected total revenue ($%s)",
				calc.GrossAmount.String(), expectedTotalRevenue.String())
		}

		// Verify fee calculation
		expectedFee := svc.CalculatePlatformFee(calc.GrossAmount)
		if !calc.PlatformFee.Equal(expectedFee) {
			t.Fatalf("PROPERTY VIOLATION: Platform fee ($%s) should equal calculated fee ($%s)",
				calc.PlatformFee.String(), expectedFee.String())
		}

		// Verify net amount
		expectedNet := calc.GrossAmount.Sub(calc.PlatformFee)
		netDiff := calc.NetAmount.Sub(expectedNet).Abs()
		if netDiff.GreaterThan(tolerance) {
			t.Fatalf("PROPERTY VIOLATION: Net amount ($%s) should equal gross - fee ($%s)",
				calc.NetAmount.String(), expectedNet.String())
		}
	})
}
