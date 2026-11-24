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

// summarized reports whether msgs include a compaction summary; cleared tool results don't count.
func summarized(msgs []messages.ChatMessage) bool {
	for _, msg := range msgs {
		if c, ok := msg.Compaction(); ok && c.Summary != "" {
			return true
		}
	}
	return false
}

// checkContextUsage warns from the request polly sent, not the transcript or billing.
func checkContextUsage(turn *core.Turn, usage memory.ContextUsage) {
	previous, _ := turn.Conversation.Usage()
	if usage.Compacted && !previous.Compacted {
		turn.ReplyAction("Model input compacted: older exchanges are now a summary")
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
		// BaseURL is set per provider at dispatch.
		Timeout:   config.API.Timeout,
		Model:     config.Model.Model,
		MaxTokens: config.Model.MaxTokens,
		Messages:  history,
		// Polly compacts the request, not the stored transcript, to this.
		MaxContextTokens: budget,
		Temperature:      llm.Float32Ptr(config.Model.Temperature),
		Tools:            tools,
		ThinkingEffort:   thinkingEffort,
	}

	if !config.Model.Stream {
		req.StreamMode = llm.Buffered
	}

	return req
}

// Complete appends msg and streams the reply. Callers must drain the stream so the turn is saved.
func Complete(turn *core.Turn, msg string) core.CompletionStream {
	return complete(turn, msg, "")
}

// CompleteWithoutDelegating is Complete without the spawn tool, for turns nobody asked for
// (child reports, unprompted replies).
func CompleteWithoutDelegating(turn *core.Turn, msg string) core.CompletionStream {
	return complete(turn, msg, subagent.ToolName)
}

// Drain replies with every chunk and reports whether the run succeeded with a nonempty reply.
func Drain(stream core.CompletionStream, reply func(string)) bool {
	answered := false
	for chunk := range stream.Chunks {
		answered = answered || strings.TrimSpace(chunk) != ""
		reply(chunk)
	}
	return <-stream.Done == nil && answered
}

func complete(turn *core.Turn, msg string, without string) core.CompletionStream {
	cfg := turn.GetConfig()
	history := turn.Conversation.Messages()

	var allTools []tools.Tool
	if registry := turn.GetSystem().GetToolRegistry(); registry != nil {
		for _, tool := range registry.All() {
			if tool.GetName() != without {
				allTools = append(allTools, tool)
			}
		}
	}

	budget := turn.GetSystem().GetMemory().Budget()
	share := messageShare(budget, contextFloor(cfg.Bot.Prompt, allTools))
	cmsg, err := boundMessage(turn, turn.Conversation.Artifacts(), messages.ChatMessage{
		Role:    messages.MessageRoleUser,
		Content: msg,
	}, share)
	if err != nil {
		// Send it unbounded; it may still fit.
		turn.GetLogger().Error("message_bound_failed", "error", err)
	}
	truncated := msg
	if len(truncated) > 100 {
		truncated = truncated[:100] + "..."
	}
	turn.GetLogger().Info("message_received", "message", truncated)
	if err := turn.Conversation.Append([]messages.ChatMessage{cmsg}); err != nil {
		turn.GetLogger().Error("conversation_append_failed", "error", err)
	}

	// Prompt is rebuilt per request, not stored, so /set prompt applies at once.
	request := make([]messages.ChatMessage, 0, len(history)+2)
	if prompt := strings.TrimSpace(cfg.Bot.Prompt); prompt != "" {
		request = append(request, messages.ChatMessage{Role: messages.MessageRoleSystem, Content: prompt})
	}
	request = append(request, history...)
	request = append(request, cmsg)

	return turn.GetSystem().GetLLM().ChatCompletionStream(turn, NewCompletionRequest(cfg, request, budget, allTools))
}
