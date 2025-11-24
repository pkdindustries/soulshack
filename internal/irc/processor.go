package irc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"pkdindustries/soulshack/internal/core"
	"strings"
	"time"

	"github.com/alexschlessinger/pollytool/llm"
	"github.com/alexschlessinger/pollytool/messages"
	"github.com/alexschlessinger/pollytool/tools"
)

// IRCEventProcessor handles event processing for IRC with chunking and actions
type IRCEventProcessor struct {
	ctx             core.ChatContextInterface
	byteChan        chan<- []byte
	chunkBuffer     *bytes.Buffer
	maxChunkSize    int
	registry        *tools.ToolRegistry
	client          llm.LLM
	streamProcessor llm.EventStreamProcessor // Add stream processor
	originalModel   string
	response        messages.ChatMessage // Store the response message

	// IRC-specific state
	sentThinkingAction bool
	thinkingStartTime  *time.Time
	thinkingTimer      *time.Timer

	// For handling tool continuation
	req *llm.CompletionRequest
}

// NewIRCEventProcessor creates a new IRC event processor
func NewIRCEventProcessor(
	ctx core.ChatContextInterface,
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

// OnReasoning handles reasoning content - starts thinking timer for IRC action
func (p *IRCEventProcessor) OnReasoning(content string, totalLength int) {
	// Start timer on first reasoning event, send action after 5 seconds if still thinking
	// Only show thinking action if both thinking mode is enabled AND showthinkingaction is true
	if !p.sentThinkingAction && p.ctx.GetConfig().Model.Thinking && p.ctx.GetConfig().Bot.ShowThinkingAction {
		if p.thinkingStartTime == nil {
			// First reasoning event - start timer
			now := time.Now()
			p.thinkingStartTime = &now
			p.thinkingTimer = time.AfterFunc(5*time.Second, func() {
				if !p.sentThinkingAction {
					channel := p.ctx.GetConfig().Server.Channel
					p.ctx.Action(channel, "still thinking.")
					p.sentThinkingAction = true
				}
			})
			p.ctx.GetLogger().Debug("Started thinking timer")
		}
	}

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
	// Cancel thinking timer if it hasn't fired yet
	if p.thinkingTimer != nil && !p.sentThinkingAction {
		p.thinkingTimer.Stop()
		p.thinkingTimer = nil
		p.thinkingStartTime = nil
		p.ctx.GetLogger().Debug("Cancelled thinking timer")
	}

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

	// Execute each tool call
	for _, toolCall := range p.response.ToolCalls {
		// Parse arguments
		var args map[string]any
		if err := json.Unmarshal([]byte(toolCall.Arguments), &args); err != nil {
			p.ctx.GetLogger().Errorf("Failed to parse tool arguments: %v", err)
			p.ctx.GetSession().AddMessage(messages.ChatMessage{
				Role:       messages.MessageRoleTool,
				Content:    fmt.Sprintf("Error parsing arguments: %v", err),
				ToolCallID: toolCall.ID,
			})
			continue
		}

		// Create a logger with tool context
		toolLogger := core.WithTool(p.ctx.GetLogger(), toolCall.Name, args)

		// Get and execute tool
		tool, exists := p.registry.Get(toolCall.Name)
		if !exists {
			p.ctx.GetLogger().Warnf("Tool not found: %s", toolCall.Name)
			p.ctx.GetSession().AddMessage(messages.ChatMessage{
				Role:       messages.MessageRoleTool,
				Content:    fmt.Sprintf("Tool not found: %s", toolCall.Name),
				ToolCallID: toolCall.ID,
			})
			continue
		}
		// Show action for tool call if enabled and not the irc_action tool
		if p.ctx.GetConfig().Bot.ShowToolActions && toolCall.Name != "irc_action" {
			// Strip namespace prefix for display (e.g., "script__weather" -> "weather")
			displayName := toolCall.Name
			if idx := strings.Index(displayName, "__"); idx != -1 {
				displayName = displayName[idx+2:]
			}
			message := fmt.Sprintf("calling %s", displayName)
			if p.ctx.IsPrivate() {
				// For private messages, send a regular reply
				p.ctx.Reply(message)
			} else {
				// For channels, use an action
				channel := p.ctx.GetConfig().Server.Channel
				p.ctx.Action(channel, message)
			}
		}

		// Set context for contextual tools (IRC tools)
		// We now pass this via the context.Context below
		ctx = context.WithValue(ctx, kContextKey, p.ctx)

		// Execute tool with timing
		startTime := time.Now()
		toolLogger.Info("Executing tool")
		result, err := tool.Execute(ctx, args)
		duration := time.Since(startTime)

		if err != nil {
			result = fmt.Sprintf("Error: %v", err)
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

		// Add tool result to session
		p.ctx.GetSession().AddMessage(messages.ChatMessage{
			Role:       messages.MessageRoleTool,
			Content:    result,
			ToolCallID: toolCall.ID,
		})
	}

	// Continue conversation with tool results
	req.Messages = p.ctx.GetSession().GetHistory()
	// Restore the original model name (MultiPass strips the provider prefix)
	req.Model = p.originalModel

	// Reset thinking action flag and timer for continuation
	p.sentThinkingAction = false
	p.thinkingStartTime = nil
	if p.thinkingTimer != nil {
		p.thinkingTimer.Stop()
		p.thinkingTimer = nil
	}

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
		chunk := p.extractChunk()
		if chunk != nil {
			p.byteChan <- chunk
		}
	}
}

// extractChunk extracts a properly sized chunk, breaking on spaces when possible
func (p *IRCEventProcessor) extractChunk() []byte {
	if p.chunkBuffer.Len() == 0 {
		return nil
	}

	data := p.chunkBuffer.Bytes()
	end := min(p.maxChunkSize, len(data))

	// Try to break on space
	if idx := bytes.LastIndexByte(data[:end], ' '); idx > 0 {
		chunk := make([]byte, idx)
		copy(chunk, data[:idx])
		p.chunkBuffer.Next(idx + 1) // Skip the space
		return chunk
	}

	// Just break at max size
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
