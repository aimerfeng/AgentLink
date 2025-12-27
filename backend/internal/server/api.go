package server

import (
	"net/http"

	"github.com/aimerfeng/AgentLink/internal/config"
	"github.com/aimerfeng/AgentLink/internal/logging"
	"github.com/aimerfeng/AgentLink/internal/middleware"
	"github.com/aimerfeng/AgentLink/internal/monitoring"
	"github.com/gin-gonic/gin"
)

// APIServer represents the main API server
type APIServer struct {
	config *config.Config
	router *gin.Engine
}

// NewAPIServer creates a new API server instance
func NewAPIServer(cfg *config.Config) *APIServer {
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

	srv := &APIServer{
		config: cfg,
		router: router,
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
		// Auth routes
		auth := v1.Group("/auth")
		{
			auth.POST("/register", s.handleRegister)
			auth.POST("/login", s.handleLogin)
			auth.POST("/logout", s.handleLogout)
			auth.POST("/refresh", s.handleRefresh)
		}

		// Creator routes
		creators := v1.Group("/creators")
		{
			creators.GET("/me", s.handleGetCreator)
			creators.PUT("/me", s.handleUpdateCreator)
			creators.PUT("/me/wallet", s.handleBindWallet)
		}

		// Agent routes
		agents := v1.Group("/agents")
		{
			agents.POST("/", s.handleCreateAgent)
			agents.GET("/", s.handleListAgents)
			agents.GET("/:id", s.handleGetAgent)
			agents.PUT("/:id", s.handleUpdateAgent)
			agents.POST("/:id/publish", s.handlePublishAgent)
			agents.POST("/:id/unpublish", s.handleUnpublishAgent)
			agents.POST("/:id/knowledge", s.handleUploadKnowledge)
		}

		// Marketplace routes
		marketplace := v1.Group("/marketplace")
		{
			marketplace.GET("/agents", s.handleSearchAgents)
			marketplace.GET("/agents/:id", s.handleGetPublicAgent)
			marketplace.GET("/categories", s.handleGetCategories)
			marketplace.GET("/featured", s.handleGetFeatured)
		}

		// Developer routes
		developers := v1.Group("/developers")
		{
			developers.GET("/me", s.handleGetDeveloper)
			developers.GET("/keys", s.handleListAPIKeys)
			developers.POST("/keys", s.handleCreateAPIKey)
			developers.DELETE("/keys/:id", s.handleDeleteAPIKey)
			developers.GET("/usage", s.handleGetUsage)
		}

		// Payment routes
		payments := v1.Group("/payments")
		{
			payments.POST("/checkout", s.handleCheckout)
			payments.POST("/webhook/stripe", s.handleStripeWebhook)
			payments.POST("/webhook/coinbase", s.handleCoinbaseWebhook)
			payments.GET("/history", s.handlePaymentHistory)
		}

		// Review routes
		reviews := v1.Group("/reviews")
		{
			reviews.POST("/agents/:id", s.handleSubmitReview)
			reviews.GET("/agents/:id", s.handleGetReviews)
		}

		// Webhook routes
		webhooks := v1.Group("/webhooks")
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

// Placeholder handlers - to be implemented in subsequent tasks
func (s *APIServer) handleRegister(c *gin.Context)        { c.JSON(501, gin.H{"error": "not implemented"}) }
func (s *APIServer) handleLogin(c *gin.Context)           { c.JSON(501, gin.H{"error": "not implemented"}) }
func (s *APIServer) handleLogout(c *gin.Context)          { c.JSON(501, gin.H{"error": "not implemented"}) }
func (s *APIServer) handleRefresh(c *gin.Context)         { c.JSON(501, gin.H{"error": "not implemented"}) }
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
