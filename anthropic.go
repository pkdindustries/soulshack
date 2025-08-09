package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
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

func (a *AnthropicClient) ChatCompletionTask(ctx context.Context, req *CompletionRequest, chunker *Chunker) (<-chan []byte, <-chan *ToolCall, <-chan *ChatMessage) {
	messageChannel := make(chan ChatMessage, 10)

	go func() {
		defer close(messageChannel)

		// Convert messages to Anthropic format
		messages, systemPrompt := MessagesToAnthropicParams(req.Session.GetHistory())

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
			messageChannel <- ChatMessage{
				Role:    MessageRoleAssistant,
				Content: "Error communicating with Anthropic: " + err.Error(),
			}
			return
		}

		// Extract the response content and any tool calls
		var (
			responseContent string
			toolCalls       []ChatMessageToolCall
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
				// Forward as an agnostic tool call
				toolCalls = append(toolCalls, ChatMessageToolCall{
					ID:        cb.ID,
					Name:      cb.Name,
					Arguments: string(cb.Input),
				})
			}
		}

		// Send the response message including any tool calls for execution.
		msg := ChatMessage{
			Role:      MessageRoleAssistant,
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
				toolInfo[i] = fmt.Sprintf("%s(%s)", tc.Name, tc.Arguments)
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
