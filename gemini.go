package main

import (
	"context"
	"encoding/json"
	"fmt"
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

func (g *GeminiClient) ChatCompletionTask(ctx context.Context, req *CompletionRequest, chunker *Chunker) (<-chan []byte, <-chan *ToolCall, <-chan *ai.ChatCompletionMessage) {
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

		// Convert session history to Gemini chat history and prepare the last user prompt parts
		var (
			history           []*genai.Content
			userParts         []genai.Part
			systemInstruction string
		)

		msgs := req.Session.GetHistory()
		// Map ToolCallID -> function name for pairing tool responses
		callIDToName := make(map[string]string)

		for i, msg := range msgs {
			isLast := i == len(msgs)-1
			switch msg.Role {
			case ai.ChatMessageRoleSystem:
				systemInstruction = msg.Content

			case ai.ChatMessageRoleUser:
				if isLast {
					userParts = append(userParts, genai.Text(msg.Content))
				} else {
					history = append(history, genai.NewUserContent(genai.Text(msg.Content)))
				}

			case ai.ChatMessageRoleAssistant:
				// Build model content, including any function calls
				var parts []genai.Part
				if msg.Content != "" {
					parts = append(parts, genai.Text(msg.Content))
				}
				if len(msg.ToolCalls) > 0 {
					for _, tc := range msg.ToolCalls {
						// Record name by ID for later tool response pairing
						if tc.ID != "" {
							callIDToName[tc.ID] = tc.Function.Name
						}
						// Parse arguments JSON
						var args map[string]interface{}
						if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err == nil {
							parts = append(parts, genai.FunctionCall{Name: tc.Function.Name, Args: args})
						} else {
							parts = append(parts, genai.Text("[function call args parse error]"))
						}
					}
				}
				if len(parts) == 0 {
					// Ensure we still represent assistant turn, even if empty
					parts = append(parts, genai.Text(""))
				}
				history = append(history, &genai.Content{Role: "model", Parts: parts})

			case ai.ChatMessageRoleTool:
				// Convert tool response into a FunctionResponse part as a user turn
				name := callIDToName[msg.ToolCallID]
				if name == "" {
					// Fallback to plain user content if we cannot map the tool call
					history = append(history, genai.NewUserContent(genai.Text(msg.Content)))
					continue
				}

				// Try to parse tool content as JSON; otherwise wrap in {result: ...}
				var respObj map[string]any
				var tmp any
				if err := json.Unmarshal([]byte(msg.Content), &tmp); err == nil {
					if m, ok := tmp.(map[string]any); ok {
						respObj = m
					} else {
						respObj = map[string]any{"result": tmp}
					}
				} else {
					respObj = map[string]any{"result": msg.Content}
				}
				history = append(history, &genai.Content{Role: "user", Parts: []genai.Part{genai.FunctionResponse{Name: name, Response: respObj}}})
			}
		}

		// Set system instruction, augmenting with tool usage guidance if tools are enabled.
		if systemInstruction != "" {
			if req.ToolsEnabled && len(req.Tools) > 0 {
				// Brief, provider-neutral nudge for tool use.
				toolInfo := "\n\nYou have access to function tools. Prefer calling a tool when it can fulfill a user request."
				systemInstruction += toolInfo
			}
			model.SystemInstruction = &genai.Content{Parts: []genai.Part{genai.Text(systemInstruction)}}
		} else if req.ToolsEnabled && len(req.Tools) > 0 {
			// No system message present; still provide minimal instruction so Gemini is aware.
			model.SystemInstruction = &genai.Content{Parts: []genai.Part{genai.Text("Use available function tools when appropriate.")}}
		}

		// Add tool support if enabled
		if req.ToolsEnabled && len(req.Tools) > 0 {
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
			messageChannel <- ai.ChatCompletionMessage{
				Role:    ai.ChatMessageRoleAssistant,
				Content: "Error communicating with Gemini: " + err.Error(),
			}
			return
		}

		// Extract response content
		var (
			responseContent string
			toolCalls       []ai.ToolCall
		)

		for _, candidate := range resp.Candidates {
			if candidate.Content != nil {
				for _, part := range candidate.Content.Parts {
					if text, ok := part.(genai.Text); ok {
						responseContent += string(text)
					}
				}
				// Extract function calls from candidate
				for idx, fc := range candidate.FunctionCalls() {
					// Convert args to JSON string
					argsJSON, _ := json.Marshal(fc.Args)
					toolCalls = append(toolCalls, ai.ToolCall{
						ID:   fmt.Sprintf("gemini-%d", idx),
						Type: ai.ToolTypeFunction,
						Function: ai.FunctionCall{
							Name:      fc.Name,
							Arguments: string(argsJSON),
						},
					})
				}
			}
		}

		// Send the response
		msg := ai.ChatCompletionMessage{
			Role:      ai.ChatMessageRoleAssistant,
			Content:   responseContent,
			ToolCalls: toolCalls,
		}

		messageChannel <- msg

		log.Printf("gemini: completed, response length: %d", len(responseContent))
	}()

	return chunker.ProcessMessages(messageChannel)
}
