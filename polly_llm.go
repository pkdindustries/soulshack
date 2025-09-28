package main

import (
	"context"

	"github.com/alexschlessinger/pollytool/llm"
	"github.com/alexschlessinger/pollytool/messages"
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

	// Set thinking effort if enabled
	if req.Thinking {
		pollyReq.ThinkingEffort = "medium" // Default to medium effort when enabled
	}

	// Set base URL for ollama if provided
	config := chatCtx.GetConfig()
	if config != nil && config.API != nil && config.API.OllamaURL != "" {
		pollyReq.BaseURL = config.API.OllamaURL
	}

	// Get the system for registry access
	sys := chatCtx.GetSystem()
	registry := sys.GetToolRegistry()

	// Create processor with IRC context and chunking
	maxChunkSize := 400 // Default IRC chunk size
	if config.Session.ChunkMax > 0 {
		maxChunkSize = config.Session.ChunkMax
	}

	// Create byte channel for IRC output
	byteChan := make(chan []byte, 10)

	go func() {
		defer close(byteChan)

		// Create IRC processor with all necessary context
		processor := NewIRCEventProcessor(chatCtx, byteChan, maxChunkSize, registry, p.client)
		processor.SetRequest(pollyReq)

		// Get event stream from LLM
		eventChan := p.client.ChatCompletionStream(ctx, pollyReq, nil)

		// Process events using the standardized processor
		response := messages.ProcessEventStream(ctx, eventChan, processor)

		// Flush any remaining buffer content
		processor.flushBuffer()

		// Handle tool continuation if needed
		if len(response.ToolCalls) > 0 {
			processor.HandleToolContinuation(ctx, pollyReq)
		}
	}()

	return byteChan
}
