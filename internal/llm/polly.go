package llm

import (
	"fmt"
	"strings"
	"time"

	"github.com/alexschlessinger/pollytool/llm"
	"github.com/alexschlessinger/pollytool/messages"
	"github.com/alexschlessinger/pollytool/tools"

	"pkdindustries/soulshack/internal/config"
	"pkdindustries/soulshack/internal/core"
	"pkdindustries/soulshack/internal/memory"
)

// PollyLLM wraps pollytool's MultiPass and Agent to implement soulshack's LLM interface
type PollyLLM struct {
	client llm.LLM
}

// NewPollyLLM creates a new pollytool-based LLM client
func NewPollyLLM(config config.APIConfig) *PollyLLM {
	apiKeys := map[string]string{
		"openai":    config.OpenAIKey,
		"anthropic": config.AnthropicKey,
		"gemini":    config.GeminiKey,
		"ollama":    config.OllamaKey,
	}
	return &PollyLLM{client: llm.NewMultiPass(apiKeys)}
}

// ChatCompletionStream returns a channel of string chunks for IRC output
func (p *PollyLLM) ChatCompletionStream(turn *core.Turn, req *CompletionRequest) <-chan string {
	cfg := turn.GetConfig()
	// Only apply OllamaURL for ollama/ models
	if strings.HasPrefix(req.Model, "ollama/") && cfg.API.OllamaURL != "" {
		req.BaseURL = cfg.API.OllamaURL
	}

	output := make(chan string, 10)

	go func() {
		defer close(output)

		agent := CreateAgentForRegistry(p.client, turn.GetSystem().GetToolRegistry(), cfg.API.Timeout)
		defer agent.Close()

		framer := turn.NewChunkWriter(output)
		cb := newCallbackHandler(turn, framer, cfg)

		resp, err := agent.Run(turn, req, cb.build())

		framer.Flush()

		if err != nil {
			turn.GetLogger().Error("agent_error", "error", err.Error())
			return
		}
		usage := memory.ContextUsage{
			EstimatedTokens:  resp.Projection.RequestEstimatedTokens,
			Budget:           req.MaxContextTokens,
			OmittedExchanges: resp.Projection.OmittedExchanges,
		}
		turn.Conversation.Append(resp.AllMessages)
		turn.Conversation.SetUsage(usage)
		checkContextUsage(turn, usage)
	}()

	return output
}

// callbackHandler organizes callback construction
type callbackHandler struct {
	turn             *core.Turn
	framer           core.ChunkWriter
	cfg              *config.Configuration
	startTime        time.Time
	lastThinkingTime time.Time
	toolCount        int
}

func newCallbackHandler(turn *core.Turn, framer core.ChunkWriter, cfg *config.Configuration) *callbackHandler {
	return &callbackHandler{
		turn:      turn,
		framer:    framer,
		cfg:       cfg,
		startTime: time.Now(),
	}
}

func (h *callbackHandler) build() *llm.AgentCallbacks {
	return &llm.AgentCallbacks{
		OnReasoning: h.onReasoning,
		OnContent:   h.onContent,
		OnToolStart: h.onToolStart,
		OnToolEnd:   h.onToolEnd,
		OnComplete:  h.onComplete,
		OnError:     h.onError,
	}
}

func (h *callbackHandler) onComplete(response *messages.ChatMessage) {
	duration := time.Since(h.startTime)
	inputTokens := response.GetInputTokens()
	outputTokens := response.GetOutputTokens()

	fields := []any{"duration_ms", duration.Milliseconds()}
	if inputTokens > 0 || outputTokens > 0 {
		fields = append(fields, "input_tokens", inputTokens, "output_tokens", outputTokens)
	}
	if h.toolCount > 0 {
		fields = append(fields, "tool_count", h.toolCount)
	}
	h.turn.GetLogger().Info("request_complete", fields...)
}

func (h *callbackHandler) onReasoning(content string) {
	h.turn.GetLogger().Debug("reasoning_chunk", "content", content)

	if !h.cfg.Bot.ShowThinkingAction {
		return
	}

	now := time.Now()
	if now.Sub(h.startTime) > 30*time.Second {
		if h.lastThinkingTime.IsZero() || now.Sub(h.lastThinkingTime) > 30*time.Second {
			elapsed := now.Sub(h.startTime).Round(time.Second)
			h.turn.ReplyAction(fmt.Sprintf("is thinking... (%s)", elapsed))
			h.lastThinkingTime = now
		}
	}
}

func (h *callbackHandler) onContent(content string) {
	h.turn.GetLogger().Debug("oncontent_callback",
		"content", content,
		"content_len", len(content),
	)
	h.framer.Write(content)
}

func (h *callbackHandler) onToolStart(calls []messages.ChatMessageToolCall) {
	h.framer.Flush()

	h.toolCount += len(calls)

	// Log each tool
	for _, tc := range calls {
		h.turn.GetLogger().Info("tool_started", "tool", tc.Name)
	}

	if !h.cfg.Bot.ShowToolActions || len(calls) == 0 {
		return
	}

	// Filter and format tool names
	var names []string
	for _, tc := range calls {
		if tc.Name == "irc__action" {
			continue
		}
		displayName := tc.Name
		if idx := strings.Index(displayName, "__"); idx != -1 {
			displayName = displayName[idx+2:]
		}
		names = append(names, displayName)
	}

	if len(names) > 0 {
		h.turn.ReplyAction(fmt.Sprintf("calling %s", strings.Join(names, ", ")))
	}
}

func (h *callbackHandler) onToolEnd(tc messages.ChatMessageToolCall, result string, duration time.Duration, toolErr error) {
	if toolErr != nil {
		h.turn.GetLogger().Error("tool_failed",
			"tool", tc.Name,
			"duration_ms", duration.Milliseconds(),
			"error", toolErr.Error(),
		)
		return
	}

	preview := result
	if len(preview) > 60 && !h.cfg.Bot.Verbose {
		preview = preview[:60] + "..."
	}
	h.turn.GetLogger().Info("tool_completed",
		"tool", tc.Name,
		"duration_ms", duration.Milliseconds(),
		"result_size", len(result),
		"preview", preview,
	)
}

func (h *callbackHandler) onError(err error) {
	h.turn.GetLogger().Error("stream_error", "error", err.Error())
	h.framer.Write(fmt.Sprintf("Error: %v", err))
}

// CreateAgentForRegistry uses Polly's private per-agent built-ins while sharing
// the caller-owned configured tools, sandbox policy, and MCP connections.
func CreateAgentForRegistry(client llm.LLM, registry *tools.ToolRegistry, timeout time.Duration) *llm.Agent {
	return llm.NewAgent(client, registry, llm.AgentConfig{MaxIterations: 10, ToolTimeout: timeout})
}
