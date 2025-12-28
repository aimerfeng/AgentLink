package server

import (
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/aimerfeng/AgentLink/internal/agent"
	"github.com/aimerfeng/AgentLink/internal/apikey"
	"github.com/aimerfeng/AgentLink/internal/auth"
	"github.com/aimerfeng/AgentLink/internal/config"
	apierrors "github.com/aimerfeng/AgentLink/internal/errors"
	"github.com/aimerfeng/AgentLink/internal/logging"
	"github.com/aimerfeng/AgentLink/internal/middleware"
	"github.com/aimerfeng/AgentLink/internal/monitoring"
	"github.com/aimerfeng/AgentLink/internal/payment"
	"github.com/aimerfeng/AgentLink/internal/trial"
	"github.com/aimerfeng/AgentLink/internal/withdrawal"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// APIServer represents the main API server
type APIServer struct {
	config             *config.Config
	router             *gin.Engine
	db                 *pgxpool.Pool
	authService        *auth.Service
	agentService       *agent.Service
	apiKeyService      *apikey.Service
	paymentService     *payment.Service
	trialService       *trial.Service
	withdrawalService  *withdrawal.Service
	jwtAuthenticator   *middleware.JWTAuthenticator
}

// NewAPIServer creates a new API server instance
func NewAPIServer(cfg *config.Config, db *pgxpool.Pool) *APIServer {
	if cfg.Server.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()

	// Add middleware in order
	router.Use(gin.Recovery())
	router.Use(middleware.RequestID())
	router.Use(middleware.CORS(cfg.CORS.AllowedOrigins))
	router.Use(monitoring.MetricsMiddleware())
	router.Use(logging.RequestLogger())

	// Create auth service
	authService := auth.NewService(db, &cfg.JWT, &cfg.Quota)

	// Create agent service
	agentService, err := agent.NewService(db, &cfg.Encryption)
	if err != nil {
		// Log error but continue - encryption key might not be set in dev
		agentService = nil
	}

	// Create API key service
	apiKeyService := apikey.NewService(db)

	// Create payment service
	paymentService := payment.NewService(db, &cfg.Stripe, &cfg.Coinbase, cfg.Server.URL, cfg.Features.CryptoPaymentEnabled)

	// Create trial service
	trialService := trial.NewService(db, &cfg.Quota)

	// Create withdrawal service
	withdrawalService := withdrawal.NewService(db, nil) // Uses default config

	// Create JWT authenticator for middleware
	jwtAuthenticator := middleware.NewJWTAuthenticator(&cfg.JWT)

	srv := &APIServer{
		config:             cfg,
		router:             router,
		db:                 db,
		authService:        authService,
		agentService:       agentService,
		apiKeyService:      apiKeyService,
		paymentService:     paymentService,
		trialService:       trialService,
		withdrawalService:  withdrawalService,
		jwtAuthenticator:   jwtAuthenticator,
	}

	srv.setupRoutes()
	return srv
}

// Router returns the gin router
func (s *APIServer) Router() http.Handler {
	return s.router
}

// setupRoutes configures all API routes
func (s *APIServer) setupRoutes() {
	// Health check
	s.router.GET("/health", s.healthCheck)

	// API v1 routes
	v1 := s.router.Group("/api/v1")
	{
		// Auth routes (public)
		authGroup := v1.Group("/auth")
		{
			authGroup.POST("/register", s.handleRegister)
			authGroup.POST("/login", s.handleLogin)
			authGroup.POST("/logout", s.handleLogout)
			authGroup.POST("/refresh", s.handleRefresh)
		}

		// Creator routes (protected - requires creator role)
		creators := v1.Group("/creators")
		creators.Use(s.jwtAuthenticator.JWTAuth())
		creators.Use(middleware.RequireCreator())
		{
			creators.GET("/me", s.handleGetCreator)
			creators.PUT("/me", s.handleUpdateCreator)
			creators.PUT("/me/wallet", s.handleBindWallet)
		}

		// Agent routes (protected - requires creator role for management)
		agents := v1.Group("/agents")
		agents.Use(s.jwtAuthenticator.JWTAuth())
		agents.Use(middleware.RequireCreator())
		{
			agents.POST("/", s.handleCreateAgent)
			agents.GET("/", s.handleListAgents)
			agents.GET("/:id", s.handleGetAgent)
			agents.PUT("/:id", s.handleUpdateAgent)
			agents.POST("/:id/publish", s.handlePublishAgent)
			agents.POST("/:id/unpublish", s.handleUnpublishAgent)
			agents.GET("/:id/versions", s.handleListAgentVersions)
			agents.GET("/:id/versions/:version", s.handleGetAgentVersion)
			agents.POST("/:id/knowledge", s.handleUploadKnowledge)
			// D5.4: Creator can disable trial for their agents
			agents.PUT("/:id/trial", s.handleSetAgentTrial)
		}

		// Trial routes (protected - requires developer role)
		trials := v1.Group("/trials")
		trials.Use(s.jwtAuthenticator.JWTAuth())
		trials.Use(middleware.RequireDeveloper())
		{
			// D5.2: Get trial info for a specific agent
			trials.GET("/agents/:id", s.handleGetTrialInfo)
			// Get all trial usage for the user
			trials.GET("/", s.handleGetUserTrialUsage)
		}

		// Marketplace routes (public)
		marketplace := v1.Group("/marketplace")
		{
			marketplace.GET("/agents", s.handleSearchAgents)
			marketplace.GET("/agents/:id", s.handleGetPublicAgent)
			marketplace.GET("/categories", s.handleGetCategories)
			marketplace.GET("/featured", s.handleGetFeatured)
		}

		// Developer routes (protected - requires developer role)
		developers := v1.Group("/developers")
		developers.Use(s.jwtAuthenticator.JWTAuth())
		developers.Use(middleware.RequireDeveloper())
		{
			developers.GET("/me", s.handleGetDeveloper)
			developers.GET("/keys", s.handleListAPIKeys)
			developers.POST("/keys", s.handleCreateAPIKey)
			developers.DELETE("/keys/:id", s.handleDeleteAPIKey)
			developers.GET("/usage", s.handleGetUsage)
		}

		// Payment routes (protected - requires any authenticated user)
		payments := v1.Group("/payments")
		{
			payments.POST("/checkout", s.jwtAuthenticator.JWTAuth(), s.handleCheckout)
			payments.POST("/checkout/coinbase", s.jwtAuthenticator.JWTAuth(), s.handleCoinbaseCheckout)
			payments.POST("/webhook/stripe", s.handleStripeWebhook)
			payments.POST("/webhook/coinbase", s.handleCoinbaseWebhook)
			payments.GET("/history", s.jwtAuthenticator.JWTAuth(), s.handlePaymentHistory)
			payments.GET("/packages", s.handleGetPackages)
			payments.GET("/quota", s.jwtAuthenticator.JWTAuth(), s.handleGetQuota)
		}

		// Review routes (mixed - some public, some protected)
		reviews := v1.Group("/reviews")
		{
			reviews.POST("/agents/:id", s.jwtAuthenticator.JWTAuth(), s.handleSubmitReview)
			reviews.GET("/agents/:id", s.handleGetReviews)
		}

		// Webhook routes (protected - requires developer role)
		webhooks := v1.Group("/webhooks")
		webhooks.Use(s.jwtAuthenticator.JWTAuth())
		webhooks.Use(middleware.RequireDeveloper())
		{
			webhooks.GET("/", s.handleListWebhooks)
			webhooks.POST("/", s.handleCreateWebhook)
			webhooks.DELETE("/:id", s.handleDeleteWebhook)
		}

		// Admin routes (protected - requires admin role)
		admin := v1.Group("/admin")
		admin.Use(s.jwtAuthenticator.JWTAuth())
		admin.Use(middleware.RequireAdmin())
		{
			admin.GET("/dashboard/stats", s.handleAdminStats)
			admin.GET("/users", s.handleAdminListUsers)
			admin.GET("/users/:id", s.handleAdminGetUser)
			admin.PUT("/users/:id/status", s.handleAdminUpdateUserStatus)
			admin.GET("/moderation/queue", s.handleAdminModerationQueue)
			admin.POST("/moderation/reviews/:id/approve", s.handleAdminApproveReview)
			admin.POST("/moderation/reviews/:id/reject", s.handleAdminRejectReview)
			// Admin withdrawal management
			admin.GET("/withdrawals/pending", s.handleAdminGetPendingWithdrawals)
			admin.POST("/withdrawals/:id/process", s.handleAdminProcessWithdrawal)
			admin.POST("/withdrawals/:id/complete", s.handleAdminCompleteWithdrawal)
			admin.POST("/withdrawals/:id/fail", s.handleAdminFailWithdrawal)
		}

		// Withdrawal routes (protected - requires creator role)
		withdrawals := v1.Group("/withdrawals")
		withdrawals.Use(s.jwtAuthenticator.JWTAuth())
		withdrawals.Use(middleware.RequireCreator())
		{
			withdrawals.GET("/earnings", s.handleGetEarnings)
			withdrawals.POST("/", s.handleCreateWithdrawal)
			withdrawals.GET("/", s.handleGetWithdrawalHistory)
			withdrawals.GET("/:id", s.handleGetWithdrawal)
			withdrawals.GET("/config", s.handleGetWithdrawalConfig)
		}
	}
}

// Health check handler
func (s *APIServer) healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"service": "api",
	})
}

// handleRegister handles user registration
func (s *APIServer) handleRegister(c *gin.Context) {
	var req auth.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apierrors.NewValidationError(err.Error()))
		return
	}

	resp, err := s.authService.Register(c.Request.Context(), &req)
	if err != nil {
		switch err {
		case auth.ErrEmailAlreadyExists:
			respondError(c, apierrors.NewInvalidRequestError("Email already registered"))
		case auth.ErrDisplayNameRequired:
			respondError(c, apierrors.NewValidationError("Display name is required for creators"))
		default:
			respondError(c, apierrors.ErrInternalServerError)
		}
		return
	}

	c.JSON(http.StatusCreated, resp)
}

// handleLogin handles user login
func (s *APIServer) handleLogin(c *gin.Context) {
	var req auth.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apierrors.NewValidationError(err.Error()))
		return
	}

	resp, err := s.authService.Login(c.Request.Context(), &req)
	if err != nil {
		if err == auth.ErrInvalidCredentials {
			respondError(c, apierrors.ErrInvalidCredentialsError)
		} else {
			respondError(c, apierrors.ErrInternalServerError)
		}
		return
	}

	c.JSON(http.StatusOK, resp)
}

// handleLogout handles user logout
func (s *APIServer) handleLogout(c *gin.Context) {
	// For stateless JWT, logout is handled client-side by removing the token
	// In a more complex system, we could blacklist the token in Redis
	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}

// handleRefresh handles token refresh
func (s *APIServer) handleRefresh(c *gin.Context) {
	var req auth.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apierrors.NewValidationError(err.Error()))
		return
	}

	tokens, err := s.authService.RefreshTokens(c.Request.Context(), req.RefreshToken)
	if err != nil {
		switch err {
		case auth.ErrInvalidToken:
			respondError(c, apierrors.ErrInvalidCredentialsError)
		case auth.ErrTokenExpired:
			respondError(c, apierrors.ErrTokenExpiredError)
		case auth.ErrUserNotFound:
			respondError(c, apierrors.ErrUserNotFoundError)
		default:
			respondError(c, apierrors.ErrInternalServerError)
		}
		return
	}

	c.JSON(http.StatusOK, tokens)
}

// respondError sends a standardized error response
func respondError(c *gin.Context, err *apierrors.APIError) {
	requestID, _ := c.Get("request_id")
	reqIDStr, _ := requestID.(string)
	correlationID, _ := c.Get("correlation_id")
	corrIDStr, _ := correlationID.(string)
	if corrIDStr == "" {
		corrIDStr = reqIDStr
	}

	response := apierrors.NewErrorResponse(
		err,
		reqIDStr,
		corrIDStr,
		c.Request.URL.Path,
		c.Request.Method,
	)

	c.JSON(err.HTTPStatus, response)
}

// Placeholder handlers - to be implemented in subsequent tasks
func (s *APIServer) handleGetCreator(c *gin.Context)      { c.JSON(501, gin.H{"error": "not implemented"}) }
func (s *APIServer) handleUpdateCreator(c *gin.Context)   { c.JSON(501, gin.H{"error": "not implemented"}) }

// handleBindWallet handles wallet address binding for creators
func (s *APIServer) handleBindWallet(c *gin.Context) {
	// Get user ID from context (set by JWT auth middleware)
	userIDStr := middleware.GetUserIDFromContext(c)
	if userIDStr == "" {
		respondError(c, apierrors.ErrInvalidCredentialsError)
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		respondError(c, apierrors.ErrInvalidCredentialsError)
		return
	}

	// Parse request body
	var req auth.BindWalletRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apierrors.NewValidationError(err.Error()))
		return
	}

	// Bind wallet
	resp, err := s.authService.BindWallet(c.Request.Context(), userID, &req)
	if err != nil {
		switch err {
		case auth.ErrInvalidWalletAddress:
			respondError(c, apierrors.NewValidationError("Invalid Ethereum wallet address format. Address must be 42 characters starting with 0x followed by 40 hex characters."))
		case auth.ErrNotCreator:
			respondError(c, apierrors.NewInvalidRequestError("Only creators can bind wallet addresses"))
		case auth.ErrUserNotFound:
			respondError(c, apierrors.ErrUserNotFoundError)
		default:
			respondError(c, apierrors.ErrInternalServerError)
		}
		return
	}

	c.JSON(http.StatusOK, resp)
}

// handleCreateAgent handles agent creation
func (s *APIServer) handleCreateAgent(c *gin.Context) {
	if s.agentService == nil {
		respondError(c, apierrors.NewInvalidRequestError("Agent service not available"))
		return
	}

	// Get user ID from context
	userIDStr := middleware.GetUserIDFromContext(c)
	if userIDStr == "" {
		respondError(c, apierrors.ErrInvalidCredentialsError)
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		respondError(c, apierrors.ErrInvalidCredentialsError)
		return
	}

	// Parse request body
	var req agent.CreateAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apierrors.NewValidationError(err.Error()))
		return
	}

	// Create agent
	resp, err := s.agentService.Create(c.Request.Context(), userID, &req)
	if err != nil {
		switch err {
		case agent.ErrInvalidPrice:
			respondError(c, apierrors.NewValidationError("Price must be between $0.001 and $100 per call"))
		default:
			if err.Error() != "" && err.Error()[:len("invalid agent configuration")] == "invalid agent configuration" {
				respondError(c, apierrors.NewValidationError(err.Error()))
			} else {
				respondError(c, apierrors.ErrInternalServerError)
			}
		}
		return
	}

	c.JSON(http.StatusCreated, resp)
}

// handleListAgents handles listing agents for a creator
func (s *APIServer) handleListAgents(c *gin.Context) {
	if s.agentService == nil {
		respondError(c, apierrors.NewInvalidRequestError("Agent service not available"))
		return
	}

	// Get user ID from context
	userIDStr := middleware.GetUserIDFromContext(c)
	if userIDStr == "" {
		respondError(c, apierrors.ErrInvalidCredentialsError)
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		respondError(c, apierrors.ErrInvalidCredentialsError)
		return
	}

	// Parse pagination params
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	// List agents
	resp, err := s.agentService.List(c.Request.Context(), userID, page, pageSize)
	if err != nil {
		respondError(c, apierrors.ErrInternalServerError)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// handleGetAgent handles getting a single agent
func (s *APIServer) handleGetAgent(c *gin.Context) {
	if s.agentService == nil {
		respondError(c, apierrors.NewInvalidRequestError("Agent service not available"))
		return
	}

	// Get user ID from context
	userIDStr := middleware.GetUserIDFromContext(c)
	if userIDStr == "" {
		respondError(c, apierrors.ErrInvalidCredentialsError)
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		respondError(c, apierrors.ErrInvalidCredentialsError)
		return
	}

	// Parse agent ID
	agentIDStr := c.Param("id")
	agentID, err := uuid.Parse(agentIDStr)
	if err != nil {
		respondError(c, apierrors.NewValidationError("Invalid agent ID"))
		return
	}

	// Get agent
	resp, err := s.agentService.GetByIDForOwner(c.Request.Context(), agentID, userID)
	if err != nil {
		switch err {
		case agent.ErrAgentNotFound:
			respondError(c, apierrors.ErrAgentNotFoundError)
		case agent.ErrAgentNotOwned:
			respondError(c, &apierrors.APIError{
				Code:       apierrors.ErrAgentNotOwned,
				Message:    "You do not own this agent",
				HTTPStatus: http.StatusForbidden,
			})
		default:
			respondError(c, apierrors.ErrInternalServerError)
		}
		return
	}

	c.JSON(http.StatusOK, resp)
}

// handleUpdateAgent handles updating an agent
func (s *APIServer) handleUpdateAgent(c *gin.Context) {
	if s.agentService == nil {
		respondError(c, apierrors.NewInvalidRequestError("Agent service not available"))
		return
	}

	// Get user ID from context
	userIDStr := middleware.GetUserIDFromContext(c)
	if userIDStr == "" {
		respondError(c, apierrors.ErrInvalidCredentialsError)
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		respondError(c, apierrors.ErrInvalidCredentialsError)
		return
	}

	// Parse agent ID
	agentIDStr := c.Param("id")
	agentID, err := uuid.Parse(agentIDStr)
	if err != nil {
		respondError(c, apierrors.NewValidationError("Invalid agent ID"))
		return
	}

	// Parse request body
	var req agent.UpdateAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apierrors.NewValidationError(err.Error()))
		return
	}

	// Update agent
	resp, err := s.agentService.Update(c.Request.Context(), agentID, userID, &req)
	if err != nil {
		switch err {
		case agent.ErrAgentNotFound:
			respondError(c, apierrors.ErrAgentNotFoundError)
		case agent.ErrAgentNotOwned:
			respondError(c, &apierrors.APIError{
				Code:       apierrors.ErrAgentNotOwned,
				Message:    "You do not own this agent",
				HTTPStatus: http.StatusForbidden,
			})
		case agent.ErrInvalidPrice:
			respondError(c, apierrors.NewValidationError("Price must be between $0.001 and $100 per call"))
		default:
			respondError(c, apierrors.ErrInternalServerError)
		}
		return
	}

	c.JSON(http.StatusOK, resp)
}

// handlePublishAgent handles publishing an agent
func (s *APIServer) handlePublishAgent(c *gin.Context) {
	if s.agentService == nil {
		respondError(c, apierrors.NewInvalidRequestError("Agent service not available"))
		return
	}

	// Get user ID from context
	userIDStr := middleware.GetUserIDFromContext(c)
	if userIDStr == "" {
		respondError(c, apierrors.ErrInvalidCredentialsError)
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		respondError(c, apierrors.ErrInvalidCredentialsError)
		return
	}

	// Parse agent ID
	agentIDStr := c.Param("id")
	agentID, err := uuid.Parse(agentIDStr)
	if err != nil {
		respondError(c, apierrors.NewValidationError("Invalid agent ID"))
		return
	}

	// Publish agent
	resp, err := s.agentService.Publish(c.Request.Context(), agentID, userID)
	if err != nil {
		switch err {
		case agent.ErrAgentNotFound:
			respondError(c, apierrors.ErrAgentNotFoundError)
		case agent.ErrAgentNotOwned:
			respondError(c, &apierrors.APIError{
				Code:       apierrors.ErrAgentNotOwned,
				Message:    "You do not own this agent",
				HTTPStatus: http.StatusForbidden,
			})
		case agent.ErrAgentAlreadyActive:
			respondError(c, apierrors.NewInvalidRequestError("Agent is already published"))
		default:
			respondError(c, apierrors.ErrInternalServerError)
		}
		return
	}

	c.JSON(http.StatusOK, resp)
}

// handleUnpublishAgent handles unpublishing an agent
func (s *APIServer) handleUnpublishAgent(c *gin.Context) {
	if s.agentService == nil {
		respondError(c, apierrors.NewInvalidRequestError("Agent service not available"))
		return
	}

	// Get user ID from context
	userIDStr := middleware.GetUserIDFromContext(c)
	if userIDStr == "" {
		respondError(c, apierrors.ErrInvalidCredentialsError)
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		respondError(c, apierrors.ErrInvalidCredentialsError)
		return
	}

	// Parse agent ID
	agentIDStr := c.Param("id")
	agentID, err := uuid.Parse(agentIDStr)
	if err != nil {
		respondError(c, apierrors.NewValidationError("Invalid agent ID"))
		return
	}

	// Unpublish agent
	resp, err := s.agentService.Unpublish(c.Request.Context(), agentID, userID)
	if err != nil {
		switch err {
		case agent.ErrAgentNotFound:
			respondError(c, apierrors.ErrAgentNotFoundError)
		case agent.ErrAgentNotOwned:
			respondError(c, &apierrors.APIError{
				Code:       apierrors.ErrAgentNotOwned,
				Message:    "You do not own this agent",
				HTTPStatus: http.StatusForbidden,
			})
		default:
			respondError(c, apierrors.ErrInternalServerError)
		}
		return
	}

	c.JSON(http.StatusOK, resp)
}

// handleListAgentVersions handles listing all versions of an agent
func (s *APIServer) handleListAgentVersions(c *gin.Context) {
	if s.agentService == nil {
		respondError(c, apierrors.NewInvalidRequestError("Agent service not available"))
		return
	}

	// Get user ID from context
	userIDStr := middleware.GetUserIDFromContext(c)
	if userIDStr == "" {
		respondError(c, apierrors.ErrInvalidCredentialsError)
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		respondError(c, apierrors.ErrInvalidCredentialsError)
		return
	}

	// Parse agent ID
	agentIDStr := c.Param("id")
	agentID, err := uuid.Parse(agentIDStr)
	if err != nil {
		respondError(c, apierrors.NewValidationError("Invalid agent ID"))
		return
	}

	// Get versions
	resp, err := s.agentService.GetVersions(c.Request.Context(), agentID, userID)
	if err != nil {
		switch err {
		case agent.ErrAgentNotFound:
			respondError(c, apierrors.ErrAgentNotFoundError)
		case agent.ErrAgentNotOwned:
			respondError(c, &apierrors.APIError{
				Code:       apierrors.ErrAgentNotOwned,
				Message:    "You do not own this agent",
				HTTPStatus: http.StatusForbidden,
			})
		default:
			respondError(c, apierrors.ErrInternalServerError)
		}
		return
	}

	c.JSON(http.StatusOK, resp)
}

// handleGetAgentVersion handles getting a specific version of an agent
func (s *APIServer) handleGetAgentVersion(c *gin.Context) {
	if s.agentService == nil {
		respondError(c, apierrors.NewInvalidRequestError("Agent service not available"))
		return
	}

	// Get user ID from context
	userIDStr := middleware.GetUserIDFromContext(c)
	if userIDStr == "" {
		respondError(c, apierrors.ErrInvalidCredentialsError)
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		respondError(c, apierrors.ErrInvalidCredentialsError)
		return
	}

	// Parse agent ID
	agentIDStr := c.Param("id")
	agentID, err := uuid.Parse(agentIDStr)
	if err != nil {
		respondError(c, apierrors.NewValidationError("Invalid agent ID"))
		return
	}

	// Parse version number
	versionStr := c.Param("version")
	version, err := strconv.Atoi(versionStr)
	if err != nil {
		respondError(c, apierrors.NewValidationError("Invalid version number"))
		return
	}

	// Get version
	resp, err := s.agentService.GetVersion(c.Request.Context(), agentID, userID, version)
	if err != nil {
		switch err {
		case agent.ErrAgentNotFound:
			respondError(c, apierrors.ErrAgentNotFoundError)
		case agent.ErrAgentNotOwned:
			respondError(c, &apierrors.APIError{
				Code:       apierrors.ErrAgentNotOwned,
				Message:    "You do not own this agent",
				HTTPStatus: http.StatusForbidden,
			})
		default:
			// Check if it's a version not found error
			if err.Error() == fmt.Sprintf("version %d not found", version) {
				respondError(c, apierrors.NewInvalidRequestError(err.Error()))
			} else {
				respondError(c, apierrors.ErrInternalServerError)
			}
		}
		return
	}

	c.JSON(http.StatusOK, resp)
}
func (s *APIServer) handleUploadKnowledge(c *gin.Context) { c.JSON(501, gin.H{"error": "not implemented"}) }
func (s *APIServer) handleSearchAgents(c *gin.Context)    { c.JSON(501, gin.H{"error": "not implemented"}) }
func (s *APIServer) handleGetPublicAgent(c *gin.Context)  { c.JSON(501, gin.H{"error": "not implemented"}) }
func (s *APIServer) handleGetCategories(c *gin.Context)   { c.JSON(501, gin.H{"error": "not implemented"}) }
func (s *APIServer) handleGetFeatured(c *gin.Context)     { c.JSON(501, gin.H{"error": "not implemented"}) }
func (s *APIServer) handleGetDeveloper(c *gin.Context)    { c.JSON(501, gin.H{"error": "not implemented"}) }

// handleListAPIKeys handles listing all API keys for a developer
func (s *APIServer) handleListAPIKeys(c *gin.Context) {
	// Get user ID from context
	userIDStr := middleware.GetUserIDFromContext(c)
	if userIDStr == "" {
		respondError(c, apierrors.ErrInvalidCredentialsError)
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		respondError(c, apierrors.ErrInvalidCredentialsError)
		return
	}

	// List API keys
	resp, err := s.apiKeyService.List(c.Request.Context(), userID)
	if err != nil {
		respondError(c, apierrors.ErrInternalServerError)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// handleCreateAPIKey handles creating a new API key
func (s *APIServer) handleCreateAPIKey(c *gin.Context) {
	// Get user ID from context
	userIDStr := middleware.GetUserIDFromContext(c)
	if userIDStr == "" {
		respondError(c, apierrors.ErrInvalidCredentialsError)
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		respondError(c, apierrors.ErrInvalidCredentialsError)
		return
	}

	// Parse request body
	var req apikey.CreateAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Allow empty body for default key creation
		req = apikey.CreateAPIKeyRequest{}
	}

	// Create API key
	resp, err := s.apiKeyService.Create(c.Request.Context(), userID, &req)
	if err != nil {
		switch err {
		case apikey.ErrMaxKeysReached:
			respondError(c, apierrors.NewInvalidRequestError("Maximum number of API keys reached (10)"))
		default:
			respondError(c, apierrors.ErrInternalServerError)
		}
		return
	}

	c.JSON(http.StatusCreated, resp)
}

// handleDeleteAPIKey handles revoking/deleting an API key
func (s *APIServer) handleDeleteAPIKey(c *gin.Context) {
	// Get user ID from context
	userIDStr := middleware.GetUserIDFromContext(c)
	if userIDStr == "" {
		respondError(c, apierrors.ErrInvalidCredentialsError)
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		respondError(c, apierrors.ErrInvalidCredentialsError)
		return
	}

	// Parse key ID from URL
	keyIDStr := c.Param("id")
	keyID, err := uuid.Parse(keyIDStr)
	if err != nil {
		respondError(c, apierrors.NewValidationError("Invalid API key ID"))
		return
	}

	// Delete (revoke) API key
	err = s.apiKeyService.Delete(c.Request.Context(), keyID, userID)
	if err != nil {
		switch err {
		case apikey.ErrAPIKeyNotFound:
			respondError(c, &apierrors.APIError{
				Code:       apierrors.ErrInvalidRequest,
				Message:    "API key not found",
				HTTPStatus: http.StatusNotFound,
			})
		case apikey.ErrAPIKeyNotOwned:
			respondError(c, apierrors.ErrForbiddenError)
		case apikey.ErrAPIKeyRevoked:
			respondError(c, apierrors.NewInvalidRequestError("API key is already revoked"))
		default:
			respondError(c, apierrors.ErrInternalServerError)
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "API key revoked successfully"})
}
func (s *APIServer) handleGetUsage(c *gin.Context)        { c.JSON(501, gin.H{"error": "not implemented"}) }

// handleCheckout handles Stripe checkout session creation
func (s *APIServer) handleCheckout(c *gin.Context) {
	if s.paymentService == nil {
		respondError(c, apierrors.NewInvalidRequestError("Payment service not available"))
		return
	}

	// Get user ID from context
	userIDStr := middleware.GetUserIDFromContext(c)
	if userIDStr == "" {
		respondError(c, apierrors.ErrInvalidCredentialsError)
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		respondError(c, apierrors.ErrInvalidCredentialsError)
		return
	}

	// Parse request body
	var req payment.CreateCheckoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apierrors.NewValidationError(err.Error()))
		return
	}

	// Create checkout session
	resp, err := s.paymentService.CreateCheckoutSession(c.Request.Context(), userID, &req)
	if err != nil {
		switch err {
		case payment.ErrInvalidAmount:
			respondError(c, apierrors.NewValidationError("Invalid package ID"))
		case payment.ErrUserNotFound:
			respondError(c, apierrors.ErrUserNotFoundError)
		default:
			respondError(c, apierrors.ErrInternalServerError)
		}
		return
	}

	c.JSON(http.StatusOK, resp)
}

// handleCoinbaseCheckout handles Coinbase Commerce charge creation
func (s *APIServer) handleCoinbaseCheckout(c *gin.Context) {
	if s.paymentService == nil {
		respondError(c, apierrors.NewInvalidRequestError("Payment service not available"))
		return
	}

	// Get user ID from context
	userIDStr := middleware.GetUserIDFromContext(c)
	if userIDStr == "" {
		respondError(c, apierrors.ErrInvalidCredentialsError)
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		respondError(c, apierrors.ErrInvalidCredentialsError)
		return
	}

	// Parse request body
	var req payment.CoinbaseChargeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apierrors.NewValidationError(err.Error()))
		return
	}

	// Create Coinbase charge
	resp, err := s.paymentService.CreateCoinbaseCharge(c.Request.Context(), userID, &req)
	if err != nil {
		switch err {
		case payment.ErrInvalidAmount:
			respondError(c, apierrors.NewValidationError("Invalid package ID"))
		case payment.ErrUserNotFound:
			respondError(c, apierrors.ErrUserNotFoundError)
		case payment.ErrCryptoPaymentDisabled:
			respondError(c, apierrors.NewInvalidRequestError("Cryptocurrency payment is currently disabled"))
		default:
			respondError(c, apierrors.ErrInternalServerError)
		}
		return
	}

	c.JSON(http.StatusOK, resp)
}

// handleStripeWebhook handles Stripe webhook events
func (s *APIServer) handleStripeWebhook(c *gin.Context) {
	if s.paymentService == nil {
		respondError(c, apierrors.NewInvalidRequestError("Payment service not available"))
		return
	}

	// Read the request body
	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		respondError(c, apierrors.NewValidationError("Failed to read request body"))
		return
	}

	// Get the Stripe signature header
	signature := c.GetHeader("Stripe-Signature")

	// Process the webhook
	err = s.paymentService.HandleStripeWebhook(c.Request.Context(), payload, signature)
	if err != nil {
		if err == payment.ErrInvalidWebhookSig {
			respondError(c, apierrors.NewValidationError("Invalid webhook signature"))
			return
		}
		// Log the error but return 200 to prevent Stripe from retrying
		c.JSON(http.StatusOK, gin.H{"received": true, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"received": true})
}

// handleCoinbaseWebhook handles Coinbase Commerce webhook events
func (s *APIServer) handleCoinbaseWebhook(c *gin.Context) {
	if s.paymentService == nil {
		respondError(c, apierrors.NewInvalidRequestError("Payment service not available"))
		return
	}

	// Read the request body
	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		respondError(c, apierrors.NewValidationError("Failed to read request body"))
		return
	}

	// Get the Coinbase signature header
	signature := c.GetHeader("X-CC-Webhook-Signature")

	// Process the webhook
	err = s.paymentService.HandleCoinbaseWebhook(c.Request.Context(), payload, signature)
	if err != nil {
		if err == payment.ErrInvalidWebhookSig {
			respondError(c, apierrors.NewValidationError("Invalid webhook signature"))
			return
		}
		// Log the error but return 200 to prevent Coinbase from retrying
		c.JSON(http.StatusOK, gin.H{"received": true, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"received": true})
}

// handlePaymentHistory handles retrieving payment history
func (s *APIServer) handlePaymentHistory(c *gin.Context) {
	if s.paymentService == nil {
		respondError(c, apierrors.NewInvalidRequestError("Payment service not available"))
		return
	}

	// Get user ID from context
	userIDStr := middleware.GetUserIDFromContext(c)
	if userIDStr == "" {
		respondError(c, apierrors.ErrInvalidCredentialsError)
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		respondError(c, apierrors.ErrInvalidCredentialsError)
		return
	}

	// Parse pagination params
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	// Get payment history
	resp, err := s.paymentService.GetPaymentHistory(c.Request.Context(), userID, page, pageSize)
	if err != nil {
		respondError(c, apierrors.ErrInternalServerError)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// handleGetPackages handles retrieving available quota packages
func (s *APIServer) handleGetPackages(c *gin.Context) {
	if s.paymentService == nil {
		respondError(c, apierrors.NewInvalidRequestError("Payment service not available"))
		return
	}

	packages := s.paymentService.GetPackages()
	c.JSON(http.StatusOK, gin.H{
		"packages":       packages,
		"crypto_enabled": s.paymentService.IsCryptoPaymentEnabled(),
	})
}

// handleGetQuota handles retrieving user's current quota
func (s *APIServer) handleGetQuota(c *gin.Context) {
	if s.paymentService == nil {
		respondError(c, apierrors.NewInvalidRequestError("Payment service not available"))
		return
	}

	// Get user ID from context
	userIDStr := middleware.GetUserIDFromContext(c)
	if userIDStr == "" {
		respondError(c, apierrors.ErrInvalidCredentialsError)
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		respondError(c, apierrors.ErrInvalidCredentialsError)
		return
	}

	// Get quota info
	resp, err := s.paymentService.GetUserQuota(c.Request.Context(), userID)
	if err != nil {
		if err == payment.ErrUserNotFound {
			respondError(c, apierrors.ErrUserNotFoundError)
			return
		}
		respondError(c, apierrors.ErrInternalServerError)
		return
	}

	c.JSON(http.StatusOK, resp)
}
func (s *APIServer) handleSubmitReview(c *gin.Context)    { c.JSON(501, gin.H{"error": "not implemented"}) }
func (s *APIServer) handleGetReviews(c *gin.Context)      { c.JSON(501, gin.H{"error": "not implemented"}) }
func (s *APIServer) handleListWebhooks(c *gin.Context)    { c.JSON(501, gin.H{"error": "not implemented"}) }
func (s *APIServer) handleCreateWebhook(c *gin.Context)   { c.JSON(501, gin.H{"error": "not implemented"}) }
func (s *APIServer) handleDeleteWebhook(c *gin.Context)   { c.JSON(501, gin.H{"error": "not implemented"}) }

// Admin handlers (placeholders)
func (s *APIServer) handleAdminStats(c *gin.Context)            { c.JSON(501, gin.H{"error": "not implemented"}) }
func (s *APIServer) handleAdminListUsers(c *gin.Context)        { c.JSON(501, gin.H{"error": "not implemented"}) }
func (s *APIServer) handleAdminGetUser(c *gin.Context)          { c.JSON(501, gin.H{"error": "not implemented"}) }
func (s *APIServer) handleAdminUpdateUserStatus(c *gin.Context) { c.JSON(501, gin.H{"error": "not implemented"}) }
func (s *APIServer) handleAdminModerationQueue(c *gin.Context)  { c.JSON(501, gin.H{"error": "not implemented"}) }
func (s *APIServer) handleAdminApproveReview(c *gin.Context)    { c.JSON(501, gin.H{"error": "not implemented"}) }
func (s *APIServer) handleAdminRejectReview(c *gin.Context)     { c.JSON(501, gin.H{"error": "not implemented"}) }


// ============================================
// Trial Handlers
// ============================================

// SetAgentTrialRequest represents a request to set agent trial status
type SetAgentTrialRequest struct {
	TrialEnabled bool `json:"trial_enabled"`
}

// handleSetAgentTrial handles setting trial enabled/disabled for an agent
// D5.4: THE Creator SHALL have option to disable trial for their Agents
func (s *APIServer) handleSetAgentTrial(c *gin.Context) {
	if s.agentService == nil {
		respondError(c, apierrors.NewInvalidRequestError("Agent service not available"))
		return
	}

	// Get user ID from context
	userIDStr := middleware.GetUserIDFromContext(c)
	if userIDStr == "" {
		respondError(c, apierrors.ErrInvalidCredentialsError)
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		respondError(c, apierrors.ErrInvalidCredentialsError)
		return
	}

	// Parse agent ID
	agentIDStr := c.Param("id")
	agentID, err := uuid.Parse(agentIDStr)
	if err != nil {
		respondError(c, apierrors.NewValidationError("Invalid agent ID"))
		return
	}

	// Parse request body
	var req SetAgentTrialRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apierrors.NewValidationError(err.Error()))
		return
	}

	// Set trial enabled status
	resp, err := s.agentService.SetTrialEnabled(c.Request.Context(), agentID, userID, req.TrialEnabled)
	if err != nil {
		switch err {
		case agent.ErrAgentNotFound:
			respondError(c, apierrors.ErrAgentNotFoundError)
		case agent.ErrAgentNotOwned:
			respondError(c, &apierrors.APIError{
				Code:       apierrors.ErrAgentNotOwned,
				Message:    "You do not own this agent",
				HTTPStatus: http.StatusForbidden,
			})
		default:
			respondError(c, apierrors.ErrInternalServerError)
		}
		return
	}

	c.JSON(http.StatusOK, resp)
}

// handleGetTrialInfo handles getting trial info for a specific agent
// D5.2: WHEN a developer uses trial calls, THE System SHALL clearly indicate remaining trial quota
func (s *APIServer) handleGetTrialInfo(c *gin.Context) {
	if s.trialService == nil {
		respondError(c, apierrors.NewInvalidRequestError("Trial service not available"))
		return
	}

	// Get user ID from context
	userIDStr := middleware.GetUserIDFromContext(c)
	if userIDStr == "" {
		respondError(c, apierrors.ErrInvalidCredentialsError)
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		respondError(c, apierrors.ErrInvalidCredentialsError)
		return
	}

	// Parse agent ID
	agentIDStr := c.Param("id")
	agentID, err := uuid.Parse(agentIDStr)
	if err != nil {
		respondError(c, apierrors.NewValidationError("Invalid agent ID"))
		return
	}

	// Get trial info
	info, err := s.trialService.GetTrialInfo(c.Request.Context(), userID, agentID)
	if err != nil {
		switch err {
		case trial.ErrAgentNotFound:
			respondError(c, apierrors.ErrAgentNotFoundError)
		default:
			respondError(c, apierrors.ErrInternalServerError)
		}
		return
	}

	c.JSON(http.StatusOK, info)
}

// handleGetUserTrialUsage handles getting all trial usage for a user
func (s *APIServer) handleGetUserTrialUsage(c *gin.Context) {
	if s.trialService == nil {
		respondError(c, apierrors.NewInvalidRequestError("Trial service not available"))
		return
	}

	// Get user ID from context
	userIDStr := middleware.GetUserIDFromContext(c)
	if userIDStr == "" {
		respondError(c, apierrors.ErrInvalidCredentialsError)
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		respondError(c, apierrors.ErrInvalidCredentialsError)
		return
	}

	// Get trial usage with agent names
	usage, err := s.trialService.GetUserTrialUsageWithAgentNames(c.Request.Context(), userID)
	if err != nil {
		respondError(c, apierrors.ErrInternalServerError)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"trial_usage":         usage,
		"trial_calls_per_agent": s.trialService.GetTrialCallsPerAgent(),
	})
}


// ============================================
// Withdrawal Handlers
// ============================================

// handleGetEarnings handles getting creator earnings information
func (s *APIServer) handleGetEarnings(c *gin.Context) {
	if s.withdrawalService == nil {
		respondError(c, apierrors.NewInvalidRequestError("Withdrawal service not available"))
		return
	}

	// Get user ID from context
	userIDStr := middleware.GetUserIDFromContext(c)
	if userIDStr == "" {
		respondError(c, apierrors.ErrInvalidCredentialsError)
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		respondError(c, apierrors.ErrInvalidCredentialsError)
		return
	}

	// Get earnings info
	info, err := s.withdrawalService.GetEarningsInfo(c.Request.Context(), userID)
	if err != nil {
		switch err {
		case withdrawal.ErrCreatorNotFound:
			respondError(c, apierrors.ErrUserNotFoundError)
		case withdrawal.ErrNotCreator:
			respondError(c, apierrors.NewInvalidRequestError("Only creators can view earnings"))
		default:
			respondError(c, apierrors.ErrInternalServerError)
		}
		return
	}

	c.JSON(http.StatusOK, info)
}

// handleCreateWithdrawal handles creating a new withdrawal request
func (s *APIServer) handleCreateWithdrawal(c *gin.Context) {
	if s.withdrawalService == nil {
		respondError(c, apierrors.NewInvalidRequestError("Withdrawal service not available"))
		return
	}

	// Get user ID from context
	userIDStr := middleware.GetUserIDFromContext(c)
	if userIDStr == "" {
		respondError(c, apierrors.ErrInvalidCredentialsError)
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		respondError(c, apierrors.ErrInvalidCredentialsError)
		return
	}

	// Parse request body
	var req withdrawal.CreateWithdrawalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apierrors.NewValidationError(err.Error()))
		return
	}

	// Create withdrawal
	resp, err := s.withdrawalService.CreateWithdrawal(c.Request.Context(), userID, &req)
	if err != nil {
		switch err {
		case withdrawal.ErrInsufficientBalance:
			respondError(c, apierrors.NewInvalidRequestError("Insufficient balance for withdrawal"))
		case withdrawal.ErrBelowMinimumThreshold:
			config := s.withdrawalService.GetConfig()
			respondError(c, apierrors.NewValidationError(fmt.Sprintf("Withdrawal amount must be at least $%s", config.MinimumAmount.String())))
		case withdrawal.ErrCreatorNotFound:
			respondError(c, apierrors.ErrUserNotFoundError)
		case withdrawal.ErrNotCreator:
			respondError(c, apierrors.NewInvalidRequestError("Only creators can request withdrawals"))
		case withdrawal.ErrNoWalletAddress:
			respondError(c, apierrors.NewValidationError("No wallet address configured for crypto withdrawal"))
		case withdrawal.ErrInvalidWithdrawalMethod:
			respondError(c, apierrors.NewValidationError("Invalid withdrawal method"))
		default:
			respondError(c, apierrors.ErrInternalServerError)
		}
		return
	}

	c.JSON(http.StatusCreated, resp)
}

// handleGetWithdrawalHistory handles getting withdrawal history for a creator
func (s *APIServer) handleGetWithdrawalHistory(c *gin.Context) {
	if s.withdrawalService == nil {
		respondError(c, apierrors.NewInvalidRequestError("Withdrawal service not available"))
		return
	}

	// Get user ID from context
	userIDStr := middleware.GetUserIDFromContext(c)
	if userIDStr == "" {
		respondError(c, apierrors.ErrInvalidCredentialsError)
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		respondError(c, apierrors.ErrInvalidCredentialsError)
		return
	}

	// Parse pagination params
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	// Get withdrawal history
	resp, err := s.withdrawalService.GetWithdrawalHistory(c.Request.Context(), userID, page, pageSize)
	if err != nil {
		respondError(c, apierrors.ErrInternalServerError)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// handleGetWithdrawal handles getting a specific withdrawal
func (s *APIServer) handleGetWithdrawal(c *gin.Context) {
	if s.withdrawalService == nil {
		respondError(c, apierrors.NewInvalidRequestError("Withdrawal service not available"))
		return
	}

	// Get user ID from context
	userIDStr := middleware.GetUserIDFromContext(c)
	if userIDStr == "" {
		respondError(c, apierrors.ErrInvalidCredentialsError)
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		respondError(c, apierrors.ErrInvalidCredentialsError)
		return
	}

	// Parse withdrawal ID
	withdrawalIDStr := c.Param("id")
	withdrawalID, err := uuid.Parse(withdrawalIDStr)
	if err != nil {
		respondError(c, apierrors.NewValidationError("Invalid withdrawal ID"))
		return
	}

	// Get withdrawal
	w, err := s.withdrawalService.GetWithdrawalByID(c.Request.Context(), withdrawalID)
	if err != nil {
		if err == withdrawal.ErrWithdrawalNotFound {
			respondError(c, &apierrors.APIError{
				Code:       apierrors.ErrInvalidRequest,
				Message:    "Withdrawal not found",
				HTTPStatus: http.StatusNotFound,
			})
			return
		}
		respondError(c, apierrors.ErrInternalServerError)
		return
	}

	// Verify ownership
	if w.CreatorID != userID {
		respondError(c, apierrors.ErrForbiddenError)
		return
	}

	c.JSON(http.StatusOK, w)
}

// handleGetWithdrawalConfig handles getting withdrawal configuration
func (s *APIServer) handleGetWithdrawalConfig(c *gin.Context) {
	if s.withdrawalService == nil {
		respondError(c, apierrors.NewInvalidRequestError("Withdrawal service not available"))
		return
	}

	config := s.withdrawalService.GetConfig()
	c.JSON(http.StatusOK, config)
}

// ============================================
// Admin Withdrawal Handlers
// ============================================

// handleAdminGetPendingWithdrawals handles getting all pending withdrawals (admin)
func (s *APIServer) handleAdminGetPendingWithdrawals(c *gin.Context) {
	if s.withdrawalService == nil {
		respondError(c, apierrors.NewInvalidRequestError("Withdrawal service not available"))
		return
	}

	// Parse pagination params
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	// Get pending withdrawals
	resp, err := s.withdrawalService.GetPendingWithdrawals(c.Request.Context(), page, pageSize)
	if err != nil {
		respondError(c, apierrors.ErrInternalServerError)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// handleAdminProcessWithdrawal handles marking a withdrawal as processing (admin)
func (s *APIServer) handleAdminProcessWithdrawal(c *gin.Context) {
	if s.withdrawalService == nil {
		respondError(c, apierrors.NewInvalidRequestError("Withdrawal service not available"))
		return
	}

	// Parse withdrawal ID
	withdrawalIDStr := c.Param("id")
	withdrawalID, err := uuid.Parse(withdrawalIDStr)
	if err != nil {
		respondError(c, apierrors.NewValidationError("Invalid withdrawal ID"))
		return
	}

	// Set withdrawal to processing
	err = s.withdrawalService.SetWithdrawalProcessing(c.Request.Context(), withdrawalID)
	if err != nil {
		switch err {
		case withdrawal.ErrWithdrawalNotFound:
			respondError(c, &apierrors.APIError{
				Code:       apierrors.ErrInvalidRequest,
				Message:    "Withdrawal not found",
				HTTPStatus: http.StatusNotFound,
			})
		case withdrawal.ErrWithdrawalNotPending:
			respondError(c, apierrors.NewInvalidRequestError("Withdrawal is not in pending status"))
		default:
			respondError(c, apierrors.ErrInternalServerError)
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Withdrawal marked as processing"})
}

// AdminCompleteWithdrawalRequest represents a request to complete a withdrawal
type AdminCompleteWithdrawalRequest struct {
	ExternalTxID string `json:"external_tx_id" binding:"required"`
}

// handleAdminCompleteWithdrawal handles completing a withdrawal (admin)
func (s *APIServer) handleAdminCompleteWithdrawal(c *gin.Context) {
	if s.withdrawalService == nil {
		respondError(c, apierrors.NewInvalidRequestError("Withdrawal service not available"))
		return
	}

	// Parse withdrawal ID
	withdrawalIDStr := c.Param("id")
	withdrawalID, err := uuid.Parse(withdrawalIDStr)
	if err != nil {
		respondError(c, apierrors.NewValidationError("Invalid withdrawal ID"))
		return
	}

	// Parse request body
	var req AdminCompleteWithdrawalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apierrors.NewValidationError(err.Error()))
		return
	}

	// Complete withdrawal
	err = s.withdrawalService.CompleteWithdrawal(c.Request.Context(), withdrawalID, req.ExternalTxID)
	if err != nil {
		switch err {
		case withdrawal.ErrWithdrawalNotFound:
			respondError(c, &apierrors.APIError{
				Code:       apierrors.ErrInvalidRequest,
				Message:    "Withdrawal not found",
				HTTPStatus: http.StatusNotFound,
			})
		case withdrawal.ErrWithdrawalAlreadyDone:
			respondError(c, apierrors.NewInvalidRequestError("Withdrawal already completed or failed"))
		default:
			respondError(c, apierrors.ErrInternalServerError)
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Withdrawal completed successfully"})
}

// AdminFailWithdrawalRequest represents a request to fail a withdrawal
type AdminFailWithdrawalRequest struct {
	Reason string `json:"reason" binding:"required"`
}

// handleAdminFailWithdrawal handles failing a withdrawal and restoring funds (admin)
func (s *APIServer) handleAdminFailWithdrawal(c *gin.Context) {
	if s.withdrawalService == nil {
		respondError(c, apierrors.NewInvalidRequestError("Withdrawal service not available"))
		return
	}

	// Parse withdrawal ID
	withdrawalIDStr := c.Param("id")
	withdrawalID, err := uuid.Parse(withdrawalIDStr)
	if err != nil {
		respondError(c, apierrors.NewValidationError("Invalid withdrawal ID"))
		return
	}

	// Parse request body
	var req AdminFailWithdrawalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apierrors.NewValidationError(err.Error()))
		return
	}

	// Fail withdrawal (this also restores funds)
	err = s.withdrawalService.FailWithdrawal(c.Request.Context(), withdrawalID, req.Reason)
	if err != nil {
		switch err {
		case withdrawal.ErrWithdrawalNotFound:
			respondError(c, &apierrors.APIError{
				Code:       apierrors.ErrInvalidRequest,
				Message:    "Withdrawal not found",
				HTTPStatus: http.StatusNotFound,
			})
		case withdrawal.ErrWithdrawalAlreadyDone:
			respondError(c, apierrors.NewInvalidRequestError("Withdrawal already completed or failed"))
		default:
			respondError(c, apierrors.ErrInternalServerError)
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Withdrawal failed and funds restored"})
}
