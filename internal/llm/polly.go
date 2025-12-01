package llm

import (
	"bytes"
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
	// Map soulshack's API keys to pollytool's expected format
	apiKeys := map[string]string{
		"openai":    config.OpenAIKey,
		"anthropic": config.AnthropicKey,
		"gemini":    config.GeminiKey,
		"ollama":    config.OllamaKey,
	}

	return &PollyLLM{
		client: llm.NewMultiPass(apiKeys),
	}
}

// ChatCompletionStream returns a single byte channel with chunked output for IRC
func (p *PollyLLM) ChatCompletionStream(req *CompletionRequest, chatCtx core.ChatContextInterface) <-chan []byte {
	// Set base URL for ollama if provided
	cfg := chatCtx.GetConfig()
	if cfg != nil && cfg.API != nil && cfg.API.OllamaURL != "" {
		req.BaseURL = cfg.API.OllamaURL
	}

	// Get the system for registry access
	sys := chatCtx.GetSystem()
	registry := sys.GetToolRegistry()

	// Create processor with IRC context and chunking
	maxChunkSize := 400 // Default IRC chunk size
	if cfg.Session.ChunkMax > 0 {
		maxChunkSize = cfg.Session.ChunkMax
	}

	// Create byte channel for IRC output
	byteChan := make(chan []byte, 10)

	go func() {
		defer close(byteChan)

		// Create the agent with the tool registry
		agent := llm.NewAgent(p.client, registry, llm.AgentConfig{
			MaxIterations: 10,
			ToolTimeout:   cfg.API.Timeout,
		})

		// Create IRC chunker for content streaming
		chunker := newIRCChunker(byteChan, maxChunkSize)

		var lastThinkingTime time.Time
		startTime := time.Now()

		// Run completion using the agent
		resp, err := agent.Run(chatCtx, req, &llm.AgentCallbacks{
			OnReasoning: func(content string) {
				chatCtx.GetLogger().Debugf("Reasoning: %q", content)

				// Show thinking action if enabled, throttled to every 5 seconds
				if cfg.Bot.ShowThinkingAction {
					now := time.Now()
					if lastThinkingTime.IsZero() || now.Sub(lastThinkingTime) > 5*time.Second {
						elapsed := now.Sub(startTime).Round(time.Second)
						if elapsed > 0 {
							chatCtx.Action(fmt.Sprintf("is thinking... (%s)", elapsed))
						} else {
							chatCtx.Action("is thinking...")
						}
						lastThinkingTime = now
					}
				}
			},
			OnContent: func(content string) {
				chatCtx.GetLogger().Debugf("Content chunk: %q", content)
				chunker.Write(content)
			},
			BeforeToolExecute: func(ctx context.Context, tc messages.ChatMessageToolCall, args map[string]any) context.Context {
				// Inject IRC context for IRC tools (irc_action, etc.)
				return irc.InjectContext(ctx, chatCtx)
			},
			OnToolStart: func(tc messages.ChatMessageToolCall) {
				// Show IRC action for tool call if enabled
				if cfg.Bot.ShowToolActions && tc.Name != "irc_action" {
					// Strip namespace prefix for display (e.g., "script__weather" -> "weather")
					displayName := tc.Name
					if idx := strings.Index(displayName, "__"); idx != -1 {
						displayName = displayName[idx+2:]
					}
					chatCtx.Action(fmt.Sprintf("calling %s", displayName))
				}

				// Log tool execution start
				toolLogger := core.WithTool(chatCtx.GetLogger(), tc.Name, nil)
				toolLogger.Info("Executing tool")
			},
			OnToolEnd: func(tc messages.ChatMessageToolCall, result string, duration time.Duration, toolErr error) {
				toolLogger := core.WithTool(chatCtx.GetLogger(), tc.Name, nil)
				if toolErr != nil {
					toolLogger.With(
						"duration_ms", duration.Milliseconds(),
						"error", toolErr.Error(),
					).Error("Tool execution failed")
				} else {
					// Log tool output (truncate if too long)
					outputPreview := result
					if len(outputPreview) > 200 && !cfg.Bot.Verbose {
						outputPreview = outputPreview[:200] + "..."
					}
					toolLogger.With(
						"duration_ms", duration.Milliseconds(),
						"result_size", len(result),
					).Infof("Tool execution completed: %s", outputPreview)
				}
			},
			OnError: func(err error) {
				chatCtx.GetLogger().Errorf("Stream error: %v", err)
				chunker.Write(fmt.Sprintf("Error: %v", err))
			},
		})

		// Flush any remaining buffer content
		chunker.Flush()

		if err != nil {
			chatCtx.GetLogger().Errorf("Agent error: %v", err)
			return
		}

		// Add all generated messages to session
		for _, msg := range resp.AllMessages {
			chatCtx.GetSession().AddMessage(msg)
		}
	}()

	return byteChan
}

// ircChunker handles chunking of content for IRC message limits
type ircChunker struct {
	byteChan     chan<- []byte
	buffer       *bytes.Buffer
	maxChunkSize int
}

func newIRCChunker(byteChan chan<- []byte, maxChunkSize int) *ircChunker {
	return &ircChunker{
		byteChan:     byteChan,
		buffer:       &bytes.Buffer{},
		maxChunkSize: maxChunkSize,
	}
}

func (c *ircChunker) Write(content string) {
	c.buffer.WriteString(content)

	// Emit complete lines immediately
	for {
		line, err := c.buffer.ReadString('\n')
		if err != nil {
			// No more complete lines, put back what we read
			if line != "" {
				c.buffer.WriteString(line)
			}
			break
		}
		// Remove the newline and send
		if line = line[:len(line)-1]; line != "" {
			c.byteChan <- []byte(line)
		}
	}

	// If buffer is getting too large, force a chunk
	if c.buffer.Len() >= c.maxChunkSize {
		chunk := c.extractBestSplitChunk()
		if chunk != nil {
			c.byteChan <- chunk
		}
	}
}

func (c *ircChunker) extractBestSplitChunk() []byte {
	if c.buffer.Len() == 0 {
		return nil
	}

	data := c.buffer.Bytes()
	end := min(c.maxChunkSize, len(data))

	// Try to find a space within the allowed range to break cleanly
	if idx := bytes.LastIndexByte(data[:end], ' '); idx > 0 {
		chunk := make([]byte, idx)
		copy(chunk, data[:idx])
		c.buffer.Next(idx + 1) // Skip the space itself
		return chunk
	}

	// If no space is found, hard break at maxChunkSize
	chunk := make([]byte, end)
	copy(chunk, data[:end])
	c.buffer.Next(end)
	return chunk
}

func (c *ircChunker) Flush() {
	if c.buffer.Len() > 0 {
		c.byteChan <- c.buffer.Bytes()
		c.buffer.Reset()
	}
}

// CreateAgentForRegistry creates an agent with the given registry for external use
func CreateAgentForRegistry(client *llm.MultiPass, registry *tools.ToolRegistry, timeout time.Duration) *llm.Agent {
	return llm.NewAgent(client, registry, llm.AgentConfig{
		MaxIterations: 10,
		ToolTimeout:   timeout,
	})
}
