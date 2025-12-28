package proxy

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// StreamHandler handles SSE streaming responses
type StreamHandler struct {
	promptInjector *PromptInjector
}

// NewStreamHandler creates a new stream handler
func NewStreamHandler(injector *PromptInjector) *StreamHandler {
	return &StreamHandler{
		promptInjector: injector,
	}
}

// StreamConfig holds configuration for streaming
type StreamConfig struct {
	SystemPrompt    string
	FlushInterval   time.Duration
	MaxChunkSize    int
	SanitizeContent bool
}

// DefaultStreamConfig returns default streaming configuration
func DefaultStreamConfig(systemPrompt string) *StreamConfig {
	return &StreamConfig{
		SystemPrompt:    systemPrompt,
		FlushInterval:   10 * time.Millisecond,
		MaxChunkSize:    4096,
		SanitizeContent: true,
	}
}

// StreamResponse streams SSE events from upstream to the client
// It handles parsing, sanitization, and forwarding of events
func (sh *StreamHandler) StreamResponse(
	ctx context.Context,
	reader io.Reader,
	writer io.Writer,
	flusher http.Flusher,
	config *StreamConfig,
) (*StreamResult, error) {
	result := &StreamResult{
		ChunksProcessed: 0,
		TotalTokens:     0,
	}

	scanner := bufio.NewScanner(reader)
	// Increase buffer size for large chunks
	buf := make([]byte, config.MaxChunkSize)
	scanner.Buffer(buf, config.MaxChunkSize)

	for scanner.Scan() {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}

		line := scanner.Text()

		// Skip empty lines
		if line == "" {
			continue
		}

		// Check for data prefix
		if !strings.HasPrefix(line, "data: ") {
			// Forward non-data lines (like comments) as-is
			if strings.HasPrefix(line, ":") {
				fmt.Fprintf(writer, "%s\n", line)
				flusher.Flush()
			}
			continue
		}

		data := strings.TrimPrefix(line, "data: ")

		// Check for stream end
		if data == "[DONE]" {
			fmt.Fprintf(writer, "data: [DONE]\n\n")
			flusher.Flush()
			break
		}

		// Parse and process the chunk
		processedData, tokens, err := sh.processChunk(data, config)
		if err != nil {
			log.Warn().Err(err).Str("data", truncateString(data, 100)).Msg("Failed to process chunk")
			// Forward original data on parse error
			processedData = data
		}

		result.ChunksProcessed++
		result.TotalTokens += tokens

		// Write the processed chunk
		fmt.Fprintf(writer, "data: %s\n\n", processedData)
		flusher.Flush()
	}

	if err := scanner.Err(); err != nil {
		return result, fmt.Errorf("error reading stream: %w", err)
	}

	return result, nil
}

// StreamResult holds the result of streaming
type StreamResult struct {
	ChunksProcessed int
	TotalTokens     int
	Error           error
}

// processChunk processes a single SSE chunk
func (sh *StreamHandler) processChunk(data string, config *StreamConfig) (string, int, error) {
	var chunk StreamChunk
	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		return data, 0, err
	}

	tokens := 0

	// Count tokens and sanitize content
	for i, choice := range chunk.Choices {
		if choice.Delta != nil && choice.Delta.Content != "" {
			// Rough token estimate (1 token ≈ 4 chars)
			tokens += len(choice.Delta.Content) / 4

			// Sanitize content if enabled
			if config.SanitizeContent && config.SystemPrompt != "" {
				sanitized := sh.promptInjector.SanitizeStreamChunk(&chunk, config.SystemPrompt)
				if sanitized != nil && len(sanitized.Choices) > i {
					chunk.Choices[i] = sanitized.Choices[i]
				}
			}
		}
	}

	// Re-serialize the chunk
	processed, err := json.Marshal(chunk)
	if err != nil {
		return data, tokens, err
	}

	return string(processed), tokens, nil
}

// StreamError sends an error event to the client
func (sh *StreamHandler) StreamError(writer io.Writer, flusher http.Flusher, errCode, errMsg string) {
	errorEvent := map[string]interface{}{
		"error": map[string]string{
			"code":    errCode,
			"message": errMsg,
		},
	}

	data, _ := json.Marshal(errorEvent)
	fmt.Fprintf(writer, "data: %s\n\n", data)
	fmt.Fprintf(writer, "data: [DONE]\n\n")
	flusher.Flush()
}

// SetupSSEHeaders sets the required headers for SSE streaming
func SetupSSEHeaders(w http.ResponseWriter, requestID string) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering
	w.Header().Set("X-Request-ID", requestID)
}

// truncateString truncates a string to maxLen characters
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
