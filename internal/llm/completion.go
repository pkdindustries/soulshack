package llm

import (
	"fmt"

	"github.com/alexschlessinger/pollytool/llm"
	"github.com/alexschlessinger/pollytool/messages"
	"github.com/alexschlessinger/pollytool/sessions"
	"github.com/alexschlessinger/pollytool/tools"

	"pkdindustries/soulshack/internal/config"
	"pkdindustries/soulshack/internal/core"
	"pkdindustries/soulshack/internal/irc"
)

type CompletionRequest = llm.CompletionRequest

// Warn from the request Polly actually projected, independently of retained
// history size and cumulative provider billing. The previous turn supplies the
// warning state without a second session cache.
func checkContextUsage(ctx irc.ChatContextInterface, usage core.ContextUsage, history []messages.ChatMessage) {
	previous, _ := core.LastContextUsage(history)
	if usage.OmittedExchanges > 0 && previous.OmittedExchanges == 0 {
		ctx.ReplyAction(fmt.Sprintf("Model input omitted %d older exchanges; stored history is retained", usage.OmittedExchanges))
		return
	}
	level := func(u core.ContextUsage) int {
		if u.Budget <= 0 {
			return 0
		}
		percentage := float64(u.EstimatedTokens) / float64(u.Budget) * 100
		if percentage >= 90 {
			return 90
		}
		if percentage >= 75 {
			return 75
		}
		return 0
	}
	if current := level(usage); current > level(previous) {
		ctx.ReplyAction(fmt.Sprintf("Model input reached %d%% of its context budget; stored history is retained", current))
	}
}

func NewCompletionRequest(config *config.Configuration, history []messages.ChatMessage, metadata *sessions.Metadata, tools []tools.Tool) *CompletionRequest {
	// Parse thinking effort - validated at config load time
	thinkingEffort, _ := llm.ParseThinkingEffort(config.Model.ThinkingEffort)

	req := &CompletionRequest{
		BaseURL:   config.API.OpenAIURL,
		Timeout:   config.API.Timeout,
		Model:     config.Model.Model,
		MaxTokens: config.Model.MaxTokens,
		Messages:  history,
		// The projection budget enforces maxcontext: polly omits the oldest
		// exchanges when the history estimate exceeds it.
		MaxContextTokens: metadata.MaxHistoryTokens,
		Temperature:      llm.Float32Ptr(config.Model.Temperature),
		Tools:            tools,
		ThinkingEffort:   thinkingEffort,
	}

	// Set streaming mode (nil = streaming default, false = non-streaming)
	if !config.Model.Stream {
		stream := false
		req.Stream = &stream
	}

	return req
}

// Complete processes a user message and returns a channel of response chunks.
func Complete(ctx irc.ChatContextInterface, msg string) (<-chan string, error) {
	session := ctx.GetSession()
	history, err := session.GetHistory(ctx)
	if err != nil {
		return nil, err
	}
	metadata, err := session.GetMetadata(ctx)
	if err != nil {
		return nil, err
	}

	// Add user message to session
	cmsg := messages.ChatMessage{
		Role:    messages.MessageRoleUser,
		Content: msg,
	}
	truncated := msg
	if len(truncated) > 100 {
		truncated = truncated[:100] + "..."
	}
	ctx.GetLogger().Info("message_received", "message", truncated)
	if err := session.AddMessage(ctx, cmsg); err != nil {
		return nil, err
	}
	history = append(history, cmsg)

	// Build completion request
	cfg := ctx.GetConfig()
	sys := ctx.GetSystem()

	var allTools []tools.Tool
	if sys.GetToolRegistry() != nil {
		allTools = sys.GetToolRegistry().All()
	}

	req := NewCompletionRequest(cfg, history, metadata, allTools)

	// Closing this stream also completes persistence; the conversation helper
	// keeps the lease until the caller has drained it.
	return sys.GetLLM().ChatCompletionStream(ctx, req), nil
}
