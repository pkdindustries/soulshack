package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	ai "github.com/sashabaranov/go-openai"
)

type AnthropicClient struct {
	client anthropic.Client
}

func NewAnthropicClient(config APIConfig) *AnthropicClient {
	if config.AnthropicKey == "" {
		log.Println("anthropic: warning - no API key configured")
	}

	client := anthropic.NewClient(
		option.WithAPIKey(config.AnthropicKey),
	)

	return &AnthropicClient{
		client: client,
	}
}

func (a *AnthropicClient) ChatCompletionTask(ctx context.Context, req *CompletionRequest, chunker *Chunker) (<-chan []byte, <-chan *ToolCall, <-chan *ai.ChatCompletionMessage) {
	messageChannel := make(chan ai.ChatCompletionMessage, 10)

	go func() {
		defer close(messageChannel)

		// Convert messages to Anthropic format
		var messages []anthropic.MessageParam
		systemPrompt := ""

		for _, msg := range req.Session.GetHistory() {
			switch msg.Role {
			case ai.ChatMessageRoleSystem:
				// Anthropic handles system messages separately
				systemPrompt = msg.Content

			case ai.ChatMessageRoleUser:
				// Plain user text
				if strings.TrimSpace(msg.Content) != "" {
					messages = append(messages, anthropic.NewUserMessage(
						anthropic.NewTextBlock(msg.Content),
					))
				}

			case ai.ChatMessageRoleAssistant:
				// Build a single assistant message that can include text and tool_use blocks
				var blocks []anthropic.ContentBlockParamUnion
				if strings.TrimSpace(msg.Content) != "" {
					blocks = append(blocks, anthropic.NewTextBlock(msg.Content))
				}
				// Rehydrate prior tool_use blocks so Anthropic can correlate tool_result
				if len(msg.ToolCalls) > 0 {
					for _, tc := range msg.ToolCalls {
						// Parse arguments JSON into an object for the SDK
						var input interface{}
						if argStr := strings.TrimSpace(tc.Function.Arguments); argStr != "" {
							var tmp interface{}
							if err := json.Unmarshal([]byte(argStr), &tmp); err == nil {
								input = tmp
							}
						}
						blocks = append(blocks, anthropic.NewToolUseBlock(tc.ID, input, tc.Function.Name))
					}
				}
				if len(blocks) > 0 {
					messages = append(messages, anthropic.NewAssistantMessage(blocks...))
				}

			case ai.ChatMessageRoleTool:
				// Send tool results back as tool_result blocks tied to the initiating tool_use id
				// so Claude can consume them and continue the conversation.
				if strings.TrimSpace(msg.ToolCallID) != "" {
					messages = append(messages, anthropic.NewUserMessage(
						anthropic.NewToolResultBlock(msg.ToolCallID, msg.Content, false),
					))
				} else if strings.TrimSpace(msg.Content) != "" {
					// Fallback to plain text if we don't have an id to bind to
					messages = append(messages, anthropic.NewUserMessage(
						anthropic.NewTextBlock(msg.Content),
					))
				}
			}
		}

		// Create the request
		params := anthropic.MessageNewParams{
			Model:       anthropic.Model(req.Model),
			MaxTokens:   int64(req.MaxTokens),
			Temperature: anthropic.Float(float64(req.Temperature)),
			Messages:    messages,
		}
		
		// Opus models don't support both temperature and top_p simultaneously
		if !strings.Contains(strings.ToLower(req.Model), "opus") {
			params.TopP = anthropic.Float(float64(req.TopP))
		}

		// Add system prompt if present
		if systemPrompt != "" {
			// System messages in Anthropic SDK are handled as TextBlockParam
			params.System = []anthropic.TextBlockParam{
				{
					Type: "text",
					Text: systemPrompt,
				},
			}
		}

		// Add tool support if available
		if len(req.Tools) > 0 {
			var anthropicTools []anthropic.ToolUnionParam
			for _, tool := range req.Tools {
				anthropicTools = append(anthropicTools, ConvertToAnthropic(tool.GetSchema()))
			}
			params.Tools = anthropicTools
		}

		log.Printf("anthropic: sending request to model %s", req.Model)

		// Make the API call
		message, err := a.client.Messages.New(ctx, params)
		if err != nil {
			log.Printf("anthropic: API error: %v", err)
			messageChannel <- ai.ChatCompletionMessage{
				Role:    ai.ChatMessageRoleAssistant,
				Content: "Error communicating with Anthropic: " + err.Error(),
			}
			return
		}

		// Extract the response content and any tool calls
		var (
			responseContent string
			toolCalls       []ai.ToolCall
		)

		// The SDK returns a union of content blocks. Marshal each block to
		// JSON and inspect its "type" to robustly handle text and tool_use
		// without relying on internal SDK struct shapes.
		for _, block := range message.Content {
			b, err := json.Marshal(block)
			if err != nil {
				continue
			}

			// Minimal structure to decode union fields we care about.
			var cb struct {
				Type  string          `json:"type"`
				Text  string          `json:"text,omitempty"`
				Name  string          `json:"name,omitempty"`
				ID    string          `json:"id,omitempty"`
				Input json.RawMessage `json:"input,omitempty"`
			}
			if err := json.Unmarshal(b, &cb); err != nil {
				continue
			}

			switch cb.Type {
			case "text":
				responseContent += cb.Text
			case "tool_use":
				// Forward as an OpenAI-style tool call so the rest of the
				// pipeline can remain provider-agnostic.
				toolCalls = append(toolCalls, ai.ToolCall{
					ID:   cb.ID,
					Type: ai.ToolTypeFunction,
					Function: ai.FunctionCall{
						Name:      cb.Name,
						Arguments: string(cb.Input),
					},
				})
			}
		}

		// Send the response message including any tool calls for execution.
		msg := ai.ChatCompletionMessage{
			Role:      ai.ChatMessageRoleAssistant,
			Content:   responseContent,
			ToolCalls: toolCalls,
		}

		messageChannel <- msg

		// Log detailed response information
		contentPreview := responseContent
		if len(contentPreview) > 200 {
			contentPreview = contentPreview[:200] + "..."
		}
		
		if len(toolCalls) > 0 {
			toolInfo := make([]string, len(toolCalls))
			for i, tc := range toolCalls {
				toolInfo[i] = fmt.Sprintf("%s(%s)", tc.Function.Name, tc.Function.Arguments)
			}
			log.Printf("anthropic: completed, content: '%s' (%d chars), tool calls: %d %v", 
				contentPreview, len(responseContent), len(toolCalls), toolInfo)
		} else if len(responseContent) == 0 {
			log.Printf("anthropic: completed, empty response (no content or tool calls)")
		} else {
			log.Printf("anthropic: completed, content: '%s' (%d chars)", 
				contentPreview, len(responseContent))
		}
	}()

	return chunker.ProcessMessages(messageChannel)
}
