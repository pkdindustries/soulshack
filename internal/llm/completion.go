package llm

import (
	"pkdindustries/soulshack/internal/config"
	"pkdindustries/soulshack/internal/core"
	"time"

	"github.com/alexschlessinger/pollytool/llm"
	"github.com/alexschlessinger/pollytool/messages"
	"github.com/alexschlessinger/pollytool/sessions"
	"github.com/alexschlessinger/pollytool/tools"
)

type LLM interface {
	// New simplified interface - single byte channel output
	ChatCompletionStream(*CompletionRequest, core.ChatContextInterface) <-chan []byte
}

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

func CompleteWithText(ctx core.ChatContextInterface, msg string) (<-chan string, error) {
	cmsg := messages.ChatMessage{
		Role:    messages.MessageRoleUser,
		Content: msg,
	}
	truncated := cmsg.Content
	if len(truncated) > 100 {
		truncated = truncated[:100] + "..."
	}
	ctx.GetLogger().Infof("Processing user message: %q", truncated)
	ctx.GetSession().AddMessage(cmsg)

	return complete(ctx)
}

func complete(ctx core.ChatContextInterface) (<-chan string, error) {
	session := ctx.GetSession()
	config := ctx.GetConfig()
	sys := ctx.GetSystem()

	// Get all tools from registry
	var allTools []tools.Tool
	if sys.GetToolRegistry() != nil {
		allTools = sys.GetToolRegistry().All()
	}

	req := NewCompletionRequest(config, session, allTools)
	llm := NewPollyLLM(*config.API)

	// Get the byte stream from the new interface
	byteChan := llm.ChatCompletionStream(req, ctx)

	// Convert bytes to strings for IRC output
	outputChan := make(chan string, 10)

	startTime := time.Now()

	go func() {
		defer close(outputChan)
		defer func() {
			duration := time.Since(startTime)
			ctx.GetLogger().Infow("Request completed", "duration_ms", duration.Milliseconds())
		}()

		for bytes := range byteChan {
			outputChan <- string(bytes)
		}
	}()

	return outputChan, nil
}
