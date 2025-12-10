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
			chatCtx.GetLogger().Errorf("Agent error: %v", err)
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
		OnError:           h.onError,
	}
}

func (h *callbackHandler) onReasoning(content string) {
	h.chatCtx.GetLogger().Debugf("Reasoning: %q", content)

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
	h.chatCtx.GetLogger().Debugf("Content chunk: %q", content)
	h.chunker.Write(content)
}

func (h *callbackHandler) beforeToolExecute(ctx context.Context, tc messages.ChatMessageToolCall, args map[string]any) context.Context {
	return irc.InjectContext(ctx, h.chatCtx)
}

func (h *callbackHandler) onToolStart(tc messages.ChatMessageToolCall) {
	if h.cfg.Bot.ShowToolActions && tc.Name != "irc__action" {
		displayName := tc.Name
		if idx := strings.Index(displayName, "__"); idx != -1 {
			displayName = displayName[idx+2:]
		}
		h.chatCtx.ReplyAction(fmt.Sprintf("calling %s", displayName))
	}

	core.WithTool(h.chatCtx.GetLogger(), tc.Name, nil).Info("Executing tool")
}

func (h *callbackHandler) onToolEnd(tc messages.ChatMessageToolCall, result string, duration time.Duration, toolErr error) {
	logger := core.WithTool(h.chatCtx.GetLogger(), tc.Name, nil)
	if toolErr != nil {
		logger.With("duration_ms", duration.Milliseconds(), "error", toolErr.Error()).Error("Tool execution failed")
		return
	}

	preview := result
	if len(preview) > 200 && !h.cfg.Bot.Verbose {
		preview = preview[:200] + "..."
	}
	logger.With("duration_ms", duration.Milliseconds(), "result_size", len(result)).Infof("Tool completed: %s", preview)
}

func (h *callbackHandler) onError(err error) {
	h.chatCtx.GetLogger().Errorf("Stream error: %v", err)
	h.chunker.Write(fmt.Sprintf("Error: %v", err))
}

// CreateAgentForRegistry creates an agent with the given registry for external use
func CreateAgentForRegistry(client *llm.MultiPass, registry *tools.ToolRegistry, timeout time.Duration) *llm.Agent {
	return llm.NewAgent(client, registry, llm.AgentConfig{
		MaxIterations: 10,
		ToolTimeout:   timeout,
	})
}
