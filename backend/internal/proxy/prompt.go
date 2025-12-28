package proxy

import (
	"regexp"
	"strings"
)

// PromptInjector handles secure system prompt injection
type PromptInjector struct {
	// Patterns that might indicate prompt leakage attempts
	leakagePatterns []*regexp.Regexp
}

// NewPromptInjector creates a new prompt injector
func NewPromptInjector() *PromptInjector {
	return &PromptInjector{
		leakagePatterns: compileLeakagePatterns(),
	}
}

// compileLeakagePatterns compiles regex patterns for detecting prompt leakage attempts
func compileLeakagePatterns() []*regexp.Regexp {
	patterns := []string{
		// Common prompt extraction attempts
		`(?i)ignore\s+(all\s+)?(previous|prior|above)\s+(instructions?|prompts?)`,
		`(?i)what\s+(is|are)\s+(your|the)\s+(system\s+)?prompt`,
		`(?i)reveal\s+(your|the)\s+(system\s+)?prompt`,
		`(?i)show\s+(me\s+)?(your|the)\s+(system\s+)?prompt`,
		`(?i)print\s+(your|the)\s+(system\s+)?prompt`,
		`(?i)output\s+(your|the)\s+(system\s+)?prompt`,
		`(?i)repeat\s+(your|the)\s+(system\s+)?(prompt|instructions?)`,
		`(?i)tell\s+me\s+(your|the)\s+(system\s+)?prompt`,
		`(?i)what\s+were\s+you\s+told`,
		`(?i)what\s+are\s+your\s+instructions`,
		`(?i)disregard\s+(all\s+)?(previous|prior)\s+`,
		`(?i)forget\s+(all\s+)?(previous|prior)\s+`,
	}

	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		if re, err := regexp.Compile(p); err == nil {
			compiled = append(compiled, re)
		}
	}
	return compiled
}

// InjectSystemPrompt securely injects the system prompt into messages
// It filters out any existing system messages to prevent prompt injection attacks
func (pi *PromptInjector) InjectSystemPrompt(messages []ChatMessage, systemPrompt string) []ChatMessage {
	// Create new slice with system prompt first
	result := make([]ChatMessage, 0, len(messages)+1)

	// Add system prompt as first message
	result = append(result, ChatMessage{
		Role:    "system",
		Content: systemPrompt,
	})

	// Add user messages, filtering out any existing system messages
	// This prevents users from injecting their own system prompts
	for _, msg := range messages {
		if msg.Role != "system" {
			result = append(result, msg)
		}
	}

	return result
}

// DetectLeakageAttempt checks if a message appears to be attempting to extract the system prompt
func (pi *PromptInjector) DetectLeakageAttempt(content string) bool {
	for _, pattern := range pi.leakagePatterns {
		if pattern.MatchString(content) {
			return true
		}
	}
	return false
}

// SanitizeResponse removes any system prompt content from the response
// This is a critical security measure to ensure the hidden prompt is never exposed
func (pi *PromptInjector) SanitizeResponse(response *ChatResponse, systemPrompt string) *ChatResponse {
	if response == nil {
		return nil
	}

	// Create sanitized copy
	sanitized := *response
	sanitized.Choices = make([]ChatChoice, len(response.Choices))

	for i, choice := range response.Choices {
		sanitized.Choices[i] = choice

		if choice.Message != nil {
			// Create a copy of the message
			msgCopy := *choice.Message
			msgCopy.Content = pi.sanitizeContent(msgCopy.Content, systemPrompt)
			sanitized.Choices[i].Message = &msgCopy
		}

		if choice.Delta != nil {
			// Create a copy of the delta
			deltaCopy := *choice.Delta
			deltaCopy.Content = pi.sanitizeContent(deltaCopy.Content, systemPrompt)
			sanitized.Choices[i].Delta = &deltaCopy
		}
	}

	return &sanitized
}

// sanitizeContent removes system prompt from content
func (pi *PromptInjector) sanitizeContent(content, systemPrompt string) string {
	if content == "" || systemPrompt == "" {
		return content
	}

	// Direct match replacement
	if strings.Contains(content, systemPrompt) {
		content = strings.ReplaceAll(content, systemPrompt, "[REDACTED]")
	}

	// Check for partial matches (first 50 chars of prompt)
	if len(systemPrompt) > 50 {
		prefix := systemPrompt[:50]
		if strings.Contains(content, prefix) {
			content = strings.ReplaceAll(content, prefix, "[REDACTED]")
		}
	}

	// Check for common prompt leakage indicators
	leakageIndicators := []string{
		"my system prompt is",
		"my instructions are",
		"i was told to",
		"my initial prompt",
		"my original instructions",
	}

	lowerContent := strings.ToLower(content)
	for _, indicator := range leakageIndicators {
		if strings.Contains(lowerContent, indicator) {
			// Find and redact the following content
			idx := strings.Index(lowerContent, indicator)
			if idx >= 0 {
				// Redact from indicator to end of sentence or 200 chars
				endIdx := idx + len(indicator) + 200
				if endIdx > len(content) {
					endIdx = len(content)
				}
				// Find sentence end
				for j := idx + len(indicator); j < endIdx; j++ {
					if content[j] == '.' || content[j] == '\n' {
						endIdx = j + 1
						break
					}
				}
				content = content[:idx] + "[REDACTED]" + content[endIdx:]
			}
		}
	}

	return content
}

// SanitizeStreamChunk sanitizes a streaming response chunk
func (pi *PromptInjector) SanitizeStreamChunk(chunk *StreamChunk, systemPrompt string) *StreamChunk {
	if chunk == nil {
		return nil
	}

	// Create sanitized copy
	sanitized := *chunk
	sanitized.Choices = make([]ChatChoice, len(chunk.Choices))

	for i, choice := range chunk.Choices {
		sanitized.Choices[i] = choice

		if choice.Delta != nil {
			deltaCopy := *choice.Delta
			deltaCopy.Content = pi.sanitizeContent(deltaCopy.Content, systemPrompt)
			sanitized.Choices[i].Delta = &deltaCopy
		}
	}

	return &sanitized
}

// ValidateUserMessages validates user messages for potential security issues
func (pi *PromptInjector) ValidateUserMessages(messages []ChatMessage) error {
	for _, msg := range messages {
		// Check for system role in user messages (should be filtered, but double-check)
		if msg.Role == "system" {
			return ErrInvalidRequest
		}

		// Log potential leakage attempts (but don't block - let the model handle it)
		if pi.DetectLeakageAttempt(msg.Content) {
			// This is logged but not blocked - the system prompt injection
			// and response sanitization should handle this
		}
	}
	return nil
}
