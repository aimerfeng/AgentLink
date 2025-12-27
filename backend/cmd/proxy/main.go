package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aimerfeng/AgentLink/internal/config"
	"github.com/aimerfeng/AgentLink/internal/logging"
	"github.com/aimerfeng/AgentLink/internal/monitoring"
	"github.com/aimerfeng/AgentLink/internal/server"
	"github.com/rs/zerolog/log"
)

func main() {
	// Load configuration first
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize structured logging
	logging.Setup(&cfg.Logging, cfg.Server.Env)

	log.Info().
		Str("env", cfg.Server.Env).
		Msg("Starting AgentLink Proxy Gateway")

	// Initialize Prometheus metrics
	monitoring.Init()
	log.Info().Msg("Prometheus metrics initialized")

	// Create and start proxy server
	srv := server.NewProxyServer(cfg)

	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Proxy.Port),
		Handler:      srv.Router(),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second, // Longer for streaming responses
		IdleTimeout:  120 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Info().
			Int("port", cfg.Proxy.Port).
			Str("url", cfg.Proxy.URL).
			Msg("Proxy Gateway listening")
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("Failed to start proxy server")
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit

	log.Info().
		Str("signal", sig.String()).
		Msg("Shutdown signal received, gracefully shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("Proxy server forced to shutdown")
	}

	log.Info().Msg("Proxy server exited gracefully")
}
