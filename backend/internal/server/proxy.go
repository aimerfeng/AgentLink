package server

import (
	"net/http"

	"github.com/aimerfeng/AgentLink/internal/config"
	"github.com/aimerfeng/AgentLink/internal/logging"
	"github.com/aimerfeng/AgentLink/internal/middleware"
	"github.com/aimerfeng/AgentLink/internal/monitoring"
	"github.com/gin-gonic/gin"
)

// ProxyServer represents the proxy gateway server
type ProxyServer struct {
	config *config.Config
	router *gin.Engine
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
// This is a placeholder - full implementation in Phase 3
func (s *ProxyServer) handleChat(c *gin.Context) {
	c.JSON(501, gin.H{"error": "not implemented"})
}
