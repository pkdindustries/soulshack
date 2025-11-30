package llm

import (
	"github.com/alexschlessinger/pollytool/llm"
	"github.com/alexschlessinger/pollytool/messages"

	"pkdindustries/soulshack/internal/config"
	"pkdindustries/soulshack/internal/irc"
)

// PollyLLM wraps pollytool's MultiPass to implement soulshack's LLM interface
type PollyLLM struct {
	client          *llm.MultiPass
	streamProcessor *messages.StreamProcessor
}

// NewPollyLLM creates a new pollytool-based LLM client
func NewPollyLLM(config config.APIConfig) *PollyLLM {
	// Map soulshack's API keys to pollytool's expected format
	apiKeys := map[string]string{
		"openai":    config.OpenAIKey,
		"anthropic": config.AnthropicKey,
		"gemini":    config.GeminiKey,
		"ollama":    config.OllamaKey,
	}

	return &PollyLLM{
		client:          llm.NewMultiPass(apiKeys),
		streamProcessor: messages.NewStreamProcessor(),
	}
}

// ChatCompletionStream returns a single byte channel with chunked output for IRC
func (p *PollyLLM) ChatCompletionStream(req *CompletionRequest, chatCtx irc.ChatContextInterface) <-chan []byte {
	// Set base URL for ollama if provided
	config := chatCtx.GetConfig()
	if config != nil && config.API != nil && config.API.OllamaURL != "" {
		req.BaseURL = config.API.OllamaURL
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
		processor := irc.NewIRCEventProcessor(chatCtx, byteChan, maxChunkSize, registry, p.client, p.streamProcessor)
		processor.SetRequest(req)

		// Get event stream from LLM
		eventChan := p.client.ChatCompletionStream(chatCtx, req, p.streamProcessor)

		// Process events using the standardized processor
		response := messages.ProcessEventStream(chatCtx, eventChan, processor)

		// Flush any remaining buffer content
		processor.FlushBuffer()

		// Handle tool continuation if needed
		if len(response.ToolCalls) > 0 {
			processor.HandleToolContinuation(chatCtx, req)
		}
	}()

	return byteChan
}
