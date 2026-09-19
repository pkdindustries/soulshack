package llm

import (
	"fmt"
	"strings"

	"github.com/alexschlessinger/pollytool/llm"
	"github.com/alexschlessinger/pollytool/messages"
	"github.com/alexschlessinger/pollytool/subagent"
	"github.com/alexschlessinger/pollytool/tools"

	"pkdindustries/soulshack/internal/config"
	"pkdindustries/soulshack/internal/core"
	"pkdindustries/soulshack/internal/memory"
)

type CompletionRequest = llm.CompletionRequest

// Warn from the request Polly actually projected, independently of the
// transcript we keep and of cumulative provider billing. The previous turn
// supplies the warning state, so no second cache is needed.
func checkContextUsage(turn *core.Turn, usage memory.ContextUsage) {
	previous, _ := turn.Conversation.Usage()
	if usage.OmittedExchanges > 0 && previous.OmittedExchanges == 0 {
		turn.ReplyAction(fmt.Sprintf("Model input omitted %d older exchanges", usage.OmittedExchanges))
		return
	}
	level := func(u memory.ContextUsage) int {
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
		turn.ReplyAction(fmt.Sprintf("Model input reached %d%% of its context budget", current))
	}
}

func NewCompletionRequest(config *config.Configuration, history []messages.ChatMessage, budget int, tools []tools.Tool) *CompletionRequest {
	// Parse thinking effort - validated at config load time
	thinkingEffort, _ := llm.ParseThinkingEffort(config.Model.ThinkingEffort)

	req := &CompletionRequest{
		// BaseURL is resolved per provider when the request is dispatched.
		Timeout:   config.API.Timeout,
		Model:     config.Model.Model,
		MaxTokens: config.Model.MaxTokens,
		Messages:  history,
		// The budget the conversation is trimmed to, so the request and the
		// transcript agree; polly omits the oldest exchanges if the request
		// still estimates above it.
		MaxContextTokens: budget,
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

// Complete appends the user message to the turn's conversation and returns the
// stream of response chunks. The stream also appends the finished turn, so
// callers must drain it.
func Complete(turn *core.Turn, msg string) <-chan string {
	return complete(turn, msg, "")
}

// CompleteWithoutDelegating is Complete for the turns that must answer for
// themselves. A turn that reports a child's findings could otherwise answer by
// starting another child, with nobody in the channel having asked for either;
// a turn the bot took on its own initiative, watching what people post, would
// be spending on work nobody requested. Both are turns where the bot is
// talking to itself, and delegation is for what someone asked for.
func CompleteWithoutDelegating(turn *core.Turn, msg string) <-chan string {
	return complete(turn, msg, subagent.ToolName)
}

// Drain replies with each chunk a completion stream emits, as the caller's
// reply function says, and reports whether the completion produced anything.
// The stream is always drained: that is what finishes the turn and stores it,
// whatever the reply function does.
func Drain(chunks <-chan string, reply func(string)) bool {
	answered := false
	for chunk := range chunks {
		answered = true
		reply(chunk)
	}
	return answered
}

// complete runs the turn, offering every tool but the one named in without.
func complete(turn *core.Turn, msg string, without string) <-chan string {
	cfg := turn.GetConfig()
	history := turn.Conversation.Messages()

	cmsg := messages.ChatMessage{
		Role:    messages.MessageRoleUser,
		Content: msg,
	}
	truncated := msg
	if len(truncated) > 100 {
		truncated = truncated[:100] + "..."
	}
	turn.GetLogger().Info("message_received", "message", truncated)
	turn.Conversation.Append([]messages.ChatMessage{cmsg})

	// The system prompt is the process's, not part of the transcript: it is
	// rebuilt per request so /set prompt takes effect immediately.
	request := make([]messages.ChatMessage, 0, len(history)+2)
	if prompt := strings.TrimSpace(cfg.Bot.Prompt); prompt != "" {
		request = append(request, messages.ChatMessage{Role: messages.MessageRoleSystem, Content: prompt})
	}
	request = append(request, history...)
	request = append(request, cmsg)

	var allTools []tools.Tool
	if registry := turn.GetSystem().GetToolRegistry(); registry != nil {
		for _, tool := range registry.All() {
			if tool.GetName() != without {
				allTools = append(allTools, tool)
			}
		}
	}

	budget := turn.GetSystem().GetMemory().Budget()
	return turn.GetSystem().GetLLM().ChatCompletionStream(turn, NewCompletionRequest(cfg, request, budget, allTools))
}
