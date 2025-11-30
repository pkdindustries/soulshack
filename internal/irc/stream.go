package irc

import (
	"bytes"
	"context"
	"fmt"
	"pkdindustries/soulshack/internal/core"
	"strings"
	"sync/atomic"
	"time"

	"github.com/alexschlessinger/pollytool/llm"
	"github.com/alexschlessinger/pollytool/messages"
	"github.com/alexschlessinger/pollytool/tools"
)

// IRCEventProcessor handles event processing for IRC with chunking and actions
type IRCEventProcessor struct {
	ctx             ChatContextInterface
	byteChan        chan<- []byte
	chunkBuffer     *bytes.Buffer
	maxChunkSize    int
	registry        *tools.ToolRegistry
	client          llm.LLM
	streamProcessor llm.EventStreamProcessor // Add stream processor
	originalModel   string
	response        messages.ChatMessage // Store the response message

	// IRC-specific state
	sentThinkingAction atomic.Bool
	thinkingStartTime  *time.Time
	thinkingTicker     *time.Ticker
	thinkingDone       chan struct{}

	// For handling tool continuation
	req *llm.CompletionRequest
}

// NewIRCEventProcessor creates a new IRC event processor
func NewIRCEventProcessor(
	ctx ChatContextInterface,
	byteChan chan<- []byte,
	maxChunkSize int,
	registry *tools.ToolRegistry,
	client llm.LLM,
	streamProcessor llm.EventStreamProcessor,
) *IRCEventProcessor {
	return &IRCEventProcessor{
		ctx:             ctx,
		byteChan:        byteChan,
		chunkBuffer:     &bytes.Buffer{},
		maxChunkSize:    maxChunkSize,
		registry:        registry,
		client:          client,
		streamProcessor: streamProcessor,
	}
}

// startThinkingTicker starts the periodic thinking notification ticker
func (p *IRCEventProcessor) startThinkingTicker() {
	if p.thinkingStartTime != nil || !p.ctx.GetConfig().Bot.ShowThinkingAction {
		return
	}
	now := time.Now()
	p.thinkingStartTime = &now
	p.thinkingTicker = time.NewTicker(15 * time.Second)
	p.thinkingDone = make(chan struct{})

	go func() {
		for {
			select {
			case <-p.thinkingTicker.C:
				elapsed := time.Since(*p.thinkingStartTime)
				p.ctx.Action(fmt.Sprintf("thinking... (%ds elapsed)", int(elapsed.Seconds())))
			case <-p.thinkingDone:
				return
			}
		}
	}()
	p.ctx.GetLogger().Debug("Started thinking ticker")
}

// stopThinkingTicker stops and cleans up the thinking notification ticker
func (p *IRCEventProcessor) stopThinkingTicker() {
	if p.thinkingTicker != nil {
		p.thinkingTicker.Stop()
		close(p.thinkingDone)
		p.thinkingTicker = nil
		p.thinkingDone = nil
		p.thinkingStartTime = nil
		p.ctx.GetLogger().Debug("Stopped thinking ticker")
	}
}

// OnReasoning handles reasoning content - starts thinking timer for IRC action
func (p *IRCEventProcessor) OnReasoning(content string, totalLength int) {
	p.startThinkingTicker()
	p.ctx.GetLogger().Debugf("Reasoning update: %q", content)
}

// OnContent handles regular content streaming with IRC chunking
func (p *IRCEventProcessor) OnContent(content string, firstChunk bool) {
	p.ctx.GetLogger().Debugf("Received content chunk: %q", content)
	// Stream content through chunking
	p.processContent(content)
}

// OnToolCall handles tool call events
func (p *IRCEventProcessor) OnToolCall(toolCall messages.ChatMessageToolCall) {
	p.ctx.GetLogger().Debugf("Received tool call: %s (ID: %s)", toolCall.Name, toolCall.ID)
	// Tool calls are handled in OnComplete in the current implementation
	// This could be used for real-time tool call notifications if needed
}

// OnComplete handles the complete message and executes tools if needed
func (p *IRCEventProcessor) OnComplete(message *messages.ChatMessage) {
	p.stopThinkingTicker()

	if message != nil {
		// Add the assistant message to session
		p.ctx.GetSession().AddMessage(*message)

		// If message has content, ensure buffer is flushed
		if message.Content != "" && len(message.ToolCalls) == 0 {
			p.FlushBuffer()
		}

		// Store the message for GetResponse
		p.response = *message

		p.ctx.GetLogger().Debugf("Message complete (Role: %s, ContentLen: %d, ToolCalls: %d)", message.Role, len(message.Content), len(message.ToolCalls))
	}
}

// OnError handles errors during streaming
func (p *IRCEventProcessor) OnError(err error) {
	if err != nil {
		p.ctx.GetLogger().Debugf("Stream error: %v", err)
		errMsg := fmt.Sprintf("Error: %v", err)
		p.processContent(errMsg)
	}
}

// GetResponse returns the accumulated response message
func (p *IRCEventProcessor) GetResponse() messages.ChatMessage {
	return p.response
}

// HandleToolContinuation executes tools and continues the conversation
func (p *IRCEventProcessor) HandleToolContinuation(ctx context.Context, req *llm.CompletionRequest) {
	if len(p.response.ToolCalls) == 0 {
		return
	}

	// Execute all tool calls using the executor with IRC-specific hooks
	executor := p.createToolExecutor()
	executor.ExecuteAll(ctx, p.response.ToolCalls, p.ctx.GetSession())

	// Continue the conversation with the tool results
	p.continueConversation(ctx, req)
}

// createToolExecutor creates a ToolExecutor with IRC-specific hooks for context injection and logging
func (p *IRCEventProcessor) createToolExecutor() *llm.ToolExecutor {
	return llm.NewToolExecutor(p.registry).WithHooks(&llm.ExecutionHooks{
		BeforeExecute: func(ctx context.Context, tc messages.ChatMessageToolCall, args map[string]any) context.Context {
			// Show IRC action for tool call if enabled and not the irc_action tool
			if p.ctx.GetConfig().Bot.ShowToolActions && tc.Name != "irc_action" {
				// Strip namespace prefix for display (e.g., "script__weather" -> "weather")
				displayName := tc.Name
				if idx := strings.Index(displayName, "__"); idx != -1 {
					displayName = displayName[idx+2:]
				}
				p.ctx.Action(fmt.Sprintf("calling %s", displayName))
			}

			// Log tool execution start
			toolLogger := core.WithTool(p.ctx.GetLogger(), tc.Name, args)
			toolLogger.Info("Executing tool")

			// Inject IRC context for IRC tools
			return context.WithValue(ctx, kContextKey, p.ctx)
		},
		AfterExecute: func(tc messages.ChatMessageToolCall, result string, duration time.Duration, err error) {
			toolLogger := core.WithTool(p.ctx.GetLogger(), tc.Name, nil)
			if err != nil {
				toolLogger.With(
					"duration_ms", duration.Milliseconds(),
					"error", err.Error(),
				).Error("Tool execution failed")
			} else {
				// Log tool output (truncate if too long)
				outputPreview := result
				if len(outputPreview) > 200 && !p.ctx.GetConfig().Bot.Verbose {
					outputPreview = outputPreview[:200] + "..."
				}
				toolLogger.With(
					"duration_ms", duration.Milliseconds(),
					"result_size", len(result),
				).Infof("Tool execution completed: %s", outputPreview)
			}
		},
	})
}

// continueConversation resumes the chat stream after tool execution
func (p *IRCEventProcessor) continueConversation(ctx context.Context, req *llm.CompletionRequest) {
	// Update request with new history including tool results
	req.Messages = p.ctx.GetSession().GetHistory()
	// Restore the original model name (MultiPass strips the provider prefix)
	req.Model = p.originalModel

	// Reset thinking action flag and ticker for continuation
	p.sentThinkingAction.Store(false)
	p.stopThinkingTicker()

	// Create a new processor for the continuation
	continuationProcessor := NewIRCEventProcessor(p.ctx, p.byteChan, p.maxChunkSize, p.registry, p.client, p.streamProcessor)
	continuationProcessor.originalModel = p.originalModel

	// Get new event stream with tool results
	eventChan := p.client.ChatCompletionStream(ctx, req, p.streamProcessor)

	// Process the continuation
	response := messages.ProcessEventStream(ctx, eventChan, continuationProcessor)

	// If there are more tool calls, continue recursively
	if len(response.ToolCalls) > 0 {
		continuationProcessor.HandleToolContinuation(ctx, req)
	}
}

// SetRequest stores the request for tool continuation
func (p *IRCEventProcessor) SetRequest(req *llm.CompletionRequest) {
	p.req = req
	p.originalModel = req.Model
}

// processContent handles chunking of content for IRC message limits
func (p *IRCEventProcessor) processContent(content string) {
	p.chunkBuffer.WriteString(content)

	// Emit complete lines immediately
	for {
		line, err := p.chunkBuffer.ReadString('\n')
		if err != nil {
			// No more complete lines, put back what we read
			if line != "" {
				p.chunkBuffer.WriteString(line)
			}
			break
		}
		// Remove the newline and send
		if line = line[:len(line)-1]; line != "" {
			p.byteChan <- []byte(line)
		}
	}

	// If buffer is getting too large, force a chunk
	if p.chunkBuffer.Len() >= p.maxChunkSize {
		chunk := p.extractBestSplitChunk()
		if chunk != nil {
			p.byteChan <- chunk
		}
	}
}

// extractBestSplitChunk extracts a properly sized chunk, preferring to break on spaces.
// It returns a byte slice of at most maxChunkSize length.
func (p *IRCEventProcessor) extractBestSplitChunk() []byte {
	if p.chunkBuffer.Len() == 0 {
		return nil
	}

	data := p.chunkBuffer.Bytes()
	// Determine the maximum possible length for this chunk
	end := min(p.maxChunkSize, len(data))

	// Try to find a space within the allowed range to break cleanly
	// We look for the *last* space to maximize the chunk size
	if idx := bytes.LastIndexByte(data[:end], ' '); idx > 0 {
		chunk := make([]byte, idx)
		copy(chunk, data[:idx])
		p.chunkBuffer.Next(idx + 1) // Skip the space itself so it's not at the start of next chunk
		return chunk
	}

	// If no space is found, we have to hard break at maxChunkSize
	chunk := make([]byte, end)
	copy(chunk, data[:end])
	p.chunkBuffer.Next(end)
	return chunk
}

// FlushBuffer sends any remaining content in the buffer
func (p *IRCEventProcessor) FlushBuffer() {
	if p.chunkBuffer.Len() > 0 {
		p.byteChan <- p.chunkBuffer.Bytes()
		p.chunkBuffer.Reset()
	}
}
