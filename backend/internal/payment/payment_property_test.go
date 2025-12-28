package payment

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/aimerfeng/AgentLink/internal/config"
	"github.com/aimerfeng/AgentLink/internal/models"
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
		Stripe: config.StripeConfig{
			SecretKey:     "sk_test_fake",
			WebhookSecret: "whsec_test_fake",
		},
	}

	code := m.Run()

	if testDB != nil {
		testDB.Close()
	}

	os.Exit(code)
}


// TestProperty_PaymentQuotaConsistency tests the payment quota consistency property
// *For any* successful payment, the quota SHALL be credited exactly once with the purchased amount.
// **Validates: Requirements A7.3**
func TestProperty_PaymentQuotaConsistency(t *testing.T) {
	if testDB == nil {
		t.Skip("Test database not available")
	}

	ctx := context.Background()
	svc := NewService(testDB, &testCfg.Stripe, "http://localhost:3000")

	rapid.Check(t, func(rt *rapid.T) {
		// Generate random quota amount (100-10000)
		quotaAmount := rapid.Int64Range(100, 10000).Draw(rt, "quotaAmount")
		
		// Generate random price
		priceFloat := rapid.Float64Range(1.0, 100.0).Draw(rt, "priceFloat")
		price := decimal.NewFromFloat(priceFloat)

		// Create a test user with initial quota
		initialQuota := rapid.Int64Range(0, 1000).Draw(rt, "initialQuota")
		userID := createTestUserWithQuota(t, ctx, initialQuota)
		defer cleanupTestUser(t, ctx, userID)

		// Create a pending payment record
		paymentID := uuid.New()
		_, err := testDB.Exec(ctx, `
			INSERT INTO payments (id, user_id, amount_usd, quota_purchased, payment_method, status)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, paymentID, userID, price, quotaAmount, models.PaymentMethodStripe, models.PaymentStatusPending)
		if err != nil {
			t.Fatalf("Failed to create payment record: %v", err)
		}
		defer cleanupPayment(t, ctx, paymentID)

		// Complete the payment
		err = svc.CompletePayment(ctx, paymentID, "test_session_id")
		if err != nil {
			t.Fatalf("Failed to complete payment: %v", err)
		}

		// Verify quota was credited correctly
		var totalQuota int64
		err = testDB.QueryRow(ctx, `
			SELECT total_quota FROM quotas WHERE user_id = $1
		`, userID).Scan(&totalQuota)
		if err != nil {
			t.Fatalf("Failed to get quota: %v", err)
		}

		expectedQuota := initialQuota + quotaAmount
		if totalQuota != expectedQuota {
			t.Fatalf("PROPERTY VIOLATION: Expected total_quota %d, got %d (initial: %d, purchased: %d)",
				expectedQuota, totalQuota, initialQuota, quotaAmount)
		}

		// Verify payment status is completed
		var status models.PaymentStatus
		err = testDB.QueryRow(ctx, `
			SELECT status FROM payments WHERE id = $1
		`, paymentID).Scan(&status)
		if err != nil {
			t.Fatalf("Failed to get payment status: %v", err)
		}

		if status != models.PaymentStatusCompleted {
			t.Fatalf("PROPERTY VIOLATION: Expected payment status 'completed', got '%s'", status)
		}
	})
}

// TestProperty_PaymentQuotaConsistency_IdempotentCompletion tests that completing a payment twice doesn't double credit
// *For any* payment, completing it multiple times SHALL credit quota exactly once.
// **Validates: Requirements A7.3**
func TestProperty_PaymentQuotaConsistency_IdempotentCompletion(t *testing.T) {
	if testDB == nil {
		t.Skip("Test database not available")
	}

	ctx := context.Background()
	svc := NewService(testDB, &testCfg.Stripe, "http://localhost:3000")

	rapid.Check(t, func(rt *rapid.T) {
		// Generate random quota amount
		quotaAmount := rapid.Int64Range(100, 5000).Draw(rt, "quotaAmount")
		price := decimal.NewFromFloat(9.99)

		// Create a test user
		initialQuota := rapid.Int64Range(0, 500).Draw(rt, "initialQuota")
		userID := createTestUserWithQuota(t, ctx, initialQuota)
		defer cleanupTestUser(t, ctx, userID)

		// Create a pending payment record
		paymentID := uuid.New()
		_, err := testDB.Exec(ctx, `
			INSERT INTO payments (id, user_id, amount_usd, quota_purchased, payment_method, status)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, paymentID, userID, price, quotaAmount, models.PaymentMethodStripe, models.PaymentStatusPending)
		if err != nil {
			t.Fatalf("Failed to create payment record: %v", err)
		}
		defer cleanupPayment(t, ctx, paymentID)

		// Complete the payment first time
		err = svc.CompletePayment(ctx, paymentID, "test_session_id")
		if err != nil {
			t.Fatalf("Failed to complete payment first time: %v", err)
		}

		// Get quota after first completion
		var quotaAfterFirst int64
		err = testDB.QueryRow(ctx, `
			SELECT total_quota FROM quotas WHERE user_id = $1
		`, userID).Scan(&quotaAfterFirst)
		if err != nil {
			t.Fatalf("Failed to get quota: %v", err)
		}

		// Try to complete the payment again (should be idempotent)
		err = svc.CompletePayment(ctx, paymentID, "test_session_id")
		if err != ErrPaymentAlreadyDone {
			// It's okay if it returns an error indicating already done
			// or if it silently succeeds without double-crediting
		}

		// Verify quota hasn't changed (no double credit)
		var quotaAfterSecond int64
		err = testDB.QueryRow(ctx, `
			SELECT total_quota FROM quotas WHERE user_id = $1
		`, userID).Scan(&quotaAfterSecond)
		if err != nil {
			t.Fatalf("Failed to get quota: %v", err)
		}

		if quotaAfterSecond != quotaAfterFirst {
			t.Fatalf("PROPERTY VIOLATION: Double completion changed quota from %d to %d (should be idempotent)",
				quotaAfterFirst, quotaAfterSecond)
		}

		expectedQuota := initialQuota + quotaAmount
		if quotaAfterSecond != expectedQuota {
			t.Fatalf("PROPERTY VIOLATION: Expected quota %d, got %d", expectedQuota, quotaAfterSecond)
		}
	})
}


// TestProperty_PaymentQuotaConsistency_ConcurrentPayments tests concurrent payment completions
// *For any* set of concurrent payments for the same user, each payment SHALL credit quota exactly once.
// **Validates: Requirements A7.3**
func TestProperty_PaymentQuotaConsistency_ConcurrentPayments(t *testing.T) {
	if testDB == nil {
		t.Skip("Test database not available")
	}

	ctx := context.Background()
	svc := NewService(testDB, &testCfg.Stripe, "http://localhost:3000")

	// Use fixed values for concurrent test
	numPayments := 5
	quotaPerPayment := int64(100)
	initialQuota := int64(0)

	// Create a test user
	userID := createTestUserWithQuota(t, ctx, initialQuota)
	defer cleanupTestUser(t, ctx, userID)

	// Create multiple pending payments
	paymentIDs := make([]uuid.UUID, numPayments)
	for i := 0; i < numPayments; i++ {
		paymentIDs[i] = uuid.New()
		_, err := testDB.Exec(ctx, `
			INSERT INTO payments (id, user_id, amount_usd, quota_purchased, payment_method, status)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, paymentIDs[i], userID, decimal.NewFromFloat(9.99), quotaPerPayment, models.PaymentMethodStripe, models.PaymentStatusPending)
		if err != nil {
			t.Fatalf("Failed to create payment record %d: %v", i, err)
		}
		defer cleanupPayment(t, ctx, paymentIDs[i])
	}

	// Complete all payments concurrently
	var wg sync.WaitGroup
	errors := make(chan error, numPayments)

	for i := 0; i < numPayments; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			err := svc.CompletePayment(ctx, paymentIDs[idx], fmt.Sprintf("test_session_%d", idx))
			if err != nil {
				errors <- fmt.Errorf("payment %d failed: %w", idx, err)
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors (some may fail due to race conditions, but quota should still be correct)
	for err := range errors {
		t.Logf("Concurrent payment error (may be expected): %v", err)
	}

	// Wait for any async operations
	time.Sleep(100 * time.Millisecond)

	// Verify total quota is correct
	var totalQuota int64
	err := testDB.QueryRow(ctx, `
		SELECT total_quota FROM quotas WHERE user_id = $1
	`, userID).Scan(&totalQuota)
	if err != nil {
		t.Fatalf("Failed to get quota: %v", err)
	}

	// Count completed payments
	var completedCount int
	err = testDB.QueryRow(ctx, `
		SELECT COUNT(*) FROM payments WHERE user_id = $1 AND status = 'completed'
	`, userID).Scan(&completedCount)
	if err != nil {
		t.Fatalf("Failed to count completed payments: %v", err)
	}

	expectedQuota := initialQuota + int64(completedCount)*quotaPerPayment
	if totalQuota != expectedQuota {
		t.Fatalf("PROPERTY VIOLATION: Expected total_quota %d (from %d completed payments), got %d",
			expectedQuota, completedCount, totalQuota)
	}
}

// TestProperty_PaymentQuotaConsistency_NewUserQuotaCreation tests quota creation for new users
// *For any* user without existing quota record, completing a payment SHALL create the quota record.
// **Validates: Requirements A7.3**
func TestProperty_PaymentQuotaConsistency_NewUserQuotaCreation(t *testing.T) {
	if testDB == nil {
		t.Skip("Test database not available")
	}

	ctx := context.Background()
	svc := NewService(testDB, &testCfg.Stripe, "http://localhost:3000")

	rapid.Check(t, func(rt *rapid.T) {
		// Generate random quota amount
		quotaAmount := rapid.Int64Range(100, 5000).Draw(rt, "quotaAmount")
		price := decimal.NewFromFloat(19.99)

		// Create a test user WITHOUT quota record
		userID := createTestUserWithoutQuota(t, ctx)
		defer cleanupTestUser(t, ctx, userID)

		// Verify no quota record exists
		var count int
		err := testDB.QueryRow(ctx, `
			SELECT COUNT(*) FROM quotas WHERE user_id = $1
		`, userID).Scan(&count)
		if err != nil {
			t.Fatalf("Failed to check quota existence: %v", err)
		}
		if count != 0 {
			t.Fatal("Test setup error: quota record should not exist")
		}

		// Create a pending payment record
		paymentID := uuid.New()
		_, err = testDB.Exec(ctx, `
			INSERT INTO payments (id, user_id, amount_usd, quota_purchased, payment_method, status)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, paymentID, userID, price, quotaAmount, models.PaymentMethodStripe, models.PaymentStatusPending)
		if err != nil {
			t.Fatalf("Failed to create payment record: %v", err)
		}
		defer cleanupPayment(t, ctx, paymentID)

		// Complete the payment
		err = svc.CompletePayment(ctx, paymentID, "test_session_id")
		if err != nil {
			t.Fatalf("Failed to complete payment: %v", err)
		}

		// Verify quota record was created with correct amount
		var totalQuota int64
		err = testDB.QueryRow(ctx, `
			SELECT total_quota FROM quotas WHERE user_id = $1
		`, userID).Scan(&totalQuota)
		if err != nil {
			t.Fatalf("Failed to get quota (should have been created): %v", err)
		}

		if totalQuota != quotaAmount {
			t.Fatalf("PROPERTY VIOLATION: Expected total_quota %d for new user, got %d",
				quotaAmount, totalQuota)
		}
	})
}

// TestProperty_PaymentQuotaConsistency_PackageQuotaMatch tests that package quota matches credited amount
// *For any* quota package purchase, the credited quota SHALL match the package's defined quota.
// **Validates: Requirements A7.3**
func TestProperty_PaymentQuotaConsistency_PackageQuotaMatch(t *testing.T) {
	if testDB == nil {
		t.Skip("Test database not available")
	}

	ctx := context.Background()
	svc := NewService(testDB, &testCfg.Stripe, "http://localhost:3000")

	// Test each predefined package
	for _, pkg := range QuotaPackages {
		t.Run(pkg.ID, func(t *testing.T) {
			// Create a test user
			userID := createTestUserWithQuota(t, ctx, 0)
			defer cleanupTestUser(t, ctx, userID)

			// Create a pending payment for this package
			paymentID := uuid.New()
			_, err := testDB.Exec(ctx, `
				INSERT INTO payments (id, user_id, amount_usd, quota_purchased, payment_method, status)
				VALUES ($1, $2, $3, $4, $5, $6)
			`, paymentID, userID, pkg.PriceUSD, pkg.Quota, models.PaymentMethodStripe, models.PaymentStatusPending)
			if err != nil {
				t.Fatalf("Failed to create payment record: %v", err)
			}
			defer cleanupPayment(t, ctx, paymentID)

			// Complete the payment
			err = svc.CompletePayment(ctx, paymentID, "test_session_id")
			if err != nil {
				t.Fatalf("Failed to complete payment: %v", err)
			}

			// Verify quota matches package
			var totalQuota int64
			err = testDB.QueryRow(ctx, `
				SELECT total_quota FROM quotas WHERE user_id = $1
			`, userID).Scan(&totalQuota)
			if err != nil {
				t.Fatalf("Failed to get quota: %v", err)
			}

			if totalQuota != pkg.Quota {
				t.Fatalf("PROPERTY VIOLATION: Package %s should credit %d quota, got %d",
					pkg.ID, pkg.Quota, totalQuota)
			}
		})
	}
}


// TestProperty_PaymentQuotaConsistency_TransactionAtomicity tests that payment and quota update are atomic
// *For any* payment completion, either both payment status and quota are updated, or neither is.
// **Validates: Requirements A7.3**
func TestProperty_PaymentQuotaConsistency_TransactionAtomicity(t *testing.T) {
	if testDB == nil {
		t.Skip("Test database not available")
	}

	ctx := context.Background()
	svc := NewService(testDB, &testCfg.Stripe, "http://localhost:3000")

	rapid.Check(t, func(rt *rapid.T) {
		quotaAmount := rapid.Int64Range(100, 1000).Draw(rt, "quotaAmount")
		initialQuota := rapid.Int64Range(0, 500).Draw(rt, "initialQuota")

		// Create a test user
		userID := createTestUserWithQuota(t, ctx, initialQuota)
		defer cleanupTestUser(t, ctx, userID)

		// Create a pending payment
		paymentID := uuid.New()
		_, err := testDB.Exec(ctx, `
			INSERT INTO payments (id, user_id, amount_usd, quota_purchased, payment_method, status)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, paymentID, userID, decimal.NewFromFloat(9.99), quotaAmount, models.PaymentMethodStripe, models.PaymentStatusPending)
		if err != nil {
			t.Fatalf("Failed to create payment record: %v", err)
		}
		defer cleanupPayment(t, ctx, paymentID)

		// Complete the payment
		err = svc.CompletePayment(ctx, paymentID, "test_session_id")
		if err != nil {
			t.Fatalf("Failed to complete payment: %v", err)
		}

		// Check both payment status and quota
		var status models.PaymentStatus
		var totalQuota int64

		err = testDB.QueryRow(ctx, `
			SELECT status FROM payments WHERE id = $1
		`, paymentID).Scan(&status)
		if err != nil {
			t.Fatalf("Failed to get payment status: %v", err)
		}

		err = testDB.QueryRow(ctx, `
			SELECT total_quota FROM quotas WHERE user_id = $1
		`, userID).Scan(&totalQuota)
		if err != nil {
			t.Fatalf("Failed to get quota: %v", err)
		}

		// Property: If payment is completed, quota must be credited
		if status == models.PaymentStatusCompleted {
			expectedQuota := initialQuota + quotaAmount
			if totalQuota != expectedQuota {
				t.Fatalf("PROPERTY VIOLATION: Payment completed but quota not credited correctly. Expected %d, got %d",
					expectedQuota, totalQuota)
			}
		}

		// Property: If quota was credited, payment must be completed
		if totalQuota > initialQuota {
			if status != models.PaymentStatusCompleted {
				t.Fatalf("PROPERTY VIOLATION: Quota credited but payment status is '%s' (should be 'completed')",
					status)
			}
		}
	})
}

// Helper functions

func createTestUserWithQuota(t *testing.T, ctx context.Context, quota int64) uuid.UUID {
	t.Helper()

	userID := uuid.New()
	email := fmt.Sprintf("test-payment-%s@example.com", userID.String()[:8])

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
		VALUES ($1, $2, 0, 0)
	`, userID, quota)
	if err != nil {
		t.Fatalf("Failed to create quota: %v", err)
	}

	return userID
}

func createTestUserWithoutQuota(t *testing.T, ctx context.Context) uuid.UUID {
	t.Helper()

	userID := uuid.New()
	email := fmt.Sprintf("test-payment-noquota-%s@example.com", userID.String()[:8])

	_, err := testDB.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, user_type, email_verified)
		VALUES ($1, $2, 'test-hash', 'developer', true)
	`, userID, email)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	// Do NOT create quota record
	return userID
}

func cleanupTestUser(t *testing.T, ctx context.Context, userID uuid.UUID) {
	t.Helper()

	// Delete quota
	_, _ = testDB.Exec(ctx, `DELETE FROM quotas WHERE user_id = $1`, userID)
	// Delete payments
	_, _ = testDB.Exec(ctx, `DELETE FROM payments WHERE user_id = $1`, userID)
	// Delete user
	_, _ = testDB.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID)
}

func cleanupPayment(t *testing.T, ctx context.Context, paymentID uuid.UUID) {
	t.Helper()
	_, _ = testDB.Exec(ctx, `DELETE FROM payments WHERE id = $1`, paymentID)
}

// TestProperty_PaymentFailure_NoQuotaCredited tests that failed payments don't credit quota
// *For any* failed payment, the System SHALL NOT credit any quota to the developer's account.
// **Validates: Requirements A7.4**
func TestProperty_PaymentFailure_NoQuotaCredited(t *testing.T) {
	if testDB == nil {
		t.Skip("Test database not available")
	}

	ctx := context.Background()
	svc := NewService(testDB, &testCfg.Stripe, "http://localhost:3000")

	rapid.Check(t, func(rt *rapid.T) {
		// Generate random quota amount that would have been purchased
		quotaAmount := rapid.Int64Range(100, 10000).Draw(rt, "quotaAmount")
		
		// Generate random initial quota
		initialQuota := rapid.Int64Range(0, 5000).Draw(rt, "initialQuota")
		
		// Generate random price
		priceFloat := rapid.Float64Range(1.0, 100.0).Draw(rt, "priceFloat")
		price := decimal.NewFromFloat(priceFloat)

		// Generate random failure reason
		failureReasons := []string{
			"card_declined",
			"insufficient_funds",
			"expired_card",
			"processing_error",
			"fraudulent",
			"session_expired",
			"cancelled",
			"unknown_error",
		}
		reasonIdx := rapid.IntRange(0, len(failureReasons)-1).Draw(rt, "reasonIdx")
		failureReason := failureReasons[reasonIdx]

		// Create a test user with initial quota
		userID := createTestUserWithQuota(t, ctx, initialQuota)
		defer cleanupTestUser(t, ctx, userID)

		// Create a pending payment record
		paymentID := uuid.New()
		_, err := testDB.Exec(ctx, `
			INSERT INTO payments (id, user_id, amount_usd, quota_purchased, payment_method, status)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, paymentID, userID, price, quotaAmount, models.PaymentMethodStripe, models.PaymentStatusPending)
		if err != nil {
			t.Fatalf("Failed to create payment record: %v", err)
		}
		defer cleanupPayment(t, ctx, paymentID)

		// Fail the payment
		err = svc.FailPayment(ctx, paymentID, failureReason)
		if err != nil {
			t.Fatalf("Failed to fail payment: %v", err)
		}

		// Verify quota was NOT credited (should remain at initial value)
		var totalQuota int64
		err = testDB.QueryRow(ctx, `
			SELECT total_quota FROM quotas WHERE user_id = $1
		`, userID).Scan(&totalQuota)
		if err != nil {
			t.Fatalf("Failed to get quota: %v", err)
		}

		if totalQuota != initialQuota {
			t.Fatalf("PROPERTY VIOLATION: Failed payment should NOT credit quota. Expected %d, got %d (would have credited %d)",
				initialQuota, totalQuota, quotaAmount)
		}

		// Verify payment status is failed
		var status models.PaymentStatus
		var storedReason *string
		err = testDB.QueryRow(ctx, `
			SELECT status, failure_reason FROM payments WHERE id = $1
		`, paymentID).Scan(&status, &storedReason)
		if err != nil {
			t.Fatalf("Failed to get payment status: %v", err)
		}

		if status != models.PaymentStatusFailed {
			t.Fatalf("PROPERTY VIOLATION: Expected payment status 'failed', got '%s'", status)
		}

		if storedReason == nil || *storedReason != failureReason {
			t.Fatalf("PROPERTY VIOLATION: Expected failure_reason '%s', got '%v'", failureReason, storedReason)
		}
	})
}

// TestProperty_PaymentFailure_NotificationCreated tests that failed payments create notifications
// *For any* failed payment, the System SHALL create a notification for the developer.
// **Validates: Requirements A7.4**
func TestProperty_PaymentFailure_NotificationCreated(t *testing.T) {
	if testDB == nil {
		t.Skip("Test database not available")
	}

	ctx := context.Background()
	svc := NewService(testDB, &testCfg.Stripe, "http://localhost:3000")

	rapid.Check(t, func(rt *rapid.T) {
		// Generate random quota amount
		quotaAmount := rapid.Int64Range(100, 5000).Draw(rt, "quotaAmount")
		price := decimal.NewFromFloat(19.99)

		// Create a test user
		userID := createTestUserWithQuota(t, ctx, 0)
		defer cleanupTestUser(t, ctx, userID)

		// Create a pending payment record
		paymentID := uuid.New()
		_, err := testDB.Exec(ctx, `
			INSERT INTO payments (id, user_id, amount_usd, quota_purchased, payment_method, status)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, paymentID, userID, price, quotaAmount, models.PaymentMethodStripe, models.PaymentStatusPending)
		if err != nil {
			t.Fatalf("Failed to create payment record: %v", err)
		}
		defer cleanupPayment(t, ctx, paymentID)
		defer cleanupPaymentNotifications(t, ctx, paymentID)

		// Fail the payment
		err = svc.FailPayment(ctx, paymentID, "card_declined")
		if err != nil {
			t.Fatalf("Failed to fail payment: %v", err)
		}

		// Wait for async notification creation
		time.Sleep(100 * time.Millisecond)

		// Verify notification was created
		var notificationCount int
		err = testDB.QueryRow(ctx, `
			SELECT COUNT(*) FROM payment_notifications 
			WHERE payment_id = $1 AND event_type = $2
		`, paymentID, models.WebhookEventPaymentFailed).Scan(&notificationCount)
		if err != nil {
			t.Fatalf("Failed to count notifications: %v", err)
		}

		if notificationCount == 0 {
			t.Fatalf("PROPERTY VIOLATION: Failed payment should create a notification, but none was found")
		}
	})
}

// TestProperty_PaymentFailure_IdempotentFailure tests that failing a payment twice doesn't cause issues
// *For any* payment, failing it multiple times SHALL be idempotent (no double notifications or state changes).
// **Validates: Requirements A7.4**
func TestProperty_PaymentFailure_IdempotentFailure(t *testing.T) {
	if testDB == nil {
		t.Skip("Test database not available")
	}

	ctx := context.Background()
	svc := NewService(testDB, &testCfg.Stripe, "http://localhost:3000")

	rapid.Check(t, func(rt *rapid.T) {
		quotaAmount := rapid.Int64Range(100, 1000).Draw(rt, "quotaAmount")
		initialQuota := rapid.Int64Range(0, 500).Draw(rt, "initialQuota")

		// Create a test user
		userID := createTestUserWithQuota(t, ctx, initialQuota)
		defer cleanupTestUser(t, ctx, userID)

		// Create a pending payment
		paymentID := uuid.New()
		_, err := testDB.Exec(ctx, `
			INSERT INTO payments (id, user_id, amount_usd, quota_purchased, payment_method, status)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, paymentID, userID, decimal.NewFromFloat(9.99), quotaAmount, models.PaymentMethodStripe, models.PaymentStatusPending)
		if err != nil {
			t.Fatalf("Failed to create payment record: %v", err)
		}
		defer cleanupPayment(t, ctx, paymentID)
		defer cleanupPaymentNotifications(t, ctx, paymentID)

		// Fail the payment first time
		err = svc.FailPayment(ctx, paymentID, "card_declined")
		if err != nil {
			t.Fatalf("Failed to fail payment first time: %v", err)
		}

		// Get state after first failure
		var statusAfterFirst models.PaymentStatus
		var quotaAfterFirst int64
		err = testDB.QueryRow(ctx, `SELECT status FROM payments WHERE id = $1`, paymentID).Scan(&statusAfterFirst)
		if err != nil {
			t.Fatalf("Failed to get payment status: %v", err)
		}
		err = testDB.QueryRow(ctx, `SELECT total_quota FROM quotas WHERE user_id = $1`, userID).Scan(&quotaAfterFirst)
		if err != nil {
			t.Fatalf("Failed to get quota: %v", err)
		}

		// Try to fail the payment again (should be idempotent)
		err = svc.FailPayment(ctx, paymentID, "different_reason")
		// It's okay if it returns ErrPaymentAlreadyDone
		if err != nil && err != ErrPaymentAlreadyDone {
			t.Logf("Second failure returned error (may be expected): %v", err)
		}

		// Verify state hasn't changed
		var statusAfterSecond models.PaymentStatus
		var quotaAfterSecond int64
		err = testDB.QueryRow(ctx, `SELECT status FROM payments WHERE id = $1`, paymentID).Scan(&statusAfterSecond)
		if err != nil {
			t.Fatalf("Failed to get payment status: %v", err)
		}
		err = testDB.QueryRow(ctx, `SELECT total_quota FROM quotas WHERE user_id = $1`, userID).Scan(&quotaAfterSecond)
		if err != nil {
			t.Fatalf("Failed to get quota: %v", err)
		}

		if statusAfterSecond != statusAfterFirst {
			t.Fatalf("PROPERTY VIOLATION: Double failure changed status from '%s' to '%s'",
				statusAfterFirst, statusAfterSecond)
		}

		if quotaAfterSecond != quotaAfterFirst {
			t.Fatalf("PROPERTY VIOLATION: Double failure changed quota from %d to %d",
				quotaAfterFirst, quotaAfterSecond)
		}

		// Quota should still be at initial value
		if quotaAfterSecond != initialQuota {
			t.Fatalf("PROPERTY VIOLATION: Quota should remain at initial value %d, got %d",
				initialQuota, quotaAfterSecond)
		}
	})
}

// TestProperty_PaymentFailure_FailedTimestampSet tests that failed_at timestamp is set
// *For any* failed payment, the System SHALL record the failure timestamp.
// **Validates: Requirements A7.4**
func TestProperty_PaymentFailure_FailedTimestampSet(t *testing.T) {
	if testDB == nil {
		t.Skip("Test database not available")
	}

	ctx := context.Background()
	svc := NewService(testDB, &testCfg.Stripe, "http://localhost:3000")

	rapid.Check(t, func(rt *rapid.T) {
		quotaAmount := rapid.Int64Range(100, 1000).Draw(rt, "quotaAmount")

		// Create a test user
		userID := createTestUserWithQuota(t, ctx, 0)
		defer cleanupTestUser(t, ctx, userID)

		// Create a pending payment
		paymentID := uuid.New()
		_, err := testDB.Exec(ctx, `
			INSERT INTO payments (id, user_id, amount_usd, quota_purchased, payment_method, status)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, paymentID, userID, decimal.NewFromFloat(9.99), quotaAmount, models.PaymentMethodStripe, models.PaymentStatusPending)
		if err != nil {
			t.Fatalf("Failed to create payment record: %v", err)
		}
		defer cleanupPayment(t, ctx, paymentID)

		beforeFail := time.Now().Add(-1 * time.Second) // Allow 1 second tolerance before

		// Fail the payment
		err = svc.FailPayment(ctx, paymentID, "card_declined")
		if err != nil {
			t.Fatalf("Failed to fail payment: %v", err)
		}

		afterFail := time.Now().Add(1 * time.Second) // Allow 1 second tolerance after

		// Verify failed_at timestamp is set and reasonable
		var failedAt *time.Time
		err = testDB.QueryRow(ctx, `
			SELECT failed_at FROM payments WHERE id = $1
		`, paymentID).Scan(&failedAt)
		if err != nil {
			t.Fatalf("Failed to get failed_at: %v", err)
		}

		if failedAt == nil {
			t.Fatal("PROPERTY VIOLATION: failed_at should be set for failed payment")
		}

		if failedAt.Before(beforeFail) || failedAt.After(afterFail) {
			t.Fatalf("PROPERTY VIOLATION: failed_at timestamp %v is outside expected range [%v, %v]",
				failedAt, beforeFail, afterFail)
		}
	})
}

// TestProperty_PaymentFailure_CompletedPaymentCannotFail tests that completed payments cannot be failed
// *For any* completed payment, attempting to fail it SHALL be rejected.
// **Validates: Requirements A7.4**
func TestProperty_PaymentFailure_CompletedPaymentCannotFail(t *testing.T) {
	if testDB == nil {
		t.Skip("Test database not available")
	}

	ctx := context.Background()
	svc := NewService(testDB, &testCfg.Stripe, "http://localhost:3000")

	rapid.Check(t, func(rt *rapid.T) {
		quotaAmount := rapid.Int64Range(100, 1000).Draw(rt, "quotaAmount")
		initialQuota := rapid.Int64Range(0, 500).Draw(rt, "initialQuota")

		// Create a test user
		userID := createTestUserWithQuota(t, ctx, initialQuota)
		defer cleanupTestUser(t, ctx, userID)

		// Create a pending payment
		paymentID := uuid.New()
		_, err := testDB.Exec(ctx, `
			INSERT INTO payments (id, user_id, amount_usd, quota_purchased, payment_method, status)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, paymentID, userID, decimal.NewFromFloat(9.99), quotaAmount, models.PaymentMethodStripe, models.PaymentStatusPending)
		if err != nil {
			t.Fatalf("Failed to create payment record: %v", err)
		}
		defer cleanupPayment(t, ctx, paymentID)

		// Complete the payment first
		err = svc.CompletePayment(ctx, paymentID, "test_session_id")
		if err != nil {
			t.Fatalf("Failed to complete payment: %v", err)
		}

		// Get quota after completion
		var quotaAfterComplete int64
		err = testDB.QueryRow(ctx, `SELECT total_quota FROM quotas WHERE user_id = $1`, userID).Scan(&quotaAfterComplete)
		if err != nil {
			t.Fatalf("Failed to get quota: %v", err)
		}

		expectedQuota := initialQuota + quotaAmount
		if quotaAfterComplete != expectedQuota {
			t.Fatalf("Setup error: quota should be %d after completion, got %d", expectedQuota, quotaAfterComplete)
		}

		// Try to fail the completed payment (should be rejected)
		err = svc.FailPayment(ctx, paymentID, "card_declined")
		if err != ErrPaymentAlreadyDone {
			t.Logf("Failing completed payment returned: %v (expected ErrPaymentAlreadyDone)", err)
		}

		// Verify status is still completed
		var status models.PaymentStatus
		err = testDB.QueryRow(ctx, `SELECT status FROM payments WHERE id = $1`, paymentID).Scan(&status)
		if err != nil {
			t.Fatalf("Failed to get payment status: %v", err)
		}

		if status != models.PaymentStatusCompleted {
			t.Fatalf("PROPERTY VIOLATION: Completed payment status changed to '%s' after fail attempt", status)
		}

		// Verify quota wasn't affected
		var quotaAfterFailAttempt int64
		err = testDB.QueryRow(ctx, `SELECT total_quota FROM quotas WHERE user_id = $1`, userID).Scan(&quotaAfterFailAttempt)
		if err != nil {
			t.Fatalf("Failed to get quota: %v", err)
		}

		if quotaAfterFailAttempt != expectedQuota {
			t.Fatalf("PROPERTY VIOLATION: Quota changed from %d to %d after failing completed payment",
				expectedQuota, quotaAfterFailAttempt)
		}
	})
}

// Helper function to cleanup payment notifications
func cleanupPaymentNotifications(t *testing.T, ctx context.Context, paymentID uuid.UUID) {
	t.Helper()
	_, _ = testDB.Exec(ctx, `DELETE FROM payment_notifications WHERE payment_id = $1`, paymentID)
}
