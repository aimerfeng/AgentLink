package server

import (
	"net/http"

	"github.com/aimerfeng/AgentLink/internal/auth"
	"github.com/aimerfeng/AgentLink/internal/config"
	apierrors "github.com/aimerfeng/AgentLink/internal/errors"
	"github.com/aimerfeng/AgentLink/internal/logging"
	"github.com/aimerfeng/AgentLink/internal/middleware"
	"github.com/aimerfeng/AgentLink/internal/monitoring"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

// APIServer represents the main API server
type APIServer struct {
	config      *config.Config
	router      *gin.Engine
	db          *pgxpool.Pool
	authService *auth.Service
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

	srv := &APIServer{
		config:      cfg,
		router:      router,
		db:          db,
		authService: authService,
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

		// Creator routes (protected)
		creators := v1.Group("/creators")
		creators.Use(s.authMiddleware())
		{
			creators.GET("/me", s.handleGetCreator)
			creators.PUT("/me", s.handleUpdateCreator)
			creators.PUT("/me/wallet", s.handleBindWallet)
		}

		// Agent routes (protected)
		agents := v1.Group("/agents")
		agents.Use(s.authMiddleware())
		{
			agents.POST("/", s.handleCreateAgent)
			agents.GET("/", s.handleListAgents)
			agents.GET("/:id", s.handleGetAgent)
			agents.PUT("/:id", s.handleUpdateAgent)
			agents.POST("/:id/publish", s.handlePublishAgent)
			agents.POST("/:id/unpublish", s.handleUnpublishAgent)
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

		// Developer routes (protected)
		developers := v1.Group("/developers")
		developers.Use(s.authMiddleware())
		{
			developers.GET("/me", s.handleGetDeveloper)
			developers.GET("/keys", s.handleListAPIKeys)
			developers.POST("/keys", s.handleCreateAPIKey)
			developers.DELETE("/keys/:id", s.handleDeleteAPIKey)
			developers.GET("/usage", s.handleGetUsage)
		}

		// Payment routes (protected)
		payments := v1.Group("/payments")
		{
			payments.POST("/checkout", s.authMiddleware(), s.handleCheckout)
			payments.POST("/webhook/stripe", s.handleStripeWebhook)
			payments.POST("/webhook/coinbase", s.handleCoinbaseWebhook)
			payments.GET("/history", s.authMiddleware(), s.handlePaymentHistory)
		}

		// Review routes
		reviews := v1.Group("/reviews")
		{
			reviews.POST("/agents/:id", s.authMiddleware(), s.handleSubmitReview)
			reviews.GET("/agents/:id", s.handleGetReviews)
		}

		// Webhook routes (protected)
		webhooks := v1.Group("/webhooks")
		webhooks.Use(s.authMiddleware())
		{
			webhooks.GET("/", s.handleListWebhooks)
			webhooks.POST("/", s.handleCreateWebhook)
			webhooks.DELETE("/:id", s.handleDeleteWebhook)
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


// authMiddleware validates JWT tokens
func (s *APIServer) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			respondError(c, apierrors.ErrInvalidCredentialsError)
			c.Abort()
			return
		}

		// Extract token from "Bearer <token>"
		const bearerPrefix = "Bearer "
		if len(authHeader) < len(bearerPrefix) || authHeader[:len(bearerPrefix)] != bearerPrefix {
			respondError(c, apierrors.ErrInvalidCredentialsError)
			c.Abort()
			return
		}

		tokenString := authHeader[len(bearerPrefix):]

		// Validate token
		claims, err := s.authService.ValidateAccessToken(tokenString)
		if err != nil {
			if err == auth.ErrTokenExpired {
				respondError(c, apierrors.ErrTokenExpiredError)
			} else {
				respondError(c, apierrors.ErrInvalidCredentialsError)
			}
			c.Abort()
			return
		}

		// Set user info in context
		c.Set("user_id", claims.UserID)
		c.Set("user_type", claims.UserType)
		c.Set("email", claims.Email)

		c.Next()
	}
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

	c.JSON(err.HTTPStatus, apierrors.ErrorResponse{
		Error:     *err,
		RequestID: reqIDStr,
	})
}

// Placeholder handlers - to be implemented in subsequent tasks
func (s *APIServer) handleGetCreator(c *gin.Context)      { c.JSON(501, gin.H{"error": "not implemented"}) }
func (s *APIServer) handleUpdateCreator(c *gin.Context)   { c.JSON(501, gin.H{"error": "not implemented"}) }
func (s *APIServer) handleBindWallet(c *gin.Context)      { c.JSON(501, gin.H{"error": "not implemented"}) }
func (s *APIServer) handleCreateAgent(c *gin.Context)     { c.JSON(501, gin.H{"error": "not implemented"}) }
func (s *APIServer) handleListAgents(c *gin.Context)      { c.JSON(501, gin.H{"error": "not implemented"}) }
func (s *APIServer) handleGetAgent(c *gin.Context)        { c.JSON(501, gin.H{"error": "not implemented"}) }
func (s *APIServer) handleUpdateAgent(c *gin.Context)     { c.JSON(501, gin.H{"error": "not implemented"}) }
func (s *APIServer) handlePublishAgent(c *gin.Context)    { c.JSON(501, gin.H{"error": "not implemented"}) }
func (s *APIServer) handleUnpublishAgent(c *gin.Context)  { c.JSON(501, gin.H{"error": "not implemented"}) }
func (s *APIServer) handleUploadKnowledge(c *gin.Context) { c.JSON(501, gin.H{"error": "not implemented"}) }
func (s *APIServer) handleSearchAgents(c *gin.Context)    { c.JSON(501, gin.H{"error": "not implemented"}) }
func (s *APIServer) handleGetPublicAgent(c *gin.Context)  { c.JSON(501, gin.H{"error": "not implemented"}) }
func (s *APIServer) handleGetCategories(c *gin.Context)   { c.JSON(501, gin.H{"error": "not implemented"}) }
func (s *APIServer) handleGetFeatured(c *gin.Context)     { c.JSON(501, gin.H{"error": "not implemented"}) }
func (s *APIServer) handleGetDeveloper(c *gin.Context)    { c.JSON(501, gin.H{"error": "not implemented"}) }
func (s *APIServer) handleListAPIKeys(c *gin.Context)     { c.JSON(501, gin.H{"error": "not implemented"}) }
func (s *APIServer) handleCreateAPIKey(c *gin.Context)    { c.JSON(501, gin.H{"error": "not implemented"}) }
func (s *APIServer) handleDeleteAPIKey(c *gin.Context)    { c.JSON(501, gin.H{"error": "not implemented"}) }
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
