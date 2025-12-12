package llm

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/alexschlessinger/pollytool/llm"
	"github.com/alexschlessinger/pollytool/messages"
	"github.com/alexschlessinger/pollytool/tools"

	"pkdindustries/soulshack/internal/config"
	"pkdindustries/soulshack/internal/core"
	"pkdindustries/soulshack/internal/irc"
)

// PollyLLM wraps pollytool's MultiPass and Agent to implement soulshack's LLM interface
type PollyLLM struct {
	client *llm.MultiPass
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
func (p *PollyLLM) ChatCompletionStream(chatCtx core.ChatContextInterface, req *CompletionRequest) <-chan string {
	cfg := chatCtx.GetConfig()
	// Only apply OllamaURL for ollama/ models
	if strings.HasPrefix(req.Model, "ollama/") && cfg.API.OllamaURL != "" {
		req.BaseURL = cfg.API.OllamaURL
	}

	maxChunkSize := 400
	if cfg.Session.ChunkMax > 0 {
		maxChunkSize = cfg.Session.ChunkMax
	}

	output := make(chan string, 10)

	go func() {
		defer close(output)

		agent := llm.NewAgent(p.client, chatCtx.GetSystem().GetToolRegistry(), llm.AgentConfig{
			MaxIterations: 10,
			ToolTimeout:   cfg.API.Timeout,
		})

		chunker := irc.NewChunker(output, maxChunkSize)
		cb := newCallbackHandler(chatCtx, chunker, cfg)

		resp, err := agent.Run(chatCtx, req, cb.build())

		chunker.Flush()

		if err != nil {
			chatCtx.GetLogger().Errorw("agent_error", "error", err.Error())
			return
		}

		for _, msg := range resp.AllMessages {
			chatCtx.GetSession().AddMessage(msg)
		}
	}()

	return output
}

// callbackHandler organizes callback construction
type callbackHandler struct {
	chatCtx          core.ChatContextInterface
	chunker          *irc.Chunker
	cfg              *config.Configuration
	startTime        time.Time
	lastThinkingTime time.Time
	toolCount        int
}

func newCallbackHandler(chatCtx core.ChatContextInterface, chunker *irc.Chunker, cfg *config.Configuration) *callbackHandler {
	return &callbackHandler{
		chatCtx:   chatCtx,
		chunker:   chunker,
		cfg:       cfg,
		startTime: time.Now(),
	}
}

func (h *callbackHandler) build() *llm.AgentCallbacks {
	return &llm.AgentCallbacks{
		OnReasoning:       h.onReasoning,
		OnContent:         h.onContent,
		BeforeToolExecute: h.beforeToolExecute,
		OnToolStart:       h.onToolStart,
		OnToolEnd:         h.onToolEnd,
		OnComplete:        h.onComplete,
		OnError:           h.onError,
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
	h.chatCtx.GetLogger().Infow("request_complete", fields...)
}

func (h *callbackHandler) onReasoning(content string) {
	h.chatCtx.GetLogger().Debugw("reasoning_chunk", "content", content)

	if !h.cfg.Bot.ShowThinkingAction {
		return
	}

	now := time.Now()
	if now.Sub(h.startTime) > 30*time.Second {
		if h.lastThinkingTime.IsZero() || now.Sub(h.lastThinkingTime) > 30*time.Second {
			elapsed := now.Sub(h.startTime).Round(time.Second)
			h.chatCtx.ReplyAction(fmt.Sprintf("is thinking... (%s)", elapsed))
			h.lastThinkingTime = now
		}
	}
}

func (h *callbackHandler) onContent(content string) {
	h.chatCtx.GetLogger().Debugw("oncontent_callback",
		"content", content,
		"content_len", len(content),
	)
	h.chunker.Write(content)
}

func (h *callbackHandler) beforeToolExecute(ctx context.Context, tc messages.ChatMessageToolCall, args map[string]any) context.Context {
	return irc.InjectContext(ctx, h.chatCtx)
}

func (h *callbackHandler) onToolStart(calls []messages.ChatMessageToolCall) {
	h.chunker.Flush()

	h.toolCount += len(calls)

	// Log each tool
	for _, tc := range calls {
		h.chatCtx.GetLogger().Infow("tool_started", "tool", tc.Name)
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
		h.chatCtx.ReplyAction(fmt.Sprintf("calling %s", strings.Join(names, ", ")))
	}
}

func (h *callbackHandler) onToolEnd(tc messages.ChatMessageToolCall, result string, duration time.Duration, toolErr error) {
	if toolErr != nil {
		h.chatCtx.GetLogger().Errorw("tool_failed",
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
	h.chatCtx.GetLogger().Infow("tool_completed",
		"tool", tc.Name,
		"duration_ms", duration.Milliseconds(),
		"result_size", len(result),
		"preview", preview,
	)
}

func (h *callbackHandler) onError(err error) {
	h.chatCtx.GetLogger().Errorw("stream_error", "error", err.Error())
	h.chunker.Write(fmt.Sprintf("Error: %v", err))
}

// CreateAgentForRegistry creates an agent with the given registry for external use
func CreateAgentForRegistry(client *llm.MultiPass, registry *tools.ToolRegistry, timeout time.Duration) *llm.Agent {
	return llm.NewAgent(client, registry, llm.AgentConfig{
		MaxIterations: 10,
		ToolTimeout:   timeout,
	})
}
