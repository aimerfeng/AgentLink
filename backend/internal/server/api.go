package server

import (
	"fmt"
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
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// APIServer represents the main API server
type APIServer struct {
	config           *config.Config
	router           *gin.Engine
	db               *pgxpool.Pool
	authService      *auth.Service
	agentService     *agent.Service
	apiKeyService    *apikey.Service
	jwtAuthenticator *middleware.JWTAuthenticator
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

	// Create JWT authenticator for middleware
	jwtAuthenticator := middleware.NewJWTAuthenticator(&cfg.JWT)

	srv := &APIServer{
		config:           cfg,
		router:           router,
		db:               db,
		authService:      authService,
		agentService:     agentService,
		apiKeyService:    apiKeyService,
		jwtAuthenticator: jwtAuthenticator,
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
			payments.POST("/webhook/stripe", s.handleStripeWebhook)
			payments.POST("/webhook/coinbase", s.handleCoinbaseWebhook)
			payments.GET("/history", s.jwtAuthenticator.JWTAuth(), s.handlePaymentHistory)
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
func (s *APIServer) handleCheckout(c *gin.Context)        { c.JSON(501, gin.H{"error": "not implemented"}) }
func (s *APIServer) handleStripeWebhook(c *gin.Context)   { c.JSON(501, gin.H{"error": "not implemented"}) }
func (s *APIServer) handleCoinbaseWebhook(c *gin.Context) { c.JSON(501, gin.H{"error": "not implemented"}) }
func (s *APIServer) handlePaymentHistory(c *gin.Context)  { c.JSON(501, gin.H{"error": "not implemented"}) }
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
