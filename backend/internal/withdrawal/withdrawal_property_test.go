package withdrawal

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
// Property Tests for Withdrawal Threshold
// ============================================

// TestProperty_WithdrawalThreshold_BelowMinimumRejected tests that withdrawals below minimum are rejected
// *For any* withdrawal amount below the minimum threshold, the System SHALL reject the withdrawal request.
// **Validates: Withdrawal Threshold Requirements**
func TestProperty_WithdrawalThreshold_BelowMinimumRejected(t *testing.T) {
	if testDB == nil {
		t.Skip("Test database not available")
	}

	ctx := context.Background()
	config := models.DefaultWithdrawalConfig()
	svc := NewService(testDB, config)

	rapid.Check(t, func(rt *rapid.T) {
		// Generate amount below minimum (0.01 to just under minimum)
		minFloat, _ := config.MinimumAmount.Float64()
		amountFloat := rapid.Float64Range(0.01, minFloat-0.01).Draw(rt, "amountFloat")
		amount := decimal.NewFromFloat(amountFloat)

		// Create a test creator with sufficient balance
		creatorID := createTestCreator(t, ctx, config.MinimumAmount.Mul(decimal.NewFromInt(10)))
		defer cleanupTestCreator(t, ctx, creatorID)

		// Attempt withdrawal
		req := &CreateWithdrawalRequest{
			Amount:           amount,
			WithdrawalMethod: models.WithdrawalMethodStripe,
		}

		_, err := svc.CreateWithdrawal(ctx, creatorID, req)

		// Should be rejected with ErrBelowMinimumThreshold
		if err != ErrBelowMinimumThreshold {
			t.Fatalf("PROPERTY VIOLATION: Withdrawal of $%s (below minimum $%s) should be rejected with ErrBelowMinimumThreshold, got: %v",
				amount.String(), config.MinimumAmount.String(), err)
		}
	})
}

// TestProperty_WithdrawalThreshold_AtMinimumAccepted tests that withdrawals at minimum are accepted
// *For any* withdrawal amount at or above the minimum threshold with sufficient balance, the System SHALL accept the withdrawal request.
// **Validates: Withdrawal Threshold Requirements**
func TestProperty_WithdrawalThreshold_AtMinimumAccepted(t *testing.T) {
	if testDB == nil {
		t.Skip("Test database not available")
	}

	ctx := context.Background()
	
	// Check if withdrawals table exists
	var exists bool
	err := testDB.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_name = 'withdrawals'
		)
	`).Scan(&exists)
	if err != nil || !exists {
		t.Skip("Withdrawals table not available - run migrations first")
	}

	config := models.DefaultWithdrawalConfig()
	svc := NewService(testDB, config)

	rapid.Check(t, func(rt *rapid.T) {
		// Generate amount at or above minimum (minimum to 2x minimum)
		minFloat, _ := config.MinimumAmount.Float64()
		amountFloat := rapid.Float64Range(minFloat, minFloat*2).Draw(rt, "amountFloat")
		amount := decimal.NewFromFloat(amountFloat).Round(2)

		// Create a test creator with sufficient balance
		balance := amount.Mul(decimal.NewFromInt(2)) // 2x the withdrawal amount
		creatorID := createTestCreator(t, ctx, balance)
		defer cleanupTestCreator(t, ctx, creatorID)

		// Attempt withdrawal
		req := &CreateWithdrawalRequest{
			Amount:           amount,
			WithdrawalMethod: models.WithdrawalMethodStripe,
		}

		resp, err := svc.CreateWithdrawal(ctx, creatorID, req)
		defer cleanupWithdrawals(t, ctx, creatorID)

		// Should be accepted
		if err != nil {
			t.Fatalf("PROPERTY VIOLATION: Withdrawal of $%s (at/above minimum $%s) with sufficient balance should be accepted, got error: %v",
				amount.String(), config.MinimumAmount.String(), err)
		}

		if resp == nil || resp.Withdrawal == nil {
			t.Fatal("PROPERTY VIOLATION: Successful withdrawal should return withdrawal record")
		}

		if resp.Withdrawal.Status != models.WithdrawalStatusPending {
			t.Fatalf("PROPERTY VIOLATION: New withdrawal should have status 'pending', got '%s'",
				resp.Withdrawal.Status)
		}
	})
}

// TestProperty_WithdrawalThreshold_ExactMinimumAccepted tests that exact minimum amount is accepted
// *For any* withdrawal at exactly the minimum threshold, the System SHALL accept the withdrawal request.
// **Validates: Withdrawal Threshold Requirements**
func TestProperty_WithdrawalThreshold_ExactMinimumAccepted(t *testing.T) {
	if testDB == nil {
		t.Skip("Test database not available")
	}

	ctx := context.Background()
	
	// Check if withdrawals table exists
	var exists bool
	err := testDB.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_name = 'withdrawals'
		)
	`).Scan(&exists)
	if err != nil || !exists {
		t.Skip("Withdrawals table not available - run migrations first")
	}

	config := models.DefaultWithdrawalConfig()
	svc := NewService(testDB, config)

	// Test exact minimum amount
	amount := config.MinimumAmount

	// Create a test creator with sufficient balance
	balance := amount.Mul(decimal.NewFromInt(2))
	creatorID := createTestCreator(t, ctx, balance)
	defer cleanupTestCreator(t, ctx, creatorID)

	// Attempt withdrawal
	req := &CreateWithdrawalRequest{
		Amount:           amount,
		WithdrawalMethod: models.WithdrawalMethodStripe,
	}

	resp, err := svc.CreateWithdrawal(ctx, creatorID, req)
	defer cleanupWithdrawals(t, ctx, creatorID)

	// Should be accepted
	if err != nil {
		t.Fatalf("PROPERTY VIOLATION: Withdrawal of exact minimum $%s should be accepted, got error: %v",
			amount.String(), err)
	}

	if resp == nil || resp.Withdrawal == nil {
		t.Fatal("PROPERTY VIOLATION: Successful withdrawal should return withdrawal record")
	}

	// Verify amount matches
	if !resp.Withdrawal.Amount.Equal(amount) {
		t.Fatalf("PROPERTY VIOLATION: Withdrawal amount should be $%s, got $%s",
			amount.String(), resp.Withdrawal.Amount.String())
	}
}

// TestProperty_WithdrawalThreshold_InsufficientBalanceRejected tests that withdrawals exceeding balance are rejected
// *For any* withdrawal amount exceeding available balance, the System SHALL reject the withdrawal request.
// **Validates: Withdrawal Threshold Requirements**
func TestProperty_WithdrawalThreshold_InsufficientBalanceRejected(t *testing.T) {
	if testDB == nil {
		t.Skip("Test database not available")
	}

	ctx := context.Background()
	config := models.DefaultWithdrawalConfig()
	svc := NewService(testDB, config)

	rapid.Check(t, func(rt *rapid.T) {
		// Generate balance above minimum
		minFloat, _ := config.MinimumAmount.Float64()
		balanceFloat := rapid.Float64Range(minFloat, minFloat*5).Draw(rt, "balanceFloat")
		balance := decimal.NewFromFloat(balanceFloat).Round(2)

		// Generate amount exceeding balance
		excessFloat := rapid.Float64Range(0.01, 100.0).Draw(rt, "excessFloat")
		amount := balance.Add(decimal.NewFromFloat(excessFloat))

		// Create a test creator with limited balance
		creatorID := createTestCreator(t, ctx, balance)
		defer cleanupTestCreator(t, ctx, creatorID)

		// Attempt withdrawal
		req := &CreateWithdrawalRequest{
			Amount:           amount,
			WithdrawalMethod: models.WithdrawalMethodStripe,
		}

		_, err := svc.CreateWithdrawal(ctx, creatorID, req)

		// Should be rejected with ErrInsufficientBalance
		if err != ErrInsufficientBalance {
			t.Fatalf("PROPERTY VIOLATION: Withdrawal of $%s (exceeding balance $%s) should be rejected with ErrInsufficientBalance, got: %v",
				amount.String(), balance.String(), err)
		}
	})
}

// TestProperty_WithdrawalThreshold_BalanceDeducted tests that balance is correctly deducted
// *For any* successful withdrawal, the pending_earnings SHALL be reduced by the withdrawal amount.
// **Validates: Withdrawal Threshold Requirements**
func TestProperty_WithdrawalThreshold_BalanceDeducted(t *testing.T) {
	if testDB == nil {
		t.Skip("Test database not available")
	}

	ctx := context.Background()
	
	// Check if withdrawals table exists
	var exists bool
	err := testDB.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_name = 'withdrawals'
		)
	`).Scan(&exists)
	if err != nil || !exists {
		t.Skip("Withdrawals table not available - run migrations first")
	}

	config := models.DefaultWithdrawalConfig()
	svc := NewService(testDB, config)

	rapid.Check(t, func(rt *rapid.T) {
		// Generate initial balance
		minFloat, _ := config.MinimumAmount.Float64()
		balanceFloat := rapid.Float64Range(minFloat*2, minFloat*10).Draw(rt, "balanceFloat")
		initialBalance := decimal.NewFromFloat(balanceFloat).Round(2)

		// Generate withdrawal amount (between minimum and balance)
		amountFloat := rapid.Float64Range(minFloat, balanceFloat).Draw(rt, "amountFloat")
		amount := decimal.NewFromFloat(amountFloat).Round(2)

		// Create a test creator
		creatorID := createTestCreator(t, ctx, initialBalance)
		defer cleanupTestCreator(t, ctx, creatorID)

		// Attempt withdrawal
		req := &CreateWithdrawalRequest{
			Amount:           amount,
			WithdrawalMethod: models.WithdrawalMethodStripe,
		}

		_, err := svc.CreateWithdrawal(ctx, creatorID, req)
		defer cleanupWithdrawals(t, ctx, creatorID)

		if err != nil {
			t.Fatalf("Failed to create withdrawal: %v", err)
		}

		// Check remaining balance
		earnings, err := svc.GetEarningsInfo(ctx, creatorID)
		if err != nil {
			t.Fatalf("Failed to get earnings info: %v", err)
		}

		expectedBalance := initialBalance.Sub(amount)
		if !earnings.PendingEarnings.Equal(expectedBalance) {
			t.Fatalf("PROPERTY VIOLATION: After withdrawal of $%s from $%s, balance should be $%s, got $%s",
				amount.String(), initialBalance.String(), expectedBalance.String(), earnings.PendingEarnings.String())
		}
	})
}

// ============================================
// Helper Functions
// ============================================

func createTestCreator(t *testing.T, ctx context.Context, pendingEarnings decimal.Decimal) uuid.UUID {
	t.Helper()

	userID := uuid.New()
	email := fmt.Sprintf("test-withdrawal-%s@example.com", userID.String()[:8])

	// Create user
	_, err := testDB.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, user_type, email_verified)
		VALUES ($1, $2, 'test-hash', 'creator', true)
	`, userID, email)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	// Create creator profile with pending earnings
	_, err = testDB.Exec(ctx, `
		INSERT INTO creator_profiles (user_id, display_name, verified, total_earnings, pending_earnings)
		VALUES ($1, $2, false, $3, $3)
	`, userID, "Test Creator", pendingEarnings)
	if err != nil {
		t.Fatalf("Failed to create creator profile: %v", err)
	}

	return userID
}

func cleanupTestCreator(t *testing.T, ctx context.Context, userID uuid.UUID) {
	t.Helper()

	// Delete withdrawals
	_, _ = testDB.Exec(ctx, `DELETE FROM withdrawals WHERE creator_id = $1`, userID)
	// Delete creator profile
	_, _ = testDB.Exec(ctx, `DELETE FROM creator_profiles WHERE user_id = $1`, userID)
	// Delete user
	_, _ = testDB.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID)
}

func cleanupWithdrawals(t *testing.T, ctx context.Context, creatorID uuid.UUID) {
	t.Helper()
	_, _ = testDB.Exec(ctx, `DELETE FROM withdrawals WHERE creator_id = $1`, creatorID)
}

// Wait for async operations
func waitForAsync() {
	time.Sleep(100 * time.Millisecond)
}


// ============================================
// Property Tests for Withdrawal Fee Calculation
// ============================================

// TestProperty_WithdrawalFee_NonNegative tests that fees are never negative
// *For any* withdrawal amount and method, the calculated fee SHALL be non-negative.
// **Validates: Withdrawal Fee Requirements**
func TestProperty_WithdrawalFee_NonNegative(t *testing.T) {
	config := models.DefaultWithdrawalConfig()
	svc := NewService(nil, config)

	methods := []models.WithdrawalMethod{
		models.WithdrawalMethodStripe,
		models.WithdrawalMethodCrypto,
		models.WithdrawalMethodBank,
	}

	rapid.Check(t, func(rt *rapid.T) {
		// Generate positive amount
		amountFloat := rapid.Float64Range(0.01, 10000.0).Draw(rt, "amountFloat")
		amount := decimal.NewFromFloat(amountFloat).Round(2)
		method := methods[rapid.IntRange(0, len(methods)-1).Draw(rt, "methodIndex")]

		fee := svc.CalculateFeeForMethod(amount, method)

		if fee.LessThan(decimal.Zero) {
			t.Fatalf("PROPERTY VIOLATION: Fee for $%s via %s should be non-negative, got $%s",
				amount.String(), method, fee.String())
		}
	})
}

// TestProperty_WithdrawalFee_LessThanAmount tests that fee is always less than the withdrawal amount
// *For any* withdrawal amount, the calculated fee SHALL be less than the withdrawal amount.
// **Validates: Withdrawal Fee Requirements**
func TestProperty_WithdrawalFee_LessThanAmount(t *testing.T) {
	config := models.DefaultWithdrawalConfig()
	svc := NewService(nil, config)

	methods := []models.WithdrawalMethod{
		models.WithdrawalMethodStripe,
		models.WithdrawalMethodCrypto,
		models.WithdrawalMethodBank,
	}

	rapid.Check(t, func(rt *rapid.T) {
		// Generate amount above minimum to ensure meaningful test
		minFloat, _ := config.MinimumAmount.Float64()
		amountFloat := rapid.Float64Range(minFloat, 10000.0).Draw(rt, "amountFloat")
		amount := decimal.NewFromFloat(amountFloat).Round(2)
		method := methods[rapid.IntRange(0, len(methods)-1).Draw(rt, "methodIndex")]

		fee := svc.CalculateFeeForMethod(amount, method)

		if fee.GreaterThanOrEqual(amount) {
			t.Fatalf("PROPERTY VIOLATION: Fee $%s should be less than amount $%s for method %s",
				fee.String(), amount.String(), method)
		}
	})
}

// TestProperty_WithdrawalFee_NetAmountPositive tests that net amount is always positive
// *For any* valid withdrawal amount, the net amount after fees SHALL be positive.
// **Validates: Withdrawal Fee Requirements**
func TestProperty_WithdrawalFee_NetAmountPositive(t *testing.T) {
	config := models.DefaultWithdrawalConfig()
	svc := NewService(nil, config)

	methods := []models.WithdrawalMethod{
		models.WithdrawalMethodStripe,
		models.WithdrawalMethodCrypto,
		models.WithdrawalMethodBank,
	}

	rapid.Check(t, func(rt *rapid.T) {
		// Generate amount above minimum
		minFloat, _ := config.MinimumAmount.Float64()
		amountFloat := rapid.Float64Range(minFloat, 10000.0).Draw(rt, "amountFloat")
		amount := decimal.NewFromFloat(amountFloat).Round(2)
		method := methods[rapid.IntRange(0, len(methods)-1).Draw(rt, "methodIndex")]

		netAmount := svc.CalculateNetAmountForMethod(amount, method)

		if netAmount.LessThanOrEqual(decimal.Zero) {
			t.Fatalf("PROPERTY VIOLATION: Net amount for $%s via %s should be positive, got $%s",
				amount.String(), method, netAmount.String())
		}
	})
}

// TestProperty_WithdrawalFee_FeeEqualsAmountMinusNet tests fee + net = amount
// *For any* withdrawal amount, the sum of fee and net amount SHALL equal the original amount.
// **Validates: Withdrawal Fee Requirements**
func TestProperty_WithdrawalFee_FeeEqualsAmountMinusNet(t *testing.T) {
	config := models.DefaultWithdrawalConfig()
	svc := NewService(nil, config)

	methods := []models.WithdrawalMethod{
		models.WithdrawalMethodStripe,
		models.WithdrawalMethodCrypto,
		models.WithdrawalMethodBank,
	}

	rapid.Check(t, func(rt *rapid.T) {
		amountFloat := rapid.Float64Range(0.01, 10000.0).Draw(rt, "amountFloat")
		amount := decimal.NewFromFloat(amountFloat).Round(2)
		method := methods[rapid.IntRange(0, len(methods)-1).Draw(rt, "methodIndex")]

		fee := svc.CalculateFeeForMethod(amount, method)
		netAmount := svc.CalculateNetAmountForMethod(amount, method)

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

// TestProperty_WithdrawalFee_CryptoDiscount tests that crypto has lower fees than Stripe
// *For any* withdrawal amount, crypto withdrawal fee SHALL be less than or equal to Stripe fee.
// **Validates: Withdrawal Fee Requirements**
func TestProperty_WithdrawalFee_CryptoDiscount(t *testing.T) {
	config := models.DefaultWithdrawalConfig()
	svc := NewService(nil, config)

	rapid.Check(t, func(rt *rapid.T) {
		amountFloat := rapid.Float64Range(10.0, 10000.0).Draw(rt, "amountFloat")
		amount := decimal.NewFromFloat(amountFloat).Round(2)

		stripeFee := svc.CalculateFeeForMethod(amount, models.WithdrawalMethodStripe)
		cryptoFee := svc.CalculateFeeForMethod(amount, models.WithdrawalMethodCrypto)

		if cryptoFee.GreaterThan(stripeFee) {
			t.Fatalf("PROPERTY VIOLATION: Crypto fee ($%s) should be <= Stripe fee ($%s) for amount $%s",
				cryptoFee.String(), stripeFee.String(), amount.String())
		}
	})
}

// TestProperty_WithdrawalFee_BankMinimumFee tests that bank transfers have minimum fee
// *For any* small withdrawal amount via bank, the fee SHALL be at least $1.00.
// **Validates: Withdrawal Fee Requirements**
func TestProperty_WithdrawalFee_BankMinimumFee(t *testing.T) {
	config := models.DefaultWithdrawalConfig()
	svc := NewService(nil, config)

	rapid.Check(t, func(rt *rapid.T) {
		// Generate small amounts where base fee would be less than $1
		amountFloat := rapid.Float64Range(10.0, 39.0).Draw(rt, "amountFloat") // 2.5% of $40 = $1
		amount := decimal.NewFromFloat(amountFloat).Round(2)

		bankFee := svc.CalculateFeeForMethod(amount, models.WithdrawalMethodBank)
		minFee := decimal.NewFromFloat(1.00)

		if bankFee.LessThan(minFee) {
			t.Fatalf("PROPERTY VIOLATION: Bank fee ($%s) should be at least $1.00 for amount $%s",
				bankFee.String(), amount.String())
		}
	})
}

// TestProperty_WithdrawalFee_BreakdownConsistency tests fee breakdown consistency
// *For any* withdrawal, the fee breakdown SHALL be internally consistent.
// **Validates: Withdrawal Fee Requirements**
func TestProperty_WithdrawalFee_BreakdownConsistency(t *testing.T) {
	config := models.DefaultWithdrawalConfig()
	svc := NewService(nil, config)

	methods := []models.WithdrawalMethod{
		models.WithdrawalMethodStripe,
		models.WithdrawalMethodCrypto,
		models.WithdrawalMethodBank,
	}

	rapid.Check(t, func(rt *rapid.T) {
		amountFloat := rapid.Float64Range(10.0, 10000.0).Draw(rt, "amountFloat")
		amount := decimal.NewFromFloat(amountFloat).Round(2)
		method := methods[rapid.IntRange(0, len(methods)-1).Draw(rt, "methodIndex")]

		breakdown := svc.CalculateFeeBreakdown(amount, method)

		// Verify gross amount matches input
		if !breakdown.GrossAmount.Equal(amount) {
			t.Fatalf("PROPERTY VIOLATION: Breakdown gross amount ($%s) should equal input ($%s)",
				breakdown.GrossAmount.String(), amount.String())
		}

		// Verify total fee + net amount = gross amount
		sum := breakdown.TotalFee.Add(breakdown.NetAmount)
		diff := sum.Sub(breakdown.GrossAmount).Abs()
		tolerance := decimal.NewFromFloat(0.00000001)
		
		if diff.GreaterThan(tolerance) {
			t.Fatalf("PROPERTY VIOLATION: TotalFee ($%s) + NetAmount ($%s) should equal GrossAmount ($%s)",
				breakdown.TotalFee.String(), breakdown.NetAmount.String(), breakdown.GrossAmount.String())
		}

		// Verify fee percentage is reasonable (0-100%)
		if breakdown.FeePercentage.LessThan(decimal.Zero) || breakdown.FeePercentage.GreaterThan(decimal.NewFromInt(100)) {
			t.Fatalf("PROPERTY VIOLATION: Fee percentage (%s%%) should be between 0 and 100",
				breakdown.FeePercentage.String())
		}
	})
}


// ============================================
// Property Tests for Withdrawal Failure Recovery
// ============================================

// TestProperty_WithdrawalFailure_FundsRestored tests that funds are restored on failure
// *For any* failed withdrawal with restore_funds=true, the creator's balance SHALL be restored.
// **Validates: Withdrawal Failure Recovery Requirements**
func TestProperty_WithdrawalFailure_FundsRestored(t *testing.T) {
	if testDB == nil {
		t.Skip("Test database not available")
	}

	ctx := context.Background()
	
	// Check if withdrawals table exists
	var exists bool
	err := testDB.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_name = 'withdrawals'
		)
	`).Scan(&exists)
	if err != nil || !exists {
		t.Skip("Withdrawals table not available - run migrations first")
	}

	config := models.DefaultWithdrawalConfig()
	svc := NewService(testDB, config)

	rapid.Check(t, func(rt *rapid.T) {
		// Generate initial balance and withdrawal amount
		minFloat, _ := config.MinimumAmount.Float64()
		balanceFloat := rapid.Float64Range(minFloat*2, minFloat*10).Draw(rt, "balanceFloat")
		initialBalance := decimal.NewFromFloat(balanceFloat).Round(2)

		amountFloat := rapid.Float64Range(minFloat, balanceFloat).Draw(rt, "amountFloat")
		amount := decimal.NewFromFloat(amountFloat).Round(2)

		// Create a test creator
		creatorID := createTestCreator(t, ctx, initialBalance)
		defer cleanupTestCreator(t, ctx, creatorID)

		// Create withdrawal
		req := &CreateWithdrawalRequest{
			Amount:           amount,
			WithdrawalMethod: models.WithdrawalMethodStripe,
		}

		resp, err := svc.CreateWithdrawal(ctx, creatorID, req)
		if err != nil {
			t.Fatalf("Failed to create withdrawal: %v", err)
		}
		defer cleanupWithdrawals(t, ctx, creatorID)

		// Verify balance was deducted
		earningsAfterWithdrawal, _ := svc.GetEarningsInfo(ctx, creatorID)
		expectedAfterWithdrawal := initialBalance.Sub(amount)
		if !earningsAfterWithdrawal.PendingEarnings.Equal(expectedAfterWithdrawal) {
			t.Fatalf("Balance after withdrawal should be $%s, got $%s",
				expectedAfterWithdrawal.String(), earningsAfterWithdrawal.PendingEarnings.String())
		}

		// Fail the withdrawal with funds restoration
		err = svc.FailWithdrawalWithOptions(ctx, resp.Withdrawal.ID, &FailWithdrawalRequest{
			Reason:       FailureReasonProviderError,
			Description:  "Test failure",
			RestoreFunds: true,
		})
		if err != nil {
			t.Fatalf("Failed to fail withdrawal: %v", err)
		}

		// Verify balance was restored
		earningsAfterFailure, _ := svc.GetEarningsInfo(ctx, creatorID)
		if !earningsAfterFailure.PendingEarnings.Equal(initialBalance) {
			t.Fatalf("PROPERTY VIOLATION: After failed withdrawal with restore_funds=true, balance should be restored to $%s, got $%s",
				initialBalance.String(), earningsAfterFailure.PendingEarnings.String())
		}
	})
}

// TestProperty_WithdrawalFailure_StatusUpdated tests that status is correctly updated on failure
// *For any* failed withdrawal, the status SHALL be updated to 'failed'.
// **Validates: Withdrawal Failure Recovery Requirements**
func TestProperty_WithdrawalFailure_StatusUpdated(t *testing.T) {
	if testDB == nil {
		t.Skip("Test database not available")
	}

	ctx := context.Background()
	
	// Check if withdrawals table exists
	var exists bool
	err := testDB.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_name = 'withdrawals'
		)
	`).Scan(&exists)
	if err != nil || !exists {
		t.Skip("Withdrawals table not available - run migrations first")
	}

	config := models.DefaultWithdrawalConfig()
	svc := NewService(testDB, config)

	rapid.Check(t, func(rt *rapid.T) {
		// Generate balance and amount
		minFloat, _ := config.MinimumAmount.Float64()
		balanceFloat := rapid.Float64Range(minFloat*2, minFloat*10).Draw(rt, "balanceFloat")
		balance := decimal.NewFromFloat(balanceFloat).Round(2)

		amountFloat := rapid.Float64Range(minFloat, balanceFloat).Draw(rt, "amountFloat")
		amount := decimal.NewFromFloat(amountFloat).Round(2)

		// Create a test creator
		creatorID := createTestCreator(t, ctx, balance)
		defer cleanupTestCreator(t, ctx, creatorID)

		// Create withdrawal
		req := &CreateWithdrawalRequest{
			Amount:           amount,
			WithdrawalMethod: models.WithdrawalMethodStripe,
		}

		resp, err := svc.CreateWithdrawal(ctx, creatorID, req)
		if err != nil {
			t.Fatalf("Failed to create withdrawal: %v", err)
		}
		defer cleanupWithdrawals(t, ctx, creatorID)

		// Fail the withdrawal
		failureReason := "Test failure reason"
		err = svc.FailWithdrawal(ctx, resp.Withdrawal.ID, failureReason)
		if err != nil {
			t.Fatalf("Failed to fail withdrawal: %v", err)
		}

		// Verify status is updated
		withdrawal, err := svc.GetWithdrawalByID(ctx, resp.Withdrawal.ID)
		if err != nil {
			t.Fatalf("Failed to get withdrawal: %v", err)
		}

		if withdrawal.Status != models.WithdrawalStatusFailed {
			t.Fatalf("PROPERTY VIOLATION: Failed withdrawal should have status 'failed', got '%s'",
				withdrawal.Status)
		}

		if withdrawal.FailureReason == nil || *withdrawal.FailureReason == "" {
			t.Fatal("PROPERTY VIOLATION: Failed withdrawal should have a failure reason")
		}

		if withdrawal.FailedAt == nil {
			t.Fatal("PROPERTY VIOLATION: Failed withdrawal should have failed_at timestamp")
		}
	})
}

// TestProperty_WithdrawalFailure_CannotFailTwice tests that a withdrawal cannot be failed twice
// *For any* already failed withdrawal, attempting to fail it again SHALL return an error.
// **Validates: Withdrawal Failure Recovery Requirements**
func TestProperty_WithdrawalFailure_CannotFailTwice(t *testing.T) {
	if testDB == nil {
		t.Skip("Test database not available")
	}

	ctx := context.Background()
	
	// Check if withdrawals table exists
	var exists bool
	err := testDB.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_name = 'withdrawals'
		)
	`).Scan(&exists)
	if err != nil || !exists {
		t.Skip("Withdrawals table not available - run migrations first")
	}

	config := models.DefaultWithdrawalConfig()
	svc := NewService(testDB, config)

	// Create a test creator with sufficient balance
	balance := config.MinimumAmount.Mul(decimal.NewFromInt(5))
	creatorID := createTestCreator(t, ctx, balance)
	defer cleanupTestCreator(t, ctx, creatorID)

	// Create withdrawal
	req := &CreateWithdrawalRequest{
		Amount:           config.MinimumAmount,
		WithdrawalMethod: models.WithdrawalMethodStripe,
	}

	resp, err := svc.CreateWithdrawal(ctx, creatorID, req)
	if err != nil {
		t.Fatalf("Failed to create withdrawal: %v", err)
	}
	defer cleanupWithdrawals(t, ctx, creatorID)

	// Fail the withdrawal first time
	err = svc.FailWithdrawal(ctx, resp.Withdrawal.ID, "First failure")
	if err != nil {
		t.Fatalf("Failed to fail withdrawal first time: %v", err)
	}

	// Try to fail again - should return error
	err = svc.FailWithdrawal(ctx, resp.Withdrawal.ID, "Second failure")
	if err != ErrWithdrawalAlreadyDone {
		t.Fatalf("PROPERTY VIOLATION: Failing an already failed withdrawal should return ErrWithdrawalAlreadyDone, got: %v", err)
	}
}

// TestProperty_WithdrawalFailure_CannotFailCompleted tests that completed withdrawals cannot be failed
// *For any* completed withdrawal, attempting to fail it SHALL return an error.
// **Validates: Withdrawal Failure Recovery Requirements**
func TestProperty_WithdrawalFailure_CannotFailCompleted(t *testing.T) {
	if testDB == nil {
		t.Skip("Test database not available")
	}

	ctx := context.Background()
	
	// Check if withdrawals table exists
	var exists bool
	err := testDB.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_name = 'withdrawals'
		)
	`).Scan(&exists)
	if err != nil || !exists {
		t.Skip("Withdrawals table not available - run migrations first")
	}

	config := models.DefaultWithdrawalConfig()
	svc := NewService(testDB, config)

	// Create a test creator with sufficient balance
	balance := config.MinimumAmount.Mul(decimal.NewFromInt(5))
	creatorID := createTestCreator(t, ctx, balance)
	defer cleanupTestCreator(t, ctx, creatorID)

	// Create withdrawal
	req := &CreateWithdrawalRequest{
		Amount:           config.MinimumAmount,
		WithdrawalMethod: models.WithdrawalMethodStripe,
	}

	resp, err := svc.CreateWithdrawal(ctx, creatorID, req)
	if err != nil {
		t.Fatalf("Failed to create withdrawal: %v", err)
	}
	defer cleanupWithdrawals(t, ctx, creatorID)

	// Complete the withdrawal
	err = svc.CompleteWithdrawal(ctx, resp.Withdrawal.ID, "tx_test_123")
	if err != nil {
		t.Fatalf("Failed to complete withdrawal: %v", err)
	}

	// Try to fail - should return error
	err = svc.FailWithdrawal(ctx, resp.Withdrawal.ID, "Trying to fail completed")
	if err != ErrWithdrawalAlreadyDone {
		t.Fatalf("PROPERTY VIOLATION: Failing a completed withdrawal should return ErrWithdrawalAlreadyDone, got: %v", err)
	}
}

// TestProperty_WithdrawalFailure_NoRestoreFunds tests that funds are not restored when restore_funds=false
// *For any* failed withdrawal with restore_funds=false, the creator's balance SHALL NOT be restored.
// **Validates: Withdrawal Failure Recovery Requirements**
func TestProperty_WithdrawalFailure_NoRestoreFunds(t *testing.T) {
	if testDB == nil {
		t.Skip("Test database not available")
	}

	ctx := context.Background()
	
	// Check if withdrawals table exists
	var exists bool
	err := testDB.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_name = 'withdrawals'
		)
	`).Scan(&exists)
	if err != nil || !exists {
		t.Skip("Withdrawals table not available - run migrations first")
	}

	config := models.DefaultWithdrawalConfig()
	svc := NewService(testDB, config)

	// Create a test creator with sufficient balance
	initialBalance := config.MinimumAmount.Mul(decimal.NewFromInt(5))
	creatorID := createTestCreator(t, ctx, initialBalance)
	defer cleanupTestCreator(t, ctx, creatorID)

	// Create withdrawal
	amount := config.MinimumAmount.Mul(decimal.NewFromInt(2))
	req := &CreateWithdrawalRequest{
		Amount:           amount,
		WithdrawalMethod: models.WithdrawalMethodStripe,
	}

	resp, err := svc.CreateWithdrawal(ctx, creatorID, req)
	if err != nil {
		t.Fatalf("Failed to create withdrawal: %v", err)
	}
	defer cleanupWithdrawals(t, ctx, creatorID)

	// Get balance after withdrawal
	earningsAfterWithdrawal, _ := svc.GetEarningsInfo(ctx, creatorID)
	balanceAfterWithdrawal := earningsAfterWithdrawal.PendingEarnings

	// Fail the withdrawal WITHOUT restoring funds
	err = svc.FailWithdrawalWithOptions(ctx, resp.Withdrawal.ID, &FailWithdrawalRequest{
		Reason:       FailureReasonRejected,
		Description:  "Rejected by provider",
		RestoreFunds: false,
	})
	if err != nil {
		t.Fatalf("Failed to fail withdrawal: %v", err)
	}

	// Verify balance was NOT restored
	earningsAfterFailure, _ := svc.GetEarningsInfo(ctx, creatorID)
	if !earningsAfterFailure.PendingEarnings.Equal(balanceAfterWithdrawal) {
		t.Fatalf("PROPERTY VIOLATION: After failed withdrawal with restore_funds=false, balance should remain $%s, got $%s",
			balanceAfterWithdrawal.String(), earningsAfterFailure.PendingEarnings.String())
	}
}

// TestProperty_FailureReason_Retryable tests that retryable failure reasons are correctly identified
// *For any* failure reason, IsRetryable() SHALL return true only for transient errors.
// **Validates: Withdrawal Failure Recovery Requirements**
func TestProperty_FailureReason_Retryable(t *testing.T) {
	// Define expected retryable reasons
	retryableReasons := map[FailureReason]bool{
		FailureReasonProviderError: true,
		FailureReasonNetworkError:  true,
		FailureReasonTimeout:       true,
	}

	// Define expected non-retryable reasons
	nonRetryableReasons := map[FailureReason]bool{
		FailureReasonInsufficientFunds:  false,
		FailureReasonInvalidDestination: false,
		FailureReasonRejected:           false,
		FailureReasonUnknown:            false,
	}

	// Test retryable reasons
	for reason, expected := range retryableReasons {
		if reason.IsRetryable() != expected {
			t.Fatalf("PROPERTY VIOLATION: FailureReason %s.IsRetryable() should be %v", reason, expected)
		}
	}

	// Test non-retryable reasons
	for reason, expected := range nonRetryableReasons {
		if reason.IsRetryable() != expected {
			t.Fatalf("PROPERTY VIOLATION: FailureReason %s.IsRetryable() should be %v", reason, expected)
		}
	}
}

// TestProperty_ExtractFailureReason tests that failure reasons are correctly extracted from strings
// *For any* failure reason string in the format "[reason] description", extractFailureReason SHALL return the correct reason.
// **Validates: Withdrawal Failure Recovery Requirements**
func TestProperty_ExtractFailureReason(t *testing.T) {
	testCases := []struct {
		input    string
		expected FailureReason
	}{
		{"[provider_error] Connection timeout", FailureReasonProviderError},
		{"[network_error] DNS resolution failed", FailureReasonNetworkError},
		{"[timeout] Request timed out", FailureReasonTimeout},
		{"[rejected] Transaction rejected", FailureReasonRejected},
		{"[insufficient_funds] Not enough balance", FailureReasonInsufficientFunds},
		{"[invalid_destination] Invalid wallet address", FailureReasonInvalidDestination},
		{"[unknown] Something went wrong", FailureReasonUnknown},
		{"No brackets here", FailureReasonUnknown},
		{"", FailureReasonUnknown},
		{"[", FailureReasonUnknown},
		{"[]", FailureReasonUnknown},
	}

	for _, tc := range testCases {
		result := extractFailureReason(tc.input)
		if result != tc.expected {
			t.Fatalf("PROPERTY VIOLATION: extractFailureReason(%q) should return %s, got %s",
				tc.input, tc.expected, result)
		}
	}
}
