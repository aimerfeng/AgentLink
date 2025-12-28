package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/aimerfeng/AgentLink/internal/agent"
	"github.com/aimerfeng/AgentLink/internal/apikey"
	"github.com/aimerfeng/AgentLink/internal/cache"
	"github.com/aimerfeng/AgentLink/internal/config"
	"github.com/aimerfeng/AgentLink/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
	"github.com/shopspring/decimal"
)

// Service errors
var (
	ErrAgentNotFound      = errors.New("agent not found")
	ErrAgentNotActive     = errors.New("agent is not active")
	ErrInvalidAPIKey      = errors.New("invalid or revoked API key")
	ErrQuotaExhausted     = errors.New("API quota exhausted")
	ErrRateLimited        = errors.New("rate limit exceeded")
	ErrUpstreamTimeout    = errors.New("upstream service timeout")
	ErrUpstreamError      = errors.New("upstream service error")
	ErrInvalidRequest     = errors.New("invalid request")
)

// Service handles proxy gateway operations
type Service struct {
	db                    *pgxpool.Pool
	redis                 *cache.Redis
	agentService          *agent.Service
	apiKeyService         *apikey.Service
	config                *config.Config
	httpClient            *http.Client
	quotaManager          *QuotaManager
	promptInjector        *PromptInjector
	streamHandler         *StreamHandler
	rateLimiter           *RateLimiter
	circuitBreakerManager *CircuitBreakerManager
	timeoutManager        *TimeoutManager
}

// NewService creates a new proxy service
func NewService(
	db *pgxpool.Pool,
	redis *cache.Redis,
	agentSvc *agent.Service,
	apiKeySvc *apikey.Service,
	cfg *config.Config,
) *Service {
	promptInjector := NewPromptInjector()
	timeoutCfg := &TimeoutConfig{
		DefaultTimeout: time.Duration(cfg.Proxy.DefaultTimeout) * time.Second,
		MaxTimeout:     120 * time.Second,
		MinTimeout:     5 * time.Second,
	}
	svc := &Service{
		db:             db,
		redis:          redis,
		agentService:   agentSvc,
		apiKeyService:  apiKeySvc,
		config:         cfg,
		httpClient: &http.Client{
			Timeout: time.Duration(cfg.Proxy.DefaultTimeout) * time.Second,
		},
		promptInjector:        promptInjector,
		streamHandler:         NewStreamHandler(promptInjector),
		rateLimiter:           NewRateLimiter(redis, &cfg.RateLimit),
		circuitBreakerManager: NewCircuitBreakerManager(DefaultCircuitBreakerConfig()),
		timeoutManager:        NewTimeoutManager(timeoutCfg),
	}
	svc.quotaManager = NewQuotaManager(svc)
	return svc
}

// GetQuotaManager returns the quota manager
func (s *Service) GetQuotaManager() *QuotaManager {
	return s.quotaManager
}

// GetPromptInjector returns the prompt injector
func (s *Service) GetPromptInjector() *PromptInjector {
	return s.promptInjector
}

// GetStreamHandler returns the stream handler
func (s *Service) GetStreamHandler() *StreamHandler {
	return s.streamHandler
}

// GetRateLimiter returns the rate limiter
func (s *Service) GetRateLimiter() *RateLimiter {
	return s.rateLimiter
}

// GetCircuitBreakerManager returns the circuit breaker manager
func (s *Service) GetCircuitBreakerManager() *CircuitBreakerManager {
	return s.circuitBreakerManager
}

// GetTimeoutManager returns the timeout manager
func (s *Service) GetTimeoutManager() *TimeoutManager {
	return s.timeoutManager
}


// ChatRequest represents a chat request to the proxy
type ChatRequest struct {
	Messages []ChatMessage `json:"messages" binding:"required"`
	Stream   bool          `json:"stream"`
}

// ChatMessage represents a single message in the chat
type ChatMessage struct {
	Role    string `json:"role" binding:"required"`
	Content string `json:"content" binding:"required"`
}

// ChatResponse represents a non-streaming chat response
type ChatResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []ChatChoice   `json:"choices"`
	Usage   *ChatUsage     `json:"usage,omitempty"`
}

// ChatChoice represents a choice in the response
type ChatChoice struct {
	Index        int          `json:"index"`
	Message      *ChatMessage `json:"message,omitempty"`
	Delta        *ChatMessage `json:"delta,omitempty"`
	FinishReason *string      `json:"finish_reason,omitempty"`
}

// ChatUsage represents token usage information
type ChatUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// StreamChunk represents a streaming response chunk
type StreamChunk struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Created int64        `json:"created"`
	Model   string       `json:"model"`
	Choices []ChatChoice `json:"choices"`
}

// CallContext holds context for a single API call
type CallContext struct {
	RequestID     string
	CorrelationID string
	AgentID       uuid.UUID
	UserID        uuid.UUID
	APIKeyID      uuid.UUID
	Agent         *models.Agent
	AgentConfig   *models.AgentConfig
	StartTime     time.Time
	IsPaidUser    bool
}

// CallResult holds the result of an API call
type CallResult struct {
	Success      bool
	InputTokens  int
	OutputTokens int
	LatencyMs    int
	ErrorCode    string
	Cost         decimal.Decimal
}


// ValidateAPIKey validates an API key and returns the associated user info
func (s *Service) ValidateAPIKey(ctx context.Context, rawKey string) (*models.APIKey, error) {
	apiKey, err := s.apiKeyService.ValidateAPIKey(ctx, rawKey)
	if err != nil {
		if errors.Is(err, apikey.ErrInvalidAPIKey) || errors.Is(err, apikey.ErrAPIKeyRevoked) {
			return nil, ErrInvalidAPIKey
		}
		return nil, fmt.Errorf("failed to validate API key: %w", err)
	}
	return apiKey, nil
}

// GetAgent retrieves and validates an agent for API calls
func (s *Service) GetAgent(ctx context.Context, agentID uuid.UUID) (*models.Agent, *models.AgentConfig, error) {
	// Get agent from database
	agentModel, err := s.agentService.GetByID(ctx, agentID)
	if err != nil {
		if errors.Is(err, agent.ErrAgentNotFound) {
			return nil, nil, ErrAgentNotFound
		}
		return nil, nil, fmt.Errorf("failed to get agent: %w", err)
	}

	// Check if agent is active
	if agentModel.Status != models.AgentStatusActive {
		return nil, nil, ErrAgentNotActive
	}

	// Get decrypted config
	agentConfig, err := s.agentService.GetConfig(ctx, agentID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get agent config: %w", err)
	}

	return agentModel, agentConfig, nil
}

// CheckQuota checks if user has sufficient quota
func (s *Service) CheckQuota(ctx context.Context, userID uuid.UUID) (int64, error) {
	// First check Redis cache
	remaining, err := s.redis.GetQuota(ctx, userID.String())
	if err == nil && remaining > 0 {
		return remaining, nil
	}

	// Fall back to database
	var quota models.Quota
	err = s.db.QueryRow(ctx, `
		SELECT user_id, total_quota, used_quota, free_quota, updated_at
		FROM quotas WHERE user_id = $1
	`, userID).Scan(&quota.UserID, &quota.TotalQuota, &quota.UsedQuota, &quota.FreeQuota, &quota.UpdatedAt)
	if err != nil {
		return 0, fmt.Errorf("failed to get quota: %w", err)
	}

	remaining = quota.RemainingQuota()
	
	// Cache the quota in Redis
	if remaining > 0 {
		_ = s.redis.SetQuota(ctx, userID.String(), remaining)
	}

	return remaining, nil
}

// CheckRateLimit checks if user is within rate limits
// Returns allowed, remaining, and error
func (s *Service) CheckRateLimit(ctx context.Context, userID uuid.UUID, isPaidUser bool) (bool, int64, error) {
	result, err := s.rateLimiter.Check(ctx, userID.String(), isPaidUser)
	if err != nil {
		return false, 0, err
	}
	return result.Allowed, result.Remaining, nil
}

// CheckRateLimitWithResult checks rate limit and returns detailed result
func (s *Service) CheckRateLimitWithResult(ctx context.Context, userID uuid.UUID, isPaidUser bool) (*RateLimitResult, error) {
	return s.rateLimiter.Check(ctx, userID.String(), isPaidUser)
}

// IsPaidUser checks if a user is a paid user (has purchased quota)
func (s *Service) IsPaidUser(ctx context.Context, userID uuid.UUID) (bool, error) {
	var totalQuota int64
	err := s.db.QueryRow(ctx, `
		SELECT total_quota FROM quotas WHERE user_id = $1
	`, userID).Scan(&totalQuota)
	if err != nil {
		return false, fmt.Errorf("failed to check paid status: %w", err)
	}
	// User is paid if they have purchased quota beyond free quota
	return totalQuota > 0, nil
}


// DecrementQuota decrements the user's quota atomically
// Returns the new remaining quota
func (s *Service) DecrementQuota(ctx context.Context, userID uuid.UUID, amount int64) (int64, error) {
	// Use Redis for atomic decrement
	remaining, err := s.redis.DecrementQuota(ctx, userID.String(), amount)
	if err != nil {
		// Fall back to database
		return s.decrementQuotaDB(ctx, userID, amount)
	}

	// If Redis shows negative, sync with database
	if remaining < 0 {
		return s.decrementQuotaDB(ctx, userID, amount)
	}

	// Async update database
	go func() {
		_, _ = s.decrementQuotaDB(context.Background(), userID, amount)
	}()

	return remaining, nil
}

// decrementQuotaDB decrements quota in the database
func (s *Service) decrementQuotaDB(ctx context.Context, userID uuid.UUID, amount int64) (int64, error) {
	var remaining int64
	err := s.db.QueryRow(ctx, `
		UPDATE quotas 
		SET used_quota = used_quota + $1, updated_at = NOW()
		WHERE user_id = $2
		RETURNING (total_quota + free_quota - used_quota)
	`, amount, userID).Scan(&remaining)
	if err != nil {
		return 0, fmt.Errorf("failed to decrement quota: %w", err)
	}

	// Update Redis cache
	_ = s.redis.SetQuota(ctx, userID.String(), remaining)

	return remaining, nil
}

// RefundQuota refunds quota when a call fails
// This ensures failed calls don't cost quota (Requirement A6.5)
func (s *Service) RefundQuota(ctx context.Context, userID uuid.UUID, amount int64) error {
	if amount <= 0 {
		return nil // Nothing to refund
	}

	// Increment in Redis first for immediate effect
	_, err := s.redis.Client.IncrBy(ctx, fmt.Sprintf("quota:%s", userID.String()), amount).Result()
	if err != nil {
		log.Warn().Err(err).Str("user_id", userID.String()).Int64("amount", amount).Msg("Failed to refund quota in Redis")
	}

	// Update database for persistence
	result, err := s.db.Exec(ctx, `
		UPDATE quotas 
		SET used_quota = GREATEST(0, used_quota - $1), updated_at = NOW()
		WHERE user_id = $2
	`, amount, userID)
	if err != nil {
		return fmt.Errorf("failed to refund quota: %w", err)
	}

	// Log if no rows were affected (user might not exist)
	if result.RowsAffected() == 0 {
		log.Warn().Str("user_id", userID.String()).Int64("amount", amount).Msg("No quota record found to refund")
	}

	return nil
}

// LogCall logs an API call to the database
func (s *Service) LogCall(ctx context.Context, callCtx *CallContext, result *CallResult) error {
	status := models.CallStatusSuccess
	if !result.Success {
		status = models.CallStatusError
	}

	var errorCode *string
	if result.ErrorCode != "" {
		errorCode = &result.ErrorCode
	}

	// Use correlation_id for trace_id if available, otherwise use request_id
	traceID := callCtx.CorrelationID
	if traceID == "" {
		traceID = callCtx.RequestID
	}

	_, err := s.db.Exec(ctx, `
		INSERT INTO call_logs (
			agent_id, api_key_id, user_id, request_id, trace_id,
			input_tokens, output_tokens, latency_ms, status, error_code, cost_usd
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, callCtx.AgentID, callCtx.APIKeyID, callCtx.UserID, callCtx.RequestID, traceID,
		result.InputTokens, result.OutputTokens, result.LatencyMs, status, errorCode, result.Cost)
	if err != nil {
		return fmt.Errorf("failed to log call: %w", err)
	}

	// Update agent statistics
	if result.Success {
		_, err = s.db.Exec(ctx, `
			UPDATE agents 
			SET total_calls = total_calls + 1, 
			    total_revenue = total_revenue + $1,
			    updated_at = NOW()
			WHERE id = $2
		`, result.Cost, callCtx.AgentID)
		if err != nil {
			log.Warn().Err(err).Str("agent_id", callCtx.AgentID.String()).Msg("Failed to update agent stats")
		}
	}

	return nil
}


// InjectSystemPrompt injects the agent's system prompt into the messages
// The system prompt is prepended and never exposed in responses
func (s *Service) InjectSystemPrompt(messages []ChatMessage, systemPrompt string) []ChatMessage {
	return s.promptInjector.InjectSystemPrompt(messages, systemPrompt)
}

// BuildUpstreamRequest builds the request to send to the AI provider
func (s *Service) BuildUpstreamRequest(agentConfig *models.AgentConfig, messages []ChatMessage, stream bool) (map[string]interface{}, error) {
	// Inject system prompt
	messagesWithPrompt := s.InjectSystemPrompt(messages, agentConfig.SystemPrompt)
	
	// Convert messages to the format expected by the provider
	formattedMessages := make([]map[string]string, len(messagesWithPrompt))
	for i, msg := range messagesWithPrompt {
		formattedMessages[i] = map[string]string{
			"role":    msg.Role,
			"content": msg.Content,
		}
	}
	
	request := map[string]interface{}{
		"model":       agentConfig.Model,
		"messages":    formattedMessages,
		"temperature": agentConfig.Temperature,
		"max_tokens":  agentConfig.MaxTokens,
		"top_p":       agentConfig.TopP,
		"stream":      stream,
	}
	
	return request, nil
}

// getProviderURL returns the API URL for the given provider
func (s *Service) getProviderURL(provider string) string {
	switch provider {
	case "openai":
		return "https://api.openai.com/v1/chat/completions"
	case "anthropic":
		return "https://api.anthropic.com/v1/messages"
	case "google":
		return "https://generativelanguage.googleapis.com/v1beta/models"
	default:
		return "https://api.openai.com/v1/chat/completions"
	}
}

// getProviderAPIKey returns the API key for the given provider
func (s *Service) getProviderAPIKey(provider string) string {
	switch provider {
	case "openai":
		return s.config.AI.OpenAIKey
	case "anthropic":
		return s.config.AI.AnthropicKey
	case "google":
		return s.config.AI.GoogleAIKey
	default:
		return s.config.AI.OpenAIKey
	}
}


// CallUpstream makes the actual call to the AI provider
func (s *Service) CallUpstream(ctx context.Context, agentConfig *models.AgentConfig, request map[string]interface{}) (*ChatResponse, error) {
	provider := agentConfig.Provider
	if provider == "" {
		provider = "openai"
	}

	// Execute with circuit breaker protection
	result, err := s.circuitBreakerManager.Execute(ctx, provider, func() (interface{}, error) {
		return s.callUpstreamInternal(ctx, agentConfig, request)
	})

	if err != nil {
		if errors.Is(err, ErrCircuitOpen) {
			return nil, fmt.Errorf("%w: %s provider circuit breaker is open", ErrUpstreamError, provider)
		}
		return nil, err
	}

	return result.(*ChatResponse), nil
}

// callUpstreamInternal makes the actual HTTP call to the AI provider
func (s *Service) callUpstreamInternal(ctx context.Context, agentConfig *models.AgentConfig, request map[string]interface{}) (*ChatResponse, error) {
	// Serialize request
	reqBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	url := s.getProviderURL(agentConfig.Provider)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	apiKey := s.getProviderAPIKey(agentConfig.Provider)
	
	switch agentConfig.Provider {
	case "anthropic":
		httpReq.Header.Set("x-api-key", apiKey)
		httpReq.Header.Set("anthropic-version", "2023-06-01")
	default:
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	}

	// Make request
	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, ErrUpstreamTimeout
		}
		return nil, fmt.Errorf("%w: %v", ErrUpstreamError, err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Error().
			Int("status", resp.StatusCode).
			Str("body", string(body)).
			Msg("Upstream error")
		return nil, fmt.Errorf("%w: status %d", ErrUpstreamError, resp.StatusCode)
	}

	// Parse response
	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &chatResp, nil
}

// CallUpstreamStream makes a streaming call to the AI provider
func (s *Service) CallUpstreamStream(ctx context.Context, agentConfig *models.AgentConfig, request map[string]interface{}, writer io.Writer, flusher http.Flusher) (*ChatUsage, error) {
	// Serialize request
	reqBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	url := s.getProviderURL(agentConfig.Provider)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	apiKey := s.getProviderAPIKey(agentConfig.Provider)
	
	switch agentConfig.Provider {
	case "anthropic":
		httpReq.Header.Set("x-api-key", apiKey)
		httpReq.Header.Set("anthropic-version", "2023-06-01")
	default:
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	}

	// Make request
	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, ErrUpstreamTimeout
		}
		return nil, fmt.Errorf("%w: %v", ErrUpstreamError, err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Error().
			Int("status", resp.StatusCode).
			Str("body", string(body)).
			Msg("Upstream streaming error")
		return nil, fmt.Errorf("%w: status %d", ErrUpstreamError, resp.StatusCode)
	}

	// Use the stream handler for processing
	streamConfig := DefaultStreamConfig(agentConfig.SystemPrompt)
	result, err := s.streamHandler.StreamResponse(ctx, resp.Body, writer, flusher, streamConfig)
	if err != nil {
		return nil, err
	}

	// Return usage info
	return &ChatUsage{
		CompletionTokens: result.TotalTokens,
		TotalTokens:      result.TotalTokens,
	}, nil
}


// SanitizeResponse removes any system prompt content from the response
// This ensures the hidden prompt is never exposed to the client
func (s *Service) SanitizeResponse(response *ChatResponse, systemPrompt string) *ChatResponse {
	return s.promptInjector.SanitizeResponse(response, systemPrompt)
}

// ProcessChat handles the complete chat flow
func (s *Service) ProcessChat(ctx context.Context, callCtx *CallContext, req *ChatRequest, writer io.Writer, flusher http.Flusher) (*CallResult, error) {
	result := &CallResult{
		Success: false,
	}

	// Build upstream request with injected system prompt
	upstreamReq, err := s.BuildUpstreamRequest(callCtx.AgentConfig, req.Messages, req.Stream)
	if err != nil {
		result.ErrorCode = "build_request_failed"
		return result, err
	}

	if req.Stream {
		// Streaming response
		usage, err := s.CallUpstreamStream(ctx, callCtx.AgentConfig, upstreamReq, writer, flusher)
		if err != nil {
			result.ErrorCode = "upstream_error"
			return result, err
		}

		result.Success = true
		if usage != nil {
			result.InputTokens = usage.PromptTokens
			result.OutputTokens = usage.CompletionTokens
		}
	} else {
		// Non-streaming response
		response, err := s.CallUpstream(ctx, callCtx.AgentConfig, upstreamReq)
		if err != nil {
			result.ErrorCode = "upstream_error"
			return result, err
		}

		// Sanitize response to ensure no prompt leakage
		response = s.SanitizeResponse(response, callCtx.AgentConfig.SystemPrompt)

		// Write response
		respBytes, err := json.Marshal(response)
		if err != nil {
			result.ErrorCode = "marshal_response_failed"
			return result, err
		}
		writer.Write(respBytes)

		result.Success = true
		if response.Usage != nil {
			result.InputTokens = response.Usage.PromptTokens
			result.OutputTokens = response.Usage.CompletionTokens
		}
	}

	// Calculate latency
	result.LatencyMs = int(time.Since(callCtx.StartTime).Milliseconds())

	// Calculate cost
	result.Cost = callCtx.Agent.PricePerCall

	return result, nil
}
