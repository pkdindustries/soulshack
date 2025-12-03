package llm

import (
	"time"

	"github.com/alexschlessinger/pollytool/llm"
	"github.com/alexschlessinger/pollytool/messages"
	"github.com/alexschlessinger/pollytool/sessions"
	"github.com/alexschlessinger/pollytool/tools"

	"pkdindustries/soulshack/internal/config"
	"pkdindustries/soulshack/internal/irc"
)

type CompletionRequest = llm.CompletionRequest

func NewCompletionRequest(config *config.Configuration, session sessions.Session, tools []tools.Tool) *CompletionRequest {
	req := &CompletionRequest{
		BaseURL:     config.API.OpenAIURL,
		Timeout:     config.API.Timeout,
		Model:       config.Model.Model,
		MaxTokens:   config.Model.MaxTokens,
		Messages:    session.GetHistory(),
		Temperature: config.Model.Temperature,
		Tools:       tools,
	}

	if config.Model.Thinking {
		req.ThinkingEffort = "medium"
	}

	return req
}

// Complete processes a user message and returns a channel of response chunks.
func Complete(ctx irc.ChatContextInterface, msg string) (<-chan string, error) {
	// Add user message to session
	cmsg := messages.ChatMessage{
		Role:    messages.MessageRoleUser,
		Content: msg,
	}
	truncated := msg
	if len(truncated) > 100 {
		truncated = truncated[:100] + "..."
	}
	ctx.GetLogger().Infof("Processing user message: %q", truncated)
	ctx.GetSession().AddMessage(cmsg)

	// Build completion request
	session := ctx.GetSession()
	cfg := ctx.GetConfig()
	sys := ctx.GetSystem()

	var allTools []tools.Tool
	if sys.GetToolRegistry() != nil {
		allTools = sys.GetToolRegistry().All()
	}

	req := NewCompletionRequest(cfg, session, allTools)

	// Get response stream from LLM
	stream := sys.GetLLM().ChatCompletionStream(ctx, req)

	// Wrap with duration logging
	output := make(chan string, 10)
	startTime := time.Now()

	go func() {
		defer close(output)
		defer func() {
			ctx.GetLogger().Infow("Request completed", "duration_ms", time.Since(startTime).Milliseconds())
		}()

		for chunk := range stream {
			output <- chunk
		}
	}()

	return output, nil
}
