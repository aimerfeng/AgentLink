package logging

import (
	"io"
	"os"
	"time"

	"github.com/aimerfeng/AgentLink/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Setup initializes the global logger based on configuration
func Setup(cfg *config.LoggingConfig, env string) {
	// Set log level
	level, err := zerolog.ParseLevel(cfg.Level)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	// Configure time format
	zerolog.TimeFieldFormat = time.RFC3339Nano

	// Configure output based on format and environment
	var output io.Writer
	if cfg.Format == "json" || env == "production" {
		output = os.Stdout
	} else {
		// Pretty console output for development
		output = zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: "15:04:05",
			NoColor:    false,
		}
	}

	// Set global logger
	log.Logger = zerolog.New(output).
		With().
		Timestamp().
		Str("service", "agentlink").
		Logger()
}

// NewLogger creates a new logger with additional context
func NewLogger(component string) zerolog.Logger {
	return log.Logger.With().Str("component", component).Logger()
}

// RequestLogger is a Gin middleware for structured request logging
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		// Process request
		c.Next()

		// Calculate latency
		latency := time.Since(start)

		// Get request ID
		requestID, _ := c.Get("request_id")

		// Build log event
		event := log.Info()
		if c.Writer.Status() >= 500 {
			event = log.Error()
		} else if c.Writer.Status() >= 400 {
			event = log.Warn()
		}

		// Log request details
		event.
			Str("request_id", requestID.(string)).
			Str("method", c.Request.Method).
			Str("path", path).
			Str("query", raw).
			Int("status", c.Writer.Status()).
			Dur("latency", latency).
			Str("client_ip", c.ClientIP()).
			Str("user_agent", c.Request.UserAgent()).
			Int("body_size", c.Writer.Size()).
			Msg("HTTP request")
	}
}


// RequestLogEntry represents a structured log entry for requests
type RequestLogEntry struct {
	RequestID   string        `json:"request_id"`
	Method      string        `json:"method"`
	Path        string        `json:"path"`
	Query       string        `json:"query,omitempty"`
	StatusCode  int           `json:"status_code"`
	Latency     time.Duration `json:"latency_ms"`
	ClientIP    string        `json:"client_ip"`
	UserAgent   string        `json:"user_agent,omitempty"`
	UserID      string        `json:"user_id,omitempty"`
	AgentID     string        `json:"agent_id,omitempty"`
	Error       string        `json:"error,omitempty"`
	BodySize    int           `json:"body_size"`
}

// APICallLogEntry represents a structured log entry for API calls
type APICallLogEntry struct {
	RequestID    string        `json:"request_id"`
	AgentID      string        `json:"agent_id"`
	UserID       string        `json:"user_id"`
	APIKeyID     string        `json:"api_key_id"`
	InputTokens  int           `json:"input_tokens"`
	OutputTokens int           `json:"output_tokens"`
	Latency      time.Duration `json:"latency_ms"`
	Status       string        `json:"status"`
	ErrorCode    string        `json:"error_code,omitempty"`
	Provider     string        `json:"provider"`
	Model        string        `json:"model"`
}

// LogAPICall logs an API call with structured data
func LogAPICall(entry *APICallLogEntry) {
	event := log.Info()
	if entry.Status == "error" {
		event = log.Error()
	}

	event.
		Str("request_id", entry.RequestID).
		Str("agent_id", entry.AgentID).
		Str("user_id", entry.UserID).
		Str("api_key_id", entry.APIKeyID).
		Int("input_tokens", entry.InputTokens).
		Int("output_tokens", entry.OutputTokens).
		Dur("latency", entry.Latency).
		Str("status", entry.Status).
		Str("error_code", entry.ErrorCode).
		Str("provider", entry.Provider).
		Str("model", entry.Model).
		Msg("API call")
}

// LogPayment logs a payment event
func LogPayment(requestID, userID, paymentID, method, status string, amount float64) {
	log.Info().
		Str("request_id", requestID).
		Str("user_id", userID).
		Str("payment_id", paymentID).
		Str("method", method).
		Str("status", status).
		Float64("amount_usd", amount).
		Msg("Payment event")
}

// LogSettlement logs a settlement event
func LogSettlement(creatorID, txHash, status string, amount, fee float64) {
	log.Info().
		Str("creator_id", creatorID).
		Str("tx_hash", txHash).
		Str("status", status).
		Float64("amount", amount).
		Float64("fee", fee).
		Msg("Settlement event")
}

// LogSecurityEvent logs security-related events
func LogSecurityEvent(eventType, userID, clientIP, details string) {
	log.Warn().
		Str("event_type", eventType).
		Str("user_id", userID).
		Str("client_ip", clientIP).
		Str("details", details).
		Msg("Security event")
}

// LogError logs an error with context
func LogError(err error, requestID, component, operation string) {
	log.Error().
		Err(err).
		Str("request_id", requestID).
		Str("component", component).
		Str("operation", operation).
		Msg("Error occurred")
}

// SanitizeForLog removes sensitive data from strings for logging
func SanitizeForLog(data string, maxLen int) string {
	if len(data) > maxLen {
		return data[:maxLen] + "...[truncated]"
	}
	return data
}
