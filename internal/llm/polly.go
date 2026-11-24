package llm

import (
	"errors"
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

// turnMaxIterations bounds one turn's tool loop.
const turnMaxIterations = 10

// PollyLLM wraps pollytool's MultiPass and Agent to implement soulshack's LLM interface
type PollyLLM struct {
	client llm.LLM
}

// NewPollyLLM creates a new pollytool-based LLM client
func NewPollyLLM(config config.APIConfig) *PollyLLM {
	apiKeys := map[string]string{
		"openai":      config.OpenAIKey,
		"anthropic":   config.AnthropicKey,
		"gemini":      config.GeminiKey,
		"ollama":      config.OllamaKey,
		"deepseek":    config.DeepSeekKey,
		"openrouter":  config.OpenRouterKey,
		"huggingface": config.HuggingFaceKey,
	}
	return &PollyLLM{client: llm.NewMultiPass(apiKeys)}
}

// ChatCompletionStream returns IRC output and the completion's final outcome.
func (p *PollyLLM) ChatCompletionStream(turn *core.Turn, req *CompletionRequest) core.CompletionStream {
	cfg := turn.GetConfig()
	req.BaseURL = baseURLForModel(req.Model, cfg.API)

	output := make(chan string, 10)
	done := make(chan error, 1)

	go func() {
		var err error
		defer func() {
			close(output)
			done <- err
			close(done)
		}()

		registry, release := offeredRegistry(turn.GetSystem().GetToolRegistry(), req.Tools)
		defer release()

		agent := CreateAgent(p.client, registry, llm.AgentConfig{
			MaxIterations: turnMaxIterations,
			ToolTimeout:   cfg.API.Timeout,
			ArtifactStore: turn.Conversation.Artifacts(),
		})
		defer agent.Close()

		framer := turn.NewChunkWriter(output)
		cb := newCallbackHandler(turn, framer, cfg)

		resp, err := agent.Run(turn, req, cb.build())

		framer.Flush()

		// Keep partial output on error: the tool calls happened, and polly ends it at a valid boundary.
		if resp != nil {
			if err := turn.Conversation.Append(resp.AllMessages); err != nil {
				turn.GetLogger().Error("conversation_append_failed", "error", err)
			}
		}
		if err != nil {
			turn.GetLogger().Error("agent_error", "error", err.Error())
			return
		}
		// Compacted is sticky: once summarized, later requests keep the summary.
		tokens := resp.Projection.RequestEstimatedTokens
		if resp.Projection.Counted {
			tokens = resp.Projection.CountedTokens
		}
		previous, _ := turn.Conversation.Usage()
		usage := memory.ContextUsage{
			EstimatedTokens: tokens,
			Budget:          req.MaxContextTokens,
			Compacted:       previous.Compacted || summarized(resp.AllMessages),
		}
		checkContextUsage(turn, usage)
		turn.Conversation.SetUsage(usage)
	}()

	return core.CompletionStream{Chunks: output, Done: done}
}

// baseURLForModel returns the endpoint override for the model's provider, or "" for polly's default.
func baseURLForModel(model string, api *config.APIConfig) string {
	provider, _, ok := strings.Cut(model, "/")
	if !ok {
		return ""
	}
	// Polly routes case-insensitively; the model name itself may be case-sensitive.
	switch strings.ToLower(provider) {
	case "ollama":
		return api.OllamaURL
	case "openai":
		return api.OpenAIURL
	}
	return ""
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
	// Context limits that survive compaction are reported plainly, not as faults.
	var limit *llm.ContextLimitError
	if errors.As(err, &limit) {
		h.framer.Write(fmt.Sprintf("Stopped: this needs about %d tokens of context and only %d fit", limit.EstimatedTokens, limit.Limit))
		return
	}
	var overflow *llm.ContextOverflowError
	if errors.As(err, &overflow) {
		if overflow.Window > 0 {
			h.framer.Write(fmt.Sprintf("Stopped: this does not fit the model's %d-token context window", overflow.Window))
		} else {
			h.framer.Write("Stopped: this does not fit the model's context window")
		}
		return
	}
	h.framer.Write(fmt.Sprintf("Error: %v", err))
}

// CreateAgent builds an agent over the caller-owned registry. Polly's builtins are installed only with an artifact store.
func CreateAgent(client llm.LLM, registry *tools.ToolRegistry, config llm.AgentConfig) *llm.Agent {
	if config.ArtifactStore == nil {
		config.Builtins = []string{}
	}
	return llm.NewAgent(client, registry, config)
}

// offeredRegistry narrows registry to the offered tools, since the agent runs by registry, not request.
// release closes only the derived view.
func offeredRegistry(registry *tools.ToolRegistry, offered []tools.Tool) (*tools.ToolRegistry, func()) {
	nothing := func() {}
	if registry == nil {
		return nil, nothing
	}

	present := make(map[string]bool, len(offered))
	for _, tool := range offered {
		present[tool.GetName()] = true
	}
	var withheld []string
	for _, tool := range registry.All() {
		if !present[tool.GetName()] {
			withheld = append(withheld, tool.GetName())
		}
	}
	if len(withheld) == 0 {
		return registry, nothing
	}

	derived := registry.Derive(tools.DenyTools(withheld...))
	return derived, func() { derived.Close() }
}
