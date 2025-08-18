package main

import (
	"context"

	"github.com/alexschlessinger/pollytool/llm"
)

// PollyLLM wraps pollytool's MultiPass to implement soulshack's LLM interface
type PollyLLM struct {
	client *llm.MultiPass
}

// NewPollyLLM creates a new pollytool-based LLM client
func NewPollyLLM(config APIConfig) *PollyLLM {
	// Map soulshack's API keys to pollytool's expected format
	apiKeys := map[string]string{
		"openai":    config.OpenAIKey,
		"anthropic": config.AnthropicKey,
		"gemini":    config.GeminiKey,
		"ollama":    config.OllamaKey,
	}
	
	return &PollyLLM{
		client: llm.NewMultiPass(apiKeys),
	}
}

// ChatCompletionStream returns a single byte channel with chunked output for IRC
func (p *PollyLLM) ChatCompletionStream(ctx context.Context, req *CompletionRequest, chatCtx ChatContextInterface) <-chan []byte {
	// Convert soulshack request to pollytool request
	pollyReq := &llm.CompletionRequest{
		Model:       req.Model,
		Messages:    req.Session.GetHistory(),
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		Tools:       req.Tools,
		Timeout:     req.Timeout,
	}
	
	// Special handling for Ollama URLs
	if req.Model == "local" || req.Model == "ollama" {
		// Default to a common model if just "local" or "ollama" is specified
		pollyReq.Model = "ollama/llama3.2"
	}
	
	// Set base URL for ollama if provided
	if config, ok := req.Session.(*LocalSession); ok && config != nil {
		if config.config != nil && config.config.API != nil && config.config.API.OllamaURL != "" {
			pollyReq.BaseURL = config.config.API.OllamaURL
		}
	}

	// Get the system for registry access
	sys := chatCtx.GetSystem()
	registry := sys.GetToolRegistry()

	// Create processor with IRC context and chunking
	config := chatCtx.GetConfig()
	maxChunkSize := 400 // Default IRC chunk size
	if config.Session.ChunkMax > 0 {
		maxChunkSize = config.Session.ChunkMax
	}

	processor := NewSoulshackStreamProcessor(chatCtx, maxChunkSize, registry, p.client)

	// Process the completion stream and return byte channel
	return processor.ProcessCompletionStream(ctx, pollyReq)
}