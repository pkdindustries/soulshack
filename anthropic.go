package main

import (
	"context"
	"log"

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

func (a *AnthropicClient) ChatCompletionTask(ctx context.Context, req *CompletionRequest, chunker *Chunker) (<-chan []byte, <-chan *ai.ToolCall, <-chan *ai.ChatCompletionMessage) {
	messageChannel := make(chan ai.ChatCompletionMessage, 10)

	go func() {
		defer close(messageChannel)

		// Convert messages to Anthropic format
		var messages []anthropic.MessageParam
		systemPrompt := ""

		for _, msg := range req.Session.GetHistory() {
			switch msg.Role {
			case ai.ChatMessageRoleSystem:
				// Anthropic handles system messages differently
				systemPrompt = msg.Content
			case ai.ChatMessageRoleUser:
				messages = append(messages, anthropic.NewUserMessage(
					anthropic.NewTextBlock(msg.Content),
				))
			case ai.ChatMessageRoleAssistant:
				messages = append(messages, anthropic.NewAssistantMessage(
					anthropic.NewTextBlock(msg.Content),
				))
			}
		}

		// Create the request
		params := anthropic.MessageNewParams{
			Model:       anthropic.Model(req.Model),
			MaxTokens:   int64(req.MaxTokens),
			Temperature: anthropic.Float(float64(req.Temperature)),
			TopP:        anthropic.Float(float64(req.TopP)),
			Messages:    messages,
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

		// Tool support would require more complex conversion
		// Skipping for now as it requires proper schema mapping

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

		// Extract the response content
		var responseContent string

		for _, content := range message.Content {
			// Use the AsText method to get text content
			if content.Type == "text" {
				responseContent += content.Text
			}
		}

		// Send the response
		msg := ai.ChatCompletionMessage{
			Role:    ai.ChatMessageRoleAssistant,
			Content: responseContent,
		}

		messageChannel <- msg

		log.Printf("anthropic: completed, response length: %d", len(responseContent))
	}()

	return chunker.ProcessMessages(messageChannel)
}