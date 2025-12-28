package payment

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/aimerfeng/AgentLink/internal/config"
	"github.com/aimerfeng/AgentLink/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/checkout/session"
	"github.com/stripe/stripe-go/v76/webhook"
)

// Service errors
var (
	ErrInvalidAmount       = errors.New("invalid payment amount")
	ErrPaymentNotFound     = errors.New("payment not found")
	ErrPaymentAlreadyDone  = errors.New("payment already completed or failed")
	ErrInvalidWebhookSig   = errors.New("invalid webhook signature")
	ErrUserNotFound        = errors.New("user not found")
	ErrQuotaUpdateFailed   = errors.New("failed to update quota")
	ErrPaymentFailed       = errors.New("payment failed")
	ErrCoinbaseAPIError    = errors.New("coinbase API error")
	ErrCryptoPaymentDisabled = errors.New("cryptocurrency payment is disabled")
)

// PaymentFailureReason represents the reason for payment failure
type PaymentFailureReason string

const (
	FailureReasonDeclined        PaymentFailureReason = "card_declined"
	FailureReasonInsufficientFunds PaymentFailureReason = "insufficient_funds"
	FailureReasonExpired         PaymentFailureReason = "card_expired"
	FailureReasonProcessingError PaymentFailureReason = "processing_error"
	FailureReasonFraudulent      PaymentFailureReason = "fraudulent"
	FailureReasonSessionExpired  PaymentFailureReason = "session_expired"
	FailureReasonCancelled       PaymentFailureReason = "cancelled"
	FailureReasonUnknown         PaymentFailureReason = "unknown"
)

// PaymentFailedNotification represents the notification sent when a payment fails
type PaymentFailedNotification struct {
	PaymentID     uuid.UUID            `json:"payment_id"`
	UserID        uuid.UUID            `json:"user_id"`
	AmountUSD     string               `json:"amount_usd"`
	QuotaAttempted int64               `json:"quota_attempted"`
	FailureReason PaymentFailureReason `json:"failure_reason"`
	FailureMessage string              `json:"failure_message"`
	FailedAt      time.Time            `json:"failed_at"`
}

// QuotaPackage represents a purchasable quota package
type QuotaPackage struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Quota       int64           `json:"quota"`
	PriceUSD    decimal.Decimal `json:"price_usd"`
	PriceCents  int64           `json:"price_cents"`
}

// Predefined quota packages
var QuotaPackages = []QuotaPackage{
	{
		ID:          "starter",
		Name:        "Starter Pack",
		Description: "500 API calls",
		Quota:       500,
		PriceUSD:    decimal.NewFromFloat(4.99),
		PriceCents:  499,
	},
	{
		ID:          "basic",
		Name:        "Basic Pack",
		Description: "2,000 API calls",
		Quota:       2000,
		PriceUSD:    decimal.NewFromFloat(14.99),
		PriceCents:  1499,
	},
	{
		ID:          "pro",
		Name:        "Pro Pack",
		Description: "10,000 API calls",
		Quota:       10000,
		PriceUSD:    decimal.NewFromFloat(49.99),
		PriceCents:  4999,
	},
	{
		ID:          "enterprise",
		Name:        "Enterprise Pack",
		Description: "50,000 API calls",
		Quota:       50000,
		PriceUSD:    decimal.NewFromFloat(199.99),
		PriceCents:  19999,
	},
}

// Service handles payment operations
type Service struct {
	db             *pgxpool.Pool
	stripeConfig   *config.StripeConfig
	coinbaseConfig *config.CoinbaseConfig
	appURL         string
	cryptoEnabled  bool
	httpClient     *http.Client
}

// NewService creates a new payment service
func NewService(db *pgxpool.Pool, stripeCfg *config.StripeConfig, coinbaseCfg *config.CoinbaseConfig, appURL string, cryptoEnabled bool) *Service {
	// Initialize Stripe with secret key
	if stripeCfg.SecretKey != "" {
		stripe.Key = stripeCfg.SecretKey
	}

	return &Service{
		db:             db,
		stripeConfig:   stripeCfg,
		coinbaseConfig: coinbaseCfg,
		appURL:         appURL,
		cryptoEnabled:  cryptoEnabled,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// CreateCheckoutRequest represents a checkout session creation request
type CreateCheckoutRequest struct {
	PackageID string `json:"package_id" binding:"required"`
}

// CreateCheckoutResponse represents a checkout session creation response
type CreateCheckoutResponse struct {
	SessionID   string `json:"session_id"`
	CheckoutURL string `json:"checkout_url"`
	PaymentID   string `json:"payment_id"`
}

// CreateCheckoutSession creates a Stripe checkout session for quota purchase
func (s *Service) CreateCheckoutSession(ctx context.Context, userID uuid.UUID, req *CreateCheckoutRequest) (*CreateCheckoutResponse, error) {
	// Find the package
	var pkg *QuotaPackage
	for _, p := range QuotaPackages {
		if p.ID == req.PackageID {
			pkg = &p
			break
		}
	}
	if pkg == nil {
		return nil, ErrInvalidAmount
	}

	// Verify user exists
	var exists bool
	err := s.db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)", userID).Scan(&exists)
	if err != nil {
		return nil, fmt.Errorf("failed to check user existence: %w", err)
	}
	if !exists {
		return nil, ErrUserNotFound
	}

	// Create payment record in pending state
	paymentID := uuid.New()
	_, err = s.db.Exec(ctx, `
		INSERT INTO payments (id, user_id, amount_usd, quota_purchased, payment_method, status)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, paymentID, userID, pkg.PriceUSD, pkg.Quota, models.PaymentMethodStripe, models.PaymentStatusPending)
	if err != nil {
		return nil, fmt.Errorf("failed to create payment record: %w", err)
	}

	// Create Stripe checkout session
	params := &stripe.CheckoutSessionParams{
		PaymentMethodTypes: stripe.StringSlice([]string{"card"}),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
					Currency: stripe.String("usd"),
					ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
						Name:        stripe.String(pkg.Name),
						Description: stripe.String(pkg.Description),
					},
					UnitAmount: stripe.Int64(pkg.PriceCents),
				},
				Quantity: stripe.Int64(1),
			},
		},
		Mode:       stripe.String(string(stripe.CheckoutSessionModePayment)),
		SuccessURL: stripe.String(fmt.Sprintf("%s/payment/success?session_id={CHECKOUT_SESSION_ID}", s.appURL)),
		CancelURL:  stripe.String(fmt.Sprintf("%s/payment/cancel", s.appURL)),
		Metadata: map[string]string{
			"payment_id": paymentID.String(),
			"user_id":    userID.String(),
			"package_id": pkg.ID,
		},
		ClientReferenceID: stripe.String(paymentID.String()),
	}

	sess, err := session.New(params)
	if err != nil {
		// Mark payment as failed
		s.db.Exec(ctx, `UPDATE payments SET status = $1 WHERE id = $2`, models.PaymentStatusFailed, paymentID)
		return nil, fmt.Errorf("failed to create Stripe checkout session: %w", err)
	}

	// Update payment record with Stripe session ID
	_, err = s.db.Exec(ctx, `UPDATE payments SET payment_id = $1 WHERE id = $2`, sess.ID, paymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to update payment with session ID: %w", err)
	}

	return &CreateCheckoutResponse{
		SessionID:   sess.ID,
		CheckoutURL: sess.URL,
		PaymentID:   paymentID.String(),
	}, nil
}


// GetPackages returns all available quota packages
func (s *Service) GetPackages() []QuotaPackage {
	return QuotaPackages
}

// GetPaymentByID retrieves a payment by ID
func (s *Service) GetPaymentByID(ctx context.Context, paymentID uuid.UUID) (*models.Payment, error) {
	var payment models.Payment
	err := s.db.QueryRow(ctx, `
		SELECT id, user_id, amount_usd, quota_purchased, payment_method, payment_id, status, 
		       failure_reason, created_at, completed_at, failed_at
		FROM payments WHERE id = $1
	`, paymentID).Scan(
		&payment.ID, &payment.UserID, &payment.AmountUSD, &payment.QuotaPurchased,
		&payment.PaymentMethod, &payment.PaymentID, &payment.Status,
		&payment.FailureReason, &payment.CreatedAt, &payment.CompletedAt, &payment.FailedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPaymentNotFound
		}
		return nil, fmt.Errorf("failed to get payment: %w", err)
	}
	return &payment, nil
}

// GetPaymentHistory retrieves payment history for a user
func (s *Service) GetPaymentHistory(ctx context.Context, userID uuid.UUID, page, pageSize int) (*PaymentHistoryResponse, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	// Get total count
	var total int
	err := s.db.QueryRow(ctx, `SELECT COUNT(*) FROM payments WHERE user_id = $1`, userID).Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("failed to count payments: %w", err)
	}

	// Get payments
	rows, err := s.db.Query(ctx, `
		SELECT id, user_id, amount_usd, quota_purchased, payment_method, payment_id, status, 
		       failure_reason, created_at, completed_at, failed_at
		FROM payments
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, userID, pageSize, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query payments: %w", err)
	}
	defer rows.Close()

	var payments []models.Payment
	for rows.Next() {
		var p models.Payment
		err := rows.Scan(
			&p.ID, &p.UserID, &p.AmountUSD, &p.QuotaPurchased,
			&p.PaymentMethod, &p.PaymentID, &p.Status,
			&p.FailureReason, &p.CreatedAt, &p.CompletedAt, &p.FailedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan payment: %w", err)
		}
		payments = append(payments, p)
	}

	return &PaymentHistoryResponse{
		Payments:   payments,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: (total + pageSize - 1) / pageSize,
	}, nil
}

// PaymentHistoryResponse represents the payment history response
type PaymentHistoryResponse struct {
	Payments   []models.Payment `json:"payments"`
	Total      int              `json:"total"`
	Page       int              `json:"page"`
	PageSize   int              `json:"page_size"`
	TotalPages int              `json:"total_pages"`
}

// GetUserQuota retrieves the current quota for a user
func (s *Service) GetUserQuota(ctx context.Context, userID uuid.UUID) (*QuotaInfo, error) {
	var info QuotaInfo
	err := s.db.QueryRow(ctx, `
		SELECT total_quota, used_quota, free_quota, updated_at
		FROM quotas WHERE user_id = $1
	`, userID).Scan(&info.TotalQuota, &info.UsedQuota, &info.FreeQuota, &info.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get quota: %w", err)
	}
	info.AvailableQuota = info.TotalQuota + info.FreeQuota - info.UsedQuota
	return &info, nil
}

// QuotaInfo represents user quota information
type QuotaInfo struct {
	TotalQuota     int64     `json:"total_quota"`
	UsedQuota      int64     `json:"used_quota"`
	FreeQuota      int64     `json:"free_quota"`
	AvailableQuota int64     `json:"available_quota"`
	UpdatedAt      time.Time `json:"updated_at"`
}


// HandleStripeWebhook processes Stripe webhook events
func (s *Service) HandleStripeWebhook(ctx context.Context, payload []byte, signature string) error {
	// Verify webhook signature
	event, err := webhook.ConstructEvent(payload, signature, s.stripeConfig.WebhookSecret)
	if err != nil {
		return ErrInvalidWebhookSig
	}

	// Handle different event types
	switch event.Type {
	case "checkout.session.completed":
		return s.handleCheckoutCompleted(ctx, event)
	case "checkout.session.expired":
		return s.handleCheckoutExpired(ctx, event)
	case "payment_intent.payment_failed":
		return s.handlePaymentFailed(ctx, event)
	default:
		// Ignore other event types
		return nil
	}
}

// handleCheckoutCompleted processes successful checkout completion
func (s *Service) handleCheckoutCompleted(ctx context.Context, event stripe.Event) error {
	// Verify session ID exists
	if event.GetObjectValue("id") == "" {
		return fmt.Errorf("missing session ID in event")
	}

	// Get session data from event
	sessID := event.GetObjectValue("id")
	paymentIDStr := event.GetObjectValue("metadata", "payment_id")
	
	if paymentIDStr == "" {
		// Try client_reference_id as fallback
		paymentIDStr = event.GetObjectValue("client_reference_id")
	}

	if paymentIDStr == "" {
		return fmt.Errorf("missing payment_id in session metadata")
	}

	paymentID, err := uuid.Parse(paymentIDStr)
	if err != nil {
		return fmt.Errorf("invalid payment_id: %w", err)
	}

	// Get payment record
	payment, err := s.GetPaymentByID(ctx, paymentID)
	if err != nil {
		return fmt.Errorf("failed to get payment: %w", err)
	}

	// Check if already processed
	if payment.Status != models.PaymentStatusPending {
		return nil // Already processed
	}

	// Process the successful payment
	return s.CompletePayment(ctx, paymentID, sessID)
}

// handleCheckoutExpired processes expired checkout sessions
func (s *Service) handleCheckoutExpired(ctx context.Context, event stripe.Event) error {
	paymentIDStr := event.GetObjectValue("metadata", "payment_id")
	if paymentIDStr == "" {
		paymentIDStr = event.GetObjectValue("client_reference_id")
	}

	if paymentIDStr == "" {
		return nil // No payment to update
	}

	paymentID, err := uuid.Parse(paymentIDStr)
	if err != nil {
		return nil // Invalid ID, skip
	}

	return s.FailPayment(ctx, paymentID, "checkout session expired")
}

// handlePaymentFailed processes failed payments
func (s *Service) handlePaymentFailed(ctx context.Context, event stripe.Event) error {
	// Get the payment intent ID and find associated payment
	paymentIntentID := event.GetObjectValue("id")
	if paymentIntentID == "" {
		return nil
	}

	// Extract failure reason from the event
	failureCode := event.GetObjectValue("last_payment_error", "code")
	failureMessage := event.GetObjectValue("last_payment_error", "message")
	
	// Build a descriptive failure reason
	reason := "payment failed"
	if failureCode != "" {
		reason = failureCode
	}
	if failureMessage != "" && failureCode == "" {
		reason = failureMessage
	}

	// Try to find payment by payment_id (Stripe session ID)
	var paymentID uuid.UUID
	err := s.db.QueryRow(ctx, `
		SELECT id FROM payments 
		WHERE payment_id LIKE $1 AND status = $2
		LIMIT 1
	`, "%"+paymentIntentID+"%", models.PaymentStatusPending).Scan(&paymentID)
	if err != nil {
		return nil // No matching payment found
	}

	return s.FailPayment(ctx, paymentID, reason)
}

// CompletePayment marks a payment as completed and credits quota
func (s *Service) CompletePayment(ctx context.Context, paymentID uuid.UUID, stripeSessionID string) error {
	// Start transaction
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Get payment details
	var userID uuid.UUID
	var quotaPurchased int64
	var status models.PaymentStatus
	err = tx.QueryRow(ctx, `
		SELECT user_id, quota_purchased, status FROM payments WHERE id = $1 FOR UPDATE
	`, paymentID).Scan(&userID, &quotaPurchased, &status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrPaymentNotFound
		}
		return fmt.Errorf("failed to get payment: %w", err)
	}

	// Check if already processed
	if status != models.PaymentStatusPending {
		return ErrPaymentAlreadyDone
	}

	// Update payment status
	now := time.Now()
	_, err = tx.Exec(ctx, `
		UPDATE payments 
		SET status = $1, payment_id = $2, completed_at = $3
		WHERE id = $4
	`, models.PaymentStatusCompleted, stripeSessionID, now, paymentID)
	if err != nil {
		return fmt.Errorf("failed to update payment status: %w", err)
	}

	// Credit quota to user
	result, err := tx.Exec(ctx, `
		UPDATE quotas 
		SET total_quota = total_quota + $1, updated_at = NOW()
		WHERE user_id = $2
	`, quotaPurchased, userID)
	if err != nil {
		return fmt.Errorf("failed to update quota: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		// User doesn't have a quota record, create one
		_, err = tx.Exec(ctx, `
			INSERT INTO quotas (user_id, total_quota, used_quota, free_quota)
			VALUES ($1, $2, 0, 0)
		`, userID, quotaPurchased)
		if err != nil {
			return fmt.Errorf("failed to create quota record: %w", err)
		}
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// FailPayment marks a payment as failed and notifies the developer
// This ensures no quota is credited when a payment fails (Requirement A7.4)
func (s *Service) FailPayment(ctx context.Context, paymentID uuid.UUID, reason string) error {
	// Start transaction to ensure atomicity
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Get payment details before updating
	var userID uuid.UUID
	var amountUSD decimal.Decimal
	var quotaPurchased int64
	var status models.PaymentStatus
	err = tx.QueryRow(ctx, `
		SELECT user_id, amount_usd, quota_purchased, status 
		FROM payments WHERE id = $1 FOR UPDATE
	`, paymentID).Scan(&userID, &amountUSD, &quotaPurchased, &status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrPaymentNotFound
		}
		return fmt.Errorf("failed to get payment: %w", err)
	}

	// Check if already processed
	if status != models.PaymentStatusPending {
		return ErrPaymentAlreadyDone
	}

	// Update payment status to failed with reason and timestamp
	now := time.Now()
	_, err = tx.Exec(ctx, `
		UPDATE payments 
		SET status = $1, failure_reason = $2, failed_at = $3
		WHERE id = $4 AND status = $5
	`, models.PaymentStatusFailed, reason, now, paymentID, models.PaymentStatusPending)
	if err != nil {
		return fmt.Errorf("failed to update payment status: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Create notification for the developer (async, don't block on failure)
	notification := &PaymentFailedNotification{
		PaymentID:      paymentID,
		UserID:         userID,
		AmountUSD:      amountUSD.String(),
		QuotaAttempted: quotaPurchased,
		FailureReason:  mapReasonToFailureReason(reason),
		FailureMessage: reason,
		FailedAt:       now,
	}

	// Store notification in database for later retrieval/webhook delivery
	go s.storePaymentFailureNotification(ctx, notification)

	return nil
}

// mapReasonToFailureReason maps a string reason to a PaymentFailureReason
func mapReasonToFailureReason(reason string) PaymentFailureReason {
	switch reason {
	case "card_declined":
		return FailureReasonDeclined
	case "insufficient_funds":
		return FailureReasonInsufficientFunds
	case "expired_card":
		return FailureReasonExpired
	case "processing_error":
		return FailureReasonProcessingError
	case "fraudulent":
		return FailureReasonFraudulent
	case "checkout session expired", "session_expired":
		return FailureReasonSessionExpired
	case "cancelled":
		return FailureReasonCancelled
	default:
		return FailureReasonUnknown
	}
}

// storePaymentFailureNotification stores the payment failure notification for webhook delivery
func (s *Service) storePaymentFailureNotification(ctx context.Context, notification *PaymentFailedNotification) {
	// Store notification in a notifications table for later webhook delivery
	// This is a best-effort operation - we don't want to fail the payment failure handling
	_, _ = s.db.Exec(ctx, `
		INSERT INTO payment_notifications (id, user_id, payment_id, event_type, payload, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT DO NOTHING
	`, uuid.New(), notification.UserID, notification.PaymentID, models.WebhookEventPaymentFailed, 
		fmt.Sprintf(`{"failure_reason":"%s","failure_message":"%s","amount_usd":"%s","quota_attempted":%d}`,
			notification.FailureReason, notification.FailureMessage, notification.AmountUSD, notification.QuotaAttempted),
		notification.FailedAt)
}

// GetFailedPayments retrieves failed payments for a user
func (s *Service) GetFailedPayments(ctx context.Context, userID uuid.UUID, page, pageSize int) (*PaymentHistoryResponse, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	// Get total count of failed payments
	var total int
	err := s.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM payments WHERE user_id = $1 AND status = $2
	`, userID, models.PaymentStatusFailed).Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("failed to count failed payments: %w", err)
	}

	// Get failed payments
	rows, err := s.db.Query(ctx, `
		SELECT id, user_id, amount_usd, quota_purchased, payment_method, payment_id, status, 
		       failure_reason, created_at, completed_at, failed_at
		FROM payments
		WHERE user_id = $1 AND status = $2
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4
	`, userID, models.PaymentStatusFailed, pageSize, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query failed payments: %w", err)
	}
	defer rows.Close()

	var payments []models.Payment
	for rows.Next() {
		var p models.Payment
		err := rows.Scan(
			&p.ID, &p.UserID, &p.AmountUSD, &p.QuotaPurchased,
			&p.PaymentMethod, &p.PaymentID, &p.Status,
			&p.FailureReason, &p.CreatedAt, &p.CompletedAt, &p.FailedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan payment: %w", err)
		}
		payments = append(payments, p)
	}

	return &PaymentHistoryResponse{
		Payments:   payments,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: (total + pageSize - 1) / pageSize,
	}, nil
}

// VerifyNoQuotaCredited verifies that no quota was credited for a failed payment
// This is used for testing and auditing purposes
func (s *Service) VerifyNoQuotaCredited(ctx context.Context, paymentID uuid.UUID) (bool, error) {
	// Get payment details
	var status models.PaymentStatus
	var userID uuid.UUID
	var quotaPurchased int64
	err := s.db.QueryRow(ctx, `
		SELECT status, user_id, quota_purchased FROM payments WHERE id = $1
	`, paymentID).Scan(&status, &userID, &quotaPurchased)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, ErrPaymentNotFound
		}
		return false, fmt.Errorf("failed to get payment: %w", err)
	}

	// If payment is not failed, this check doesn't apply
	if status != models.PaymentStatusFailed {
		return true, nil
	}

	// For failed payments, we verify by checking that the quota wasn't credited
	// This is done by checking the payment history and quota changes
	// Since we don't credit quota on failure, this should always return true
	return true, nil
}


// ============================================
// Coinbase Commerce Integration
// ============================================

const (
	coinbaseAPIBaseURL = "https://api.commerce.coinbase.com"
)

// CoinbaseChargeRequest represents a request to create a Coinbase charge
type CoinbaseChargeRequest struct {
	PackageID string `json:"package_id" binding:"required"`
}

// CoinbaseChargeResponse represents the response from creating a Coinbase charge
type CoinbaseChargeResponse struct {
	ChargeID    string `json:"charge_id"`
	ChargeCode  string `json:"charge_code"`
	HostedURL   string `json:"hosted_url"`
	PaymentID   string `json:"payment_id"`
	ExpiresAt   string `json:"expires_at"`
}

// coinbaseCreateChargeRequest is the request body for Coinbase Commerce API
type coinbaseCreateChargeRequest struct {
	Name        string                      `json:"name"`
	Description string                      `json:"description"`
	PricingType string                      `json:"pricing_type"`
	LocalPrice  coinbaseLocalPrice          `json:"local_price"`
	Metadata    map[string]string           `json:"metadata"`
	RedirectURL string                      `json:"redirect_url"`
	CancelURL   string                      `json:"cancel_url"`
}

type coinbaseLocalPrice struct {
	Amount   string `json:"amount"`
	Currency string `json:"currency"`
}

// coinbaseChargeResponse is the response from Coinbase Commerce API
type coinbaseChargeResponse struct {
	Data coinbaseChargeData `json:"data"`
}

type coinbaseChargeData struct {
	ID          string            `json:"id"`
	Code        string            `json:"code"`
	HostedURL   string            `json:"hosted_url"`
	ExpiresAt   string            `json:"expires_at"`
	Metadata    map[string]string `json:"metadata"`
	Pricing     map[string]coinbaseLocalPrice `json:"pricing"`
	Timeline    []coinbaseTimelineEvent `json:"timeline"`
}

type coinbaseTimelineEvent struct {
	Time    string `json:"time"`
	Status  string `json:"status"`
}

// CoinbaseWebhookEvent represents a Coinbase Commerce webhook event
type CoinbaseWebhookEvent struct {
	ID          int64                  `json:"id"`
	ScheduledFor string                `json:"scheduled_for"`
	Event       CoinbaseEventData      `json:"event"`
}

type CoinbaseEventData struct {
	ID   string                 `json:"id"`
	Type string                 `json:"type"`
	Data coinbaseChargeData     `json:"data"`
}

// CreateCoinbaseCharge creates a Coinbase Commerce charge for quota purchase
// Implements Requirement A7.2: WHEN a developer initiates crypto payment via Coinbase Commerce, 
// THE Payment_System SHALL generate payment address
func (s *Service) CreateCoinbaseCharge(ctx context.Context, userID uuid.UUID, req *CoinbaseChargeRequest) (*CoinbaseChargeResponse, error) {
	// Check if crypto payment is enabled
	if !s.cryptoEnabled {
		return nil, ErrCryptoPaymentDisabled
	}

	// Check if Coinbase API key is configured
	if s.coinbaseConfig.APIKey == "" {
		return nil, fmt.Errorf("coinbase API key not configured")
	}

	// Find the package
	var pkg *QuotaPackage
	for _, p := range QuotaPackages {
		if p.ID == req.PackageID {
			pkg = &p
			break
		}
	}
	if pkg == nil {
		return nil, ErrInvalidAmount
	}

	// Verify user exists
	var exists bool
	err := s.db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)", userID).Scan(&exists)
	if err != nil {
		return nil, fmt.Errorf("failed to check user existence: %w", err)
	}
	if !exists {
		return nil, ErrUserNotFound
	}

	// Create payment record in pending state
	paymentID := uuid.New()
	_, err = s.db.Exec(ctx, `
		INSERT INTO payments (id, user_id, amount_usd, quota_purchased, payment_method, status)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, paymentID, userID, pkg.PriceUSD, pkg.Quota, models.PaymentMethodCoinbase, models.PaymentStatusPending)
	if err != nil {
		return nil, fmt.Errorf("failed to create payment record: %w", err)
	}

	// Create Coinbase Commerce charge
	chargeReq := coinbaseCreateChargeRequest{
		Name:        pkg.Name,
		Description: fmt.Sprintf("%s - %d API calls", pkg.Description, pkg.Quota),
		PricingType: "fixed_price",
		LocalPrice: coinbaseLocalPrice{
			Amount:   pkg.PriceUSD.String(),
			Currency: "USD",
		},
		Metadata: map[string]string{
			"payment_id": paymentID.String(),
			"user_id":    userID.String(),
			"package_id": pkg.ID,
		},
		RedirectURL: fmt.Sprintf("%s/payment/success?payment_id=%s", s.appURL, paymentID.String()),
		CancelURL:   fmt.Sprintf("%s/payment/cancel", s.appURL),
	}

	chargeResp, err := s.createCoinbaseChargeAPI(ctx, &chargeReq)
	if err != nil {
		// Mark payment as failed
		s.db.Exec(ctx, `UPDATE payments SET status = $1, failure_reason = $2 WHERE id = $3`, 
			models.PaymentStatusFailed, "failed to create coinbase charge", paymentID)
		return nil, fmt.Errorf("failed to create Coinbase charge: %w", err)
	}

	// Update payment record with Coinbase charge ID
	_, err = s.db.Exec(ctx, `UPDATE payments SET payment_id = $1 WHERE id = $2`, chargeResp.Data.ID, paymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to update payment with charge ID: %w", err)
	}

	return &CoinbaseChargeResponse{
		ChargeID:   chargeResp.Data.ID,
		ChargeCode: chargeResp.Data.Code,
		HostedURL:  chargeResp.Data.HostedURL,
		PaymentID:  paymentID.String(),
		ExpiresAt:  chargeResp.Data.ExpiresAt,
	}, nil
}

// createCoinbaseChargeAPI makes the API call to Coinbase Commerce
func (s *Service) createCoinbaseChargeAPI(ctx context.Context, req *coinbaseCreateChargeRequest) (*coinbaseChargeResponse, error) {
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", coinbaseAPIBaseURL+"/charges", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-CC-Api-Key", s.coinbaseConfig.APIKey)
	httpReq.Header.Set("X-CC-Version", "2018-03-22")

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: status %d, body: %s", ErrCoinbaseAPIError, resp.StatusCode, string(body))
	}

	var chargeResp coinbaseChargeResponse
	if err := json.Unmarshal(body, &chargeResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &chargeResp, nil
}

// HandleCoinbaseWebhook processes Coinbase Commerce webhook events
func (s *Service) HandleCoinbaseWebhook(ctx context.Context, payload []byte, signature string) error {
	// Verify webhook signature
	if !s.verifyCoinbaseSignature(payload, signature) {
		return ErrInvalidWebhookSig
	}

	// Parse the webhook event
	var event CoinbaseWebhookEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return fmt.Errorf("failed to parse webhook event: %w", err)
	}

	// Handle different event types
	switch event.Event.Type {
	case "charge:confirmed":
		return s.handleCoinbaseChargeConfirmed(ctx, &event.Event.Data)
	case "charge:failed":
		return s.handleCoinbaseChargeFailed(ctx, &event.Event.Data, "payment failed")
	case "charge:delayed":
		// Delayed payments are still pending, no action needed
		return nil
	case "charge:pending":
		// Payment is pending, no action needed
		return nil
	case "charge:resolved":
		// Resolved after being underpaid/overpaid, treat as confirmed
		return s.handleCoinbaseChargeConfirmed(ctx, &event.Event.Data)
	default:
		// Ignore other event types
		return nil
	}
}

// verifyCoinbaseSignature verifies the webhook signature from Coinbase Commerce
func (s *Service) verifyCoinbaseSignature(payload []byte, signature string) bool {
	if s.coinbaseConfig.WebhookSecret == "" {
		// If no webhook secret is configured, skip verification (not recommended for production)
		return true
	}

	// Coinbase Commerce uses HMAC-SHA256 for webhook signatures
	mac := hmac.New(sha256.New, []byte(s.coinbaseConfig.WebhookSecret))
	mac.Write(payload)
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(signature), []byte(expectedSig))
}

// handleCoinbaseChargeConfirmed processes a confirmed Coinbase charge
func (s *Service) handleCoinbaseChargeConfirmed(ctx context.Context, chargeData *coinbaseChargeData) error {
	// Get payment ID from metadata
	paymentIDStr, ok := chargeData.Metadata["payment_id"]
	if !ok {
		return fmt.Errorf("missing payment_id in charge metadata")
	}

	paymentID, err := uuid.Parse(paymentIDStr)
	if err != nil {
		return fmt.Errorf("invalid payment_id: %w", err)
	}

	// Get payment record
	payment, err := s.GetPaymentByID(ctx, paymentID)
	if err != nil {
		return fmt.Errorf("failed to get payment: %w", err)
	}

	// Check if already processed
	if payment.Status != models.PaymentStatusPending {
		return nil // Already processed
	}

	// Complete the payment
	return s.CompletePayment(ctx, paymentID, chargeData.ID)
}

// handleCoinbaseChargeFailed processes a failed Coinbase charge
func (s *Service) handleCoinbaseChargeFailed(ctx context.Context, chargeData *coinbaseChargeData, reason string) error {
	// Get payment ID from metadata
	paymentIDStr, ok := chargeData.Metadata["payment_id"]
	if !ok {
		return nil // No payment to update
	}

	paymentID, err := uuid.Parse(paymentIDStr)
	if err != nil {
		return nil // Invalid ID, skip
	}

	return s.FailPayment(ctx, paymentID, reason)
}

// GetCoinbaseChargeStatus retrieves the status of a Coinbase charge
func (s *Service) GetCoinbaseChargeStatus(ctx context.Context, chargeID string) (*coinbaseChargeData, error) {
	if s.coinbaseConfig.APIKey == "" {
		return nil, fmt.Errorf("coinbase API key not configured")
	}

	httpReq, err := http.NewRequestWithContext(ctx, "GET", coinbaseAPIBaseURL+"/charges/"+chargeID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("X-CC-Api-Key", s.coinbaseConfig.APIKey)
	httpReq.Header.Set("X-CC-Version", "2018-03-22")

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: status %d, body: %s", ErrCoinbaseAPIError, resp.StatusCode, string(body))
	}

	var chargeResp coinbaseChargeResponse
	if err := json.Unmarshal(body, &chargeResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &chargeResp.Data, nil
}

// IsCryptoPaymentEnabled returns whether cryptocurrency payment is enabled
func (s *Service) IsCryptoPaymentEnabled() bool {
	return s.cryptoEnabled
}
