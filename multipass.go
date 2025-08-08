package main

import (
	"context"
	"fmt"
	"strings"

	ai "github.com/sashabaranov/go-openai"
)

type MultiPass struct {
	config APIConfig
}

func NewMultiPass(config APIConfig) *MultiPass {
	return &MultiPass{
		config: config,
	}
}

func (m *MultiPass) ChatCompletionTask(ctx context.Context, req *CompletionRequest, chunker *Chunker) (<-chan []byte, <-chan *ai.ToolCall, <-chan *ai.ChatCompletionMessage) {
	// Parse the model string to extract provider and actual model name
	parts := strings.SplitN(req.Model, "/", 2)
	if len(parts) != 2 {
		// Return error through the channel
		errorChan := make(chan ai.ChatCompletionMessage, 1)
		errorChan <- ai.ChatCompletionMessage{
			Role:    ai.ChatMessageRoleAssistant,
			Content: fmt.Sprintf("Error: model must include provider prefix (e.g., 'openai/gpt-4o', 'ollama/llama3.2'). Got: %s", req.Model),
		}
		close(errorChan)
		return chunker.ProcessMessages(errorChan)
	}

	provider := parts[0]
	actualModel := parts[1]

	// Update the request with the actual model name (without prefix)
	req.Model = actualModel

	// Route to the appropriate provider
	var llm LLM
	switch provider {
	case "openai":
		llm = NewOpenAIClient(m.config)
	case "anthropic":
		llm = NewAnthropicClient(m.config)
	case "gemini":
		llm = NewGeminiClient(m.config)
	case "ollama", "local":
		llm = NewOllamaClient(m.config)
	default:
		// Return error through the channel
		errorChan := make(chan ai.ChatCompletionMessage, 1)
		errorChan <- ai.ChatCompletionMessage{
			Role:    ai.ChatMessageRoleAssistant,
			Content: fmt.Sprintf("Error: unknown provider '%s'. Valid providers: openai, anthropic, gemini, ollama", provider),
		}
		close(errorChan)
		return chunker.ProcessMessages(errorChan)
	}

	// Delegate to the selected provider
	return llm.ChatCompletionTask(ctx, req, chunker)
}