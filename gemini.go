package main

import (
	"context"
	"fmt"
	"log"

	"github.com/google/generative-ai-go/genai"
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

func (g *GeminiClient) ChatCompletionTask(ctx context.Context, req *CompletionRequest, chunker *Chunker) (<-chan []byte, <-chan *ToolCall, <-chan *ChatMessage) {
	messageChannel := make(chan ChatMessage, 10)

	go func() {
		defer close(messageChannel)

		if g.apiKey == "" {
			messageChannel <- ChatMessage{
				Role:    MessageRoleAssistant,
				Content: "Error: Gemini API key not configured",
			}
			return
		}

		// Create client with API key
		client, err := genai.NewClient(ctx, option.WithAPIKey(g.apiKey))
		if err != nil {
			log.Printf("gemini: failed to create client: %v", err)
			messageChannel <- ChatMessage{
				Role:    MessageRoleAssistant,
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

		// Convert session history to Gemini chat history
		msgs := req.Session.GetHistory()
		history, systemInstruction, _ := MessagesToGeminiContent(msgs)
		
		// Extract the last user message parts if present
		var userParts []genai.Part
		if len(msgs) > 0 {
			lastMsg := msgs[len(msgs)-1]
			if lastMsg.Role == MessageRoleUser {
				userParts = append(userParts, genai.Text(lastMsg.Content))
				// Remove last from history since we'll send it separately
				if len(history) > 0 && history[len(history)-1].Role == "user" {
					history = history[:len(history)-1]
				}
			} else if lastMsg.Role == MessageRoleAssistant {
				// Handle assistant message with tool calls as last message
				if len(history) > 0 && history[len(history)-1].Role == "model" {
					userParts = history[len(history)-1].Parts
					history = history[:len(history)-1]
				}
			} else if lastMsg.Role == MessageRoleTool {
				// Handle tool response as last message
				if len(history) > 0 && history[len(history)-1].Role == "user" {
					userParts = history[len(history)-1].Parts
					history = history[:len(history)-1]
				}
			}
		}

		// Set system instruction, augmenting with tool usage guidance if tools are available.
		if systemInstruction != "" {
			if len(req.Tools) > 0 {
				// Brief, provider-neutral nudge for tool use.
				toolInfo := "\n\nYou have access to function tools. Prefer calling a tool when it can fulfill a user request."
				systemInstruction += toolInfo
			}
			model.SystemInstruction = &genai.Content{Parts: []genai.Part{genai.Text(systemInstruction)}}
		} else if len(req.Tools) > 0 {
			// No system message present; still provide minimal instruction so Gemini is aware.
			model.SystemInstruction = &genai.Content{Parts: []genai.Part{genai.Text("Use available function tools when appropriate.")}}
		}

		// Add tool support if available
		if len(req.Tools) > 0 {
			// Gemini works best when all function declarations are grouped
			// under a single Tool entry. Aggregate all declarations into one.
			var fdecls []*genai.FunctionDeclaration
			for _, tool := range req.Tools {
				t := ConvertToGemini(tool.GetSchema())
				if t != nil && len(t.FunctionDeclarations) > 0 {
					fdecls = append(fdecls, t.FunctionDeclarations[0])
				}
			}
			if len(fdecls) > 0 {
				model.Tools = []*genai.Tool{{FunctionDeclarations: fdecls}}
				// Allow the model to use tools
				model.ToolConfig = &genai.ToolConfig{FunctionCallingConfig: &genai.FunctionCallingConfig{Mode: genai.FunctionCallingAuto}}
			}
		}

		log.Printf("gemini: sending request to model %s", req.Model)

		// Start chat session and seed history from prior turns
		cs := model.StartChat()
		if len(history) > 0 {
			cs.History = append(cs.History, history...)
		}

		// Generate response with the last user parts (may be empty if not found)
		resp, err := cs.SendMessage(ctx, userParts...)
		if err != nil {
			log.Printf("gemini: API error: %v", err)
			messageChannel <- ChatMessage{
				Role:    MessageRoleAssistant,
				Content: "Error communicating with Gemini: " + err.Error(),
			}
			return
		}

		// Extract response content and convert to agnostic format
		var msg ChatMessage
		if len(resp.Candidates) > 0 {
			msg = MessageFromGeminiCandidate(resp.Candidates[0])
			// Generate IDs for tool calls since Gemini doesn't provide them
			for idx := range msg.ToolCalls {
				msg.ToolCalls[idx].ID = fmt.Sprintf("gemini-%d", idx)
			}
		} else {
			msg = ChatMessage{
				Role:    MessageRoleAssistant,
				Content: "",
			}
		}

		messageChannel <- msg

		// Log detailed response information
		contentPreview := msg.Content
		if len(contentPreview) > 200 {
			contentPreview = contentPreview[:200] + "..."
		}
		
		if len(msg.ToolCalls) > 0 {
			toolInfo := make([]string, len(msg.ToolCalls))
			for i, tc := range msg.ToolCalls {
				toolInfo[i] = fmt.Sprintf("%s(%s)", tc.Name, tc.Arguments)
			}
			log.Printf("gemini: completed, content: '%s' (%d chars), tool calls: %d %v", 
				contentPreview, len(msg.Content), len(msg.ToolCalls), toolInfo)
		} else if len(msg.Content) == 0 {
			log.Printf("gemini: completed, empty response (no content or tool calls)")
		} else {
			log.Printf("gemini: completed, content: '%s' (%d chars)", 
				contentPreview, len(msg.Content))
		}
	}()

	return chunker.ProcessMessages(messageChannel)
}
