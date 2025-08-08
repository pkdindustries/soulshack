package main

import (
	"context"
	"log"

	"github.com/google/generative-ai-go/genai"
	ai "github.com/sashabaranov/go-openai"
	"google.golang.org/api/option"
)

type GeminiClient struct {
	apiKey string
}

func NewGeminiClient(config APIConfig) *GeminiClient {
	if config.GeminiKey == "" {
		log.Println("gemini: warning - no API key configured")
	}

	return &GeminiClient{
		apiKey: config.GeminiKey,
	}
}

func (g *GeminiClient) ChatCompletionTask(ctx context.Context, req *CompletionRequest, chunker *Chunker) (<-chan []byte, <-chan *ai.ToolCall, <-chan *ai.ChatCompletionMessage) {
	messageChannel := make(chan ai.ChatCompletionMessage, 10)

	go func() {
		defer close(messageChannel)

		if g.apiKey == "" {
			messageChannel <- ai.ChatCompletionMessage{
				Role:    ai.ChatMessageRoleAssistant,
				Content: "Error: Gemini API key not configured",
			}
			return
		}

		// Create client with API key
		client, err := genai.NewClient(ctx, option.WithAPIKey(g.apiKey))
		if err != nil {
			log.Printf("gemini: failed to create client: %v", err)
			messageChannel <- ai.ChatCompletionMessage{
				Role:    ai.ChatMessageRoleAssistant,
				Content: "Error creating Gemini client: " + err.Error(),
			}
			return
		}
		defer client.Close()

		// Get the model
		model := client.GenerativeModel(req.Model)

		// Configure model parameters
		model.SetTemperature(req.Temperature)
		model.SetTopP(req.TopP)
		model.SetMaxOutputTokens(int32(req.MaxTokens))

		// Convert messages to Gemini format
		var parts []genai.Part
		var systemInstruction string

		for _, msg := range req.Session.GetHistory() {
			switch msg.Role {
			case ai.ChatMessageRoleSystem:
				// Gemini handles system instructions separately
				systemInstruction = msg.Content
			case ai.ChatMessageRoleUser:
				parts = append(parts, genai.Text(msg.Content))
			case ai.ChatMessageRoleAssistant:
				// In Gemini, we need to structure this as a conversation
				// For now, we'll treat assistant messages as context
				parts = append(parts, genai.Text("Assistant: "+msg.Content))
			}
		}

		// Set system instruction if present
		if systemInstruction != "" {
			model.SystemInstruction = &genai.Content{
				Parts: []genai.Part{genai.Text(systemInstruction)},
			}
		}

		// Tool support would require more complex conversion
		// Skipping for now as it requires proper schema mapping

		log.Printf("gemini: sending request to model %s", req.Model)

		// Start chat session
		cs := model.StartChat()

		// Generate response
		resp, err := cs.SendMessage(ctx, parts...)
		if err != nil {
			log.Printf("gemini: API error: %v", err)
			messageChannel <- ai.ChatCompletionMessage{
				Role:    ai.ChatMessageRoleAssistant,
				Content: "Error communicating with Gemini: " + err.Error(),
			}
			return
		}

		// Extract response content
		var responseContent string

		for _, candidate := range resp.Candidates {
			if candidate.Content != nil {
				for _, part := range candidate.Content.Parts {
					if text, ok := part.(genai.Text); ok {
						responseContent += string(text)
					}
				}
			}
		}

		// Send the response
		msg := ai.ChatCompletionMessage{
			Role:    ai.ChatMessageRoleAssistant,
			Content: responseContent,
		}

		messageChannel <- msg

		log.Printf("gemini: completed, response length: %d", len(responseContent))
	}()

	return chunker.ProcessMessages(messageChannel)
}