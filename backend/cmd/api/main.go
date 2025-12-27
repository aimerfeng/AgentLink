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
	"github.com/aimerfeng/AgentLink/internal/database"
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
		Str("name", cfg.Server.Name).
		Msg("Starting AgentLink API server")

	// Initialize database connection
	db, err := database.New(cfg.Database.URL)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer db.Close()

	// Initialize Prometheus metrics
	monitoring.Init()
	log.Info().Msg("Prometheus metrics initialized")

	// Start metrics server if enabled
	if cfg.Monitoring.PrometheusEnabled {
		go startMetricsServer(cfg.Monitoring.PrometheusPort)
	}

	// Create and start server
	srv := server.NewAPIServer(cfg, db.Pool)

	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      srv.Router(),
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	// Start server in goroutine
	go func() {
		log.Info().
			Int("port", cfg.Server.Port).
			Str("url", cfg.Server.URL).
			Msg("API server listening")
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("Failed to start server")
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
		log.Error().Err(err).Msg("Server forced to shutdown")
	}

	log.Info().Msg("Server exited gracefully")
}

func startMetricsServer(port int) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", monitoring.Handler())

	metricsServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	log.Info().
		Int("port", port).
		Msg("Prometheus metrics server listening")

	if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Error().Err(err).Msg("Metrics server error")
	}
}
