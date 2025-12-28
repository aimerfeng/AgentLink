package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/aimerfeng/AgentLink/internal/agent"
	"github.com/aimerfeng/AgentLink/internal/apikey"
	"github.com/aimerfeng/AgentLink/internal/cache"
	"github.com/aimerfeng/AgentLink/internal/config"
	apierrors "github.com/aimerfeng/AgentLink/internal/errors"
	"github.com/aimerfeng/AgentLink/internal/logging"
	"github.com/aimerfeng/AgentLink/internal/middleware"
	"github.com/aimerfeng/AgentLink/internal/monitoring"
	"github.com/aimerfeng/AgentLink/internal/proxy"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// ProxyServer represents the proxy gateway server
type ProxyServer struct {
	config       *config.Config
	router       *gin.Engine
	db           *pgxpool.Pool
	redis        *cache.Redis
	proxyService *proxy.Service
}

// NewProxyServer creates a new proxy server instance
func NewProxyServer(cfg *config.Config) *ProxyServer {
	if cfg.Server.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()

	// Add middleware in order
	router.Use(gin.Recovery())
	router.Use(middleware.RequestID())
	router.Use(middleware.CorrelationID())
	router.Use(middleware.CORS(cfg.CORS.AllowedOrigins))
	router.Use(monitoring.MetricsMiddleware())
	router.Use(logging.RequestLogger())

	srv := &ProxyServer{
		config: cfg,
		router: router,
	}

	srv.setupRoutes()
	return srv
}

// NewProxyServerWithDeps creates a new proxy server with dependencies
func NewProxyServerWithDeps(cfg *config.Config, db *pgxpool.Pool, redis *cache.Redis, agentSvc *agent.Service, apiKeySvc *apikey.Service) *ProxyServer {
	if cfg.Server.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()

	// Add middleware in order
	router.Use(gin.Recovery())
	router.Use(middleware.RequestID())
	router.Use(middleware.CorrelationID())
	router.Use(middleware.CORS(cfg.CORS.AllowedOrigins))
	router.Use(monitoring.MetricsMiddleware())
	router.Use(logging.RequestLogger())

	srv := &ProxyServer{
		config:       cfg,
		router:       router,
		db:           db,
		redis:        redis,
		proxyService: proxy.NewService(db, redis, agentSvc, apiKeySvc, cfg),
	}

	srv.setupRoutes()
	return srv
}


// Router returns the gin router
func (s *ProxyServer) Router() http.Handler {
	return s.router
}

// setupRoutes configures proxy routes
func (s *ProxyServer) setupRoutes() {
	// Health check
	s.router.GET("/health", s.healthCheck)

	// Proxy v1 routes
	v1 := s.router.Group("/proxy/v1")
	{
		v1.POST("/agents/:agentId/chat", s.handleChat)
	}
}

// Health check handler
func (s *ProxyServer) healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"service": "proxy",
	})
}

// handleChat handles AI chat requests through the proxy
func (s *ProxyServer) handleChat(c *gin.Context) {
	requestID := c.GetString("request_id")
	correlationID := c.GetString("correlation_id")
	startTime := time.Now()

	// Parse agent ID
	agentIDStr := c.Param("agentId")
	agentID, err := uuid.Parse(agentIDStr)
	if err != nil {
		s.sendError(c, requestID, apierrors.NewInvalidRequestError("invalid agent ID"))
		return
	}

	// Check if proxy service is initialized
	if s.proxyService == nil {
		s.sendError(c, requestID, &apierrors.APIError{
			Code:       apierrors.ErrInternalServer,
			Message:    "Proxy service not initialized",
			HTTPStatus: http.StatusInternalServerError,
		})
		return
	}

	// Validate API key from header
	apiKeyHeader := c.GetHeader("X-AgentLink-Key")
	if apiKeyHeader == "" {
		s.sendError(c, requestID, apierrors.ErrMissingAPIKeyError)
		return
	}

	// Validate API key
	apiKeyModel, err := s.proxyService.ValidateAPIKey(c.Request.Context(), apiKeyHeader)
	if err != nil {
		if errors.Is(err, proxy.ErrInvalidAPIKey) {
			s.sendError(c, requestID, apierrors.ErrInvalidAPIKeyError)
			return
		}
		log.Error().Err(err).Str("correlation_id", correlationID).Msg("Failed to validate API key")
		s.sendError(c, requestID, apierrors.ErrInternalServerError)
		return
	}

	// Get agent and validate it's active
	agentModel, agentConfig, err := s.proxyService.GetAgent(c.Request.Context(), agentID)
	if err != nil {
		if errors.Is(err, proxy.ErrAgentNotFound) {
			s.sendError(c, requestID, apierrors.ErrAgentNotFoundError)
			return
		}
		if errors.Is(err, proxy.ErrAgentNotActive) {
			s.sendError(c, requestID, apierrors.ErrAgentNotActiveError)
			return
		}
		log.Error().Err(err).Str("correlation_id", correlationID).Msg("Failed to get agent")
		s.sendError(c, requestID, apierrors.ErrInternalServerError)
		return
	}

	// Check if user is paid
	isPaidUser, err := s.proxyService.IsPaidUser(c.Request.Context(), apiKeyModel.UserID)
	if err != nil {
		log.Warn().Err(err).Str("correlation_id", correlationID).Msg("Failed to check paid status, assuming free user")
		isPaidUser = false
	}

	// Check rate limit with detailed result
	rateLimitResult, err := s.proxyService.CheckRateLimitWithResult(c.Request.Context(), apiKeyModel.UserID, isPaidUser)
	if err != nil {
		log.Error().Err(err).Str("correlation_id", correlationID).Msg("Failed to check rate limit")
		s.sendError(c, requestID, apierrors.ErrInternalServerError)
		return
	}
	c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", rateLimitResult.Remaining))
	c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", rateLimitResult.Limit))
	if !rateLimitResult.Allowed {
		retryAfterSeconds := int64(rateLimitResult.RetryAfter.Seconds())
		if retryAfterSeconds < 1 {
			retryAfterSeconds = 1
		}
		s.sendRateLimitError(c, requestID, retryAfterSeconds)
		return
	}

	// Check quota
	quotaRemaining, err := s.proxyService.CheckQuota(c.Request.Context(), apiKeyModel.UserID)
	if err != nil {
		log.Error().Err(err).Str("correlation_id", correlationID).Msg("Failed to check quota")
		s.sendError(c, requestID, apierrors.ErrInternalServerError)
		return
	}
	if quotaRemaining <= 0 {
		s.sendError(c, requestID, apierrors.ErrQuotaExhaustedError)
		return
	}

	// Parse request body
	var req proxy.ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		s.sendError(c, requestID, apierrors.NewValidationError(err.Error()))
		return
	}

	// Validate messages
	if len(req.Messages) == 0 {
		s.sendError(c, requestID, apierrors.NewValidationError("messages cannot be empty"))
		return
	}

	// Create call context with correlation ID
	callCtx := &proxy.CallContext{
		RequestID:     requestID,
		CorrelationID: correlationID,
		AgentID:       agentID,
		UserID:        apiKeyModel.UserID,
		APIKeyID:      apiKeyModel.ID,
		Agent:         agentModel,
		AgentConfig:   agentConfig,
		StartTime:     startTime,
		IsPaidUser:    isPaidUser,
	}

	// Decrement quota before making the call
	_, err = s.proxyService.DecrementQuota(c.Request.Context(), apiKeyModel.UserID, 1)
	if err != nil {
		log.Error().Err(err).Str("correlation_id", correlationID).Msg("Failed to decrement quota")
		s.sendError(c, requestID, apierrors.ErrInternalServerError)
		return
	}

	// Process the chat request
	var result *proxy.CallResult
	if req.Stream {
		// Set up SSE headers
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("X-Request-ID", requestID)

		// Get the flusher
		flusher, ok := c.Writer.(http.Flusher)
		if !ok {
			s.proxyService.RefundQuota(c.Request.Context(), apiKeyModel.UserID, 1)
			s.sendError(c, requestID, apierrors.NewInvalidRequestError("streaming not supported"))
			return
		}

		// Process streaming chat
		result, err = s.proxyService.ProcessChat(c.Request.Context(), callCtx, &req, c.Writer, flusher)
	} else {
		// Set response headers
		c.Header("Content-Type", "application/json")
		c.Header("X-Request-ID", requestID)

		// Process non-streaming chat
		result, err = s.proxyService.ProcessChat(c.Request.Context(), callCtx, &req, c.Writer, nil)
	}

	// Handle errors
	if err != nil {
		// Refund quota on failure - failed calls don't cost quota (Requirement A6.5)
		refundErr := s.proxyService.RefundQuota(c.Request.Context(), apiKeyModel.UserID, 1)
		if refundErr != nil {
			log.Error().Err(refundErr).Str("correlation_id", correlationID).Msg("Failed to refund quota")
		}

		// Log the failed call
		if result == nil {
			result = &proxy.CallResult{
				Success:   false,
				LatencyMs: int(time.Since(startTime).Milliseconds()),
			}
		}
		result.Success = false

		// Determine error code for logging
		switch {
		case errors.Is(err, proxy.ErrUpstreamTimeout):
			result.ErrorCode = "upstream_timeout"
			s.sendError(c, requestID, apierrors.ErrUpstreamTimeoutError)
		case errors.Is(err, proxy.ErrUpstreamError):
			result.ErrorCode = "upstream_error"
			s.sendError(c, requestID, apierrors.ErrUpstreamUnavailableError)
		case errors.Is(err, proxy.ErrCircuitOpen):
			result.ErrorCode = "circuit_breaker_open"
			s.sendError(c, requestID, apierrors.ErrCircuitBreakerOpenError)
		default:
			result.ErrorCode = "internal_error"
			log.Error().Err(err).Str("correlation_id", correlationID).Msg("Failed to process chat")
			s.sendError(c, requestID, apierrors.ErrInternalServerError)
		}

		// Log the failed call asynchronously
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if logErr := s.proxyService.LogCall(ctx, callCtx, result); logErr != nil {
				log.Error().Err(logErr).Str("correlation_id", correlationID).Msg("Failed to log failed call")
			}
		}()
		return
	}

	// Log the call asynchronously
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.proxyService.LogCall(ctx, callCtx, result); err != nil {
			log.Error().Err(err).Msg("Failed to log call")
		}
	}()
}

// sendError sends a standardized error response with correlation ID
func (s *ProxyServer) sendError(c *gin.Context, requestID string, apiErr *apierrors.APIError) {
	correlationID := c.GetString("correlation_id")
	if correlationID == "" {
		correlationID = requestID // Use request ID as correlation ID if not set
	}

	response := apierrors.NewErrorResponse(
		apiErr,
		requestID,
		correlationID,
		c.Request.URL.Path,
		c.Request.Method,
	)

	// Set correlation ID header for tracing
	c.Header("X-Correlation-ID", correlationID)

	c.JSON(apiErr.HTTPStatus, response)
}

// sendRateLimitError sends a rate limit error with Retry-After header
func (s *ProxyServer) sendRateLimitError(c *gin.Context, requestID string, retryAfter int64) {
	correlationID := c.GetString("correlation_id")
	if correlationID == "" {
		correlationID = requestID
	}

	apiErr := apierrors.NewRateLimitError(retryAfter)
	response := &apierrors.RateLimitErrorResponse{
		ErrorResponse: *apierrors.NewErrorResponse(
			apiErr,
			requestID,
			correlationID,
			c.Request.URL.Path,
			c.Request.Method,
		),
		RetryAfter: retryAfter,
	}

	// Set standard Retry-After header
	c.Header("Retry-After", fmt.Sprintf("%d", retryAfter))
	c.Header("X-Correlation-ID", correlationID)

	c.JSON(apiErr.HTTPStatus, response)
}
