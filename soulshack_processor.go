package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/alexschlessinger/pollytool/llm"
	"github.com/alexschlessinger/pollytool/messages"
	"github.com/alexschlessinger/pollytool/tools"
)

// SoulshackStreamProcessor handles streaming LLM responses with tool execution and IRC chunking
type SoulshackStreamProcessor struct {
	ctx          ChatContextInterface
	maxChunkSize int
	chunkBuffer  *bytes.Buffer
	registry     *tools.ToolRegistry
	client       llm.LLM
}

// NewSoulshackStreamProcessor creates a processor for soulshack
func NewSoulshackStreamProcessor(ctx ChatContextInterface, maxChunkSize int, registry *tools.ToolRegistry, client llm.LLM) *SoulshackStreamProcessor {
	return &SoulshackStreamProcessor{
		ctx:          ctx,
		maxChunkSize: maxChunkSize,
		chunkBuffer:  &bytes.Buffer{},
		registry:     registry,
		client:       client,
	}
}

// ProcessCompletionStream handles a completion request and outputs chunked bytes for IRC
func (s *SoulshackStreamProcessor) ProcessCompletionStream(ctx context.Context, req *llm.CompletionRequest) <-chan []byte {
	byteChan := make(chan []byte, 10)

	go func() {
		defer close(byteChan)
		defer s.flushBuffer(byteChan)

		// Use SimpleProcessor from pollytool
		processor := &llm.SimpleProcessor{}
		eventChan := s.client.ChatCompletionStream(ctx, req, processor)

		s.processEvents(ctx, eventChan, req, byteChan)
	}()

	return byteChan
}

// processEvents handles the event stream, executes tools, and outputs chunked bytes
func (s *SoulshackStreamProcessor) processEvents(ctx context.Context, eventChan <-chan *messages.StreamEvent, req *llm.CompletionRequest, byteChan chan<- []byte) {
	for event := range eventChan {
		switch event.Type {
		case messages.EventTypeContent:
			// Stream content through chunking
			s.processContent(event.Content, byteChan)

		case messages.EventTypeToolCall:
			// Tool calls come through the Complete event with the full message

		case messages.EventTypeComplete:
			if event.Message != nil {
				// Add the assistant message to session
				s.ctx.GetSession().AddMessage(*event.Message)

				// If message has content, ensure it's output
				if event.Message.Content != "" && len(event.Message.ToolCalls) == 0 {
					// Content-only messages are already streamed via EventTypeContent
					// But flush any remaining buffer
					s.flushBuffer(byteChan)
				}

				// If message has tool calls, execute them and continue
				if len(event.Message.ToolCalls) > 0 {
					s.handleToolCallsAndContinue(ctx, event.Message, req, byteChan)
				}
			}

		case messages.EventTypeError:
			if event.Error != nil {
				errMsg := fmt.Sprintf("Error: %v", event.Error)
				s.processContent(errMsg, byteChan)
			}
		}
	}
}

// handleToolCallsAndContinue executes tools and continues the conversation
func (s *SoulshackStreamProcessor) handleToolCallsAndContinue(ctx context.Context, msg *messages.ChatMessage, req *llm.CompletionRequest, byteChan chan<- []byte) {
	// Execute each tool call
	for _, toolCall := range msg.ToolCalls {
		// Parse arguments
		var args map[string]any
		if err := json.Unmarshal([]byte(toolCall.Arguments), &args); err != nil {
			log.Printf("Failed to parse tool arguments: %v", err)
			s.ctx.GetSession().AddMessage(messages.ChatMessage{
				Role:       messages.MessageRoleTool,
				Content:    fmt.Sprintf("Error parsing arguments: %v", err),
				ToolCallID: toolCall.ID,
			})
			continue
		}

		// Get and execute tool
		tool, exists := s.registry.Get(toolCall.Name)
		if !exists {
			log.Printf("Tool not found: %s", toolCall.Name)
			s.ctx.GetSession().AddMessage(messages.ChatMessage{
				Role:       messages.MessageRoleTool,
				Content:    fmt.Sprintf("Tool not found: %s", toolCall.Name),
				ToolCallID: toolCall.ID,
			})
			continue
		}

		// Set context for contextual tools (IRC tools)
		if contextualTool, ok := tool.(tools.ContextualTool); ok {
			contextualTool.SetContext(s.ctx)
		}

		// Execute tool
		log.Printf("Executing tool: %s(%v)", toolCall.Name, args)
		result, err := tool.Execute(ctx, args)
		if err != nil {
			result = fmt.Sprintf("Error: %v", err)
		}

		// Log tool output (truncate if too long)
		outputPreview := result
		if len(outputPreview) > 200 {
			outputPreview = outputPreview[:200] + "..."
		}
		log.Printf("Tool %s output: %s", toolCall.Name, outputPreview)

		// Add tool result to session
		s.ctx.GetSession().AddMessage(messages.ChatMessage{
			Role:       messages.MessageRoleTool,
			Content:    result,
			ToolCallID: toolCall.ID,
		})
	}

	// Continue conversation with tool results
	req.Messages = s.ctx.GetSession().GetHistory()
	processor := &llm.SimpleProcessor{}
	eventChan := s.client.ChatCompletionStream(ctx, req, processor)

	// Process the continuation recursively
	s.processEvents(ctx, eventChan, req, byteChan)
}

// processContent handles chunking of content for IRC message limits
func (s *SoulshackStreamProcessor) processContent(content string, byteChan chan<- []byte) {
	s.chunkBuffer.WriteString(content)

	// Emit complete lines immediately
	for {
		line, err := s.chunkBuffer.ReadString('\n')
		if err != nil {
			// No more complete lines, put back what we read
			if line != "" {
				s.chunkBuffer.WriteString(line)
			}
			break
		}
		// Remove the newline and send
		if line = line[:len(line)-1]; line != "" {
			byteChan <- []byte(line)
		}
	}

	// If buffer is getting too large, force a chunk
	if s.chunkBuffer.Len() >= s.maxChunkSize {
		chunk := s.extractChunk()
		if chunk != nil {
			byteChan <- chunk
		}
	}
}

// extractChunk extracts a properly sized chunk, breaking on spaces when possible
func (s *SoulshackStreamProcessor) extractChunk() []byte {
	if s.chunkBuffer.Len() == 0 {
		return nil
	}

	data := s.chunkBuffer.Bytes()
	end := s.maxChunkSize
	if end > len(data) {
		end = len(data)
	}

	// Try to break on space
	if idx := bytes.LastIndexByte(data[:end], ' '); idx > 0 {
		chunk := make([]byte, idx)
		copy(chunk, data[:idx])
		s.chunkBuffer.Next(idx + 1) // Skip the space
		return chunk
	}

	// Just break at max size
	chunk := make([]byte, end)
	copy(chunk, data[:end])
	s.chunkBuffer.Next(end)
	return chunk
}

// flushBuffer sends any remaining content in the buffer
func (s *SoulshackStreamProcessor) flushBuffer(byteChan chan<- []byte) {
	if s.chunkBuffer.Len() > 0 {
		byteChan <- s.chunkBuffer.Bytes()
		s.chunkBuffer.Reset()
	}
}