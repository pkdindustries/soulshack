package main

import (
	"encoding/json"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/google/generative-ai-go/genai"
	ollamaapi "github.com/ollama/ollama/api"
	ai "github.com/sashabaranov/go-openai"
)

// OpenAI conversions

// MessageFromOpenAI converts an OpenAI message to our agnostic format
func MessageFromOpenAI(msg ai.ChatCompletionMessage) ChatMessage {
	m := ChatMessage{
		Role:       msg.Role,
		Content:    msg.Content,
		ToolCallID: msg.ToolCallID,
	}
	
	for _, tc := range msg.ToolCalls {
		m.ToolCalls = append(m.ToolCalls, ChatMessageToolCall{
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: tc.Function.Arguments,
		})
	}
	
	return m
}

// MessageToOpenAI converts our agnostic message to OpenAI format
func MessageToOpenAI(msg ChatMessage) ai.ChatCompletionMessage {
	m := ai.ChatCompletionMessage{
		Role:       msg.Role,
		Content:    msg.Content,
		ToolCallID: msg.ToolCallID,
	}
	
	for _, tc := range msg.ToolCalls {
		m.ToolCalls = append(m.ToolCalls, ai.ToolCall{
			ID:   tc.ID,
			Type: ai.ToolTypeFunction,
			Function: ai.FunctionCall{
				Name:      tc.Name,
				Arguments: tc.Arguments,
			},
		})
	}
	
	return m
}

// MessagesToOpenAI converts a slice of agnostic messages to OpenAI format
func MessagesToOpenAI(messages []ChatMessage) []ai.ChatCompletionMessage {
	result := make([]ai.ChatCompletionMessage, len(messages))
	for i, msg := range messages {
		result[i] = MessageToOpenAI(msg)
	}
	return result
}

// Anthropic conversions

// MessagesToAnthropicParams converts messages to Anthropic message parameters
func MessagesToAnthropicParams(messages []ChatMessage) ([]anthropic.MessageParam, string) {
	var anthropicMessages []anthropic.MessageParam
	systemPrompt := ""
	
	for _, msg := range messages {
		switch msg.Role {
		case MessageRoleSystem:
			systemPrompt = msg.Content
			
		case MessageRoleUser:
			if strings.TrimSpace(msg.Content) != "" {
				anthropicMessages = append(anthropicMessages, anthropic.NewUserMessage(
					anthropic.NewTextBlock(msg.Content),
				))
			}
			
		case MessageRoleAssistant:
			var blocks []anthropic.ContentBlockParamUnion
			if strings.TrimSpace(msg.Content) != "" {
				blocks = append(blocks, anthropic.NewTextBlock(msg.Content))
			}
			if len(msg.ToolCalls) > 0 {
				for _, tc := range msg.ToolCalls {
					var input interface{}
					if argStr := strings.TrimSpace(tc.Arguments); argStr != "" {
						var tmp interface{}
						if err := json.Unmarshal([]byte(argStr), &tmp); err == nil {
							input = tmp
						}
					}
					blocks = append(blocks, anthropic.NewToolUseBlock(tc.ID, input, tc.Name))
				}
			}
			if len(blocks) > 0 {
				anthropicMessages = append(anthropicMessages, anthropic.NewAssistantMessage(blocks...))
			}
			
		case MessageRoleTool:
			if strings.TrimSpace(msg.ToolCallID) != "" {
				anthropicMessages = append(anthropicMessages, anthropic.NewUserMessage(
					anthropic.NewToolResultBlock(msg.ToolCallID, msg.Content, false),
				))
			} else if strings.TrimSpace(msg.Content) != "" {
				anthropicMessages = append(anthropicMessages, anthropic.NewUserMessage(
					anthropic.NewTextBlock(msg.Content),
				))
			}
		}
	}
	
	return anthropicMessages, systemPrompt
}


// Gemini conversions

// MessagesToGeminiContent converts messages to Gemini content format
func MessagesToGeminiContent(messages []ChatMessage) ([]*genai.Content, string, map[string]string) {
	var history []*genai.Content
	var systemInstruction string
	callIDToName := make(map[string]string)
	
	for _, msg := range messages {
		switch msg.Role {
		case MessageRoleSystem:
			systemInstruction = msg.Content
			
		case MessageRoleUser:
			history = append(history, genai.NewUserContent(genai.Text(msg.Content)))
			
		case MessageRoleAssistant:
			var parts []genai.Part
			if msg.Content != "" {
				parts = append(parts, genai.Text(msg.Content))
			}
			if len(msg.ToolCalls) > 0 {
				for _, tc := range msg.ToolCalls {
					if tc.ID != "" {
						callIDToName[tc.ID] = tc.Name
					}
					var args map[string]interface{}
					if err := json.Unmarshal([]byte(tc.Arguments), &args); err == nil {
						parts = append(parts, genai.FunctionCall{
							Name: tc.Name,
							Args: args,
						})
					}
				}
			}
			if len(parts) > 0 {
				history = append(history, &genai.Content{
					Role:  "model",
					Parts: parts,
				})
			}
			
		case MessageRoleTool:
			funcName := ""
			if msg.ToolCallID != "" {
				funcName = callIDToName[msg.ToolCallID]
			}
			
			var output interface{}
			if err := json.Unmarshal([]byte(msg.Content), &output); err != nil {
				output = msg.Content
			}
			
			// Ensure output is a map[string]any as required by genai
			var response map[string]any
			if m, ok := output.(map[string]any); ok {
				response = m
			} else {
				response = map[string]any{"result": output}
			}
			history = append(history, genai.NewUserContent(genai.FunctionResponse{
				Name:     funcName,
				Response: response,
			}))
		}
	}
	
	return history, systemInstruction, callIDToName
}

// MessageFromGeminiCandidate converts a Gemini candidate to our message format
func MessageFromGeminiCandidate(candidate *genai.Candidate) ChatMessage {
	msg := ChatMessage{
		Role: MessageRoleAssistant,
	}
	
	if candidate.Content != nil {
		for _, part := range candidate.Content.Parts {
			switch p := part.(type) {
			case genai.Text:
				msg.Content += string(p)
			case genai.FunctionCall:
				argsJSON, _ := json.Marshal(p.Args)
				msg.ToolCalls = append(msg.ToolCalls, ChatMessageToolCall{
					ID:        "", // Gemini doesn't use IDs
					Name:      p.Name,
					Arguments: string(argsJSON),
				})
			}
		}
	}
	
	return msg
}

// Ollama conversions

// MessagesToOllama converts messages to Ollama format
func MessagesToOllama(messages []ChatMessage) []ollamaapi.Message {
	var ollamaMessages []ollamaapi.Message
	
	for _, msg := range messages {
		ollamaMsg := ollamaapi.Message{
			Role:    msg.Role,
			Content: msg.Content,
		}
		
		if msg.Role == MessageRoleAssistant && len(msg.ToolCalls) > 0 {
			var ollamaToolCalls []ollamaapi.ToolCall
			for _, tc := range msg.ToolCalls {
				var args map[string]interface{}
				if err := json.Unmarshal([]byte(tc.Arguments), &args); err == nil {
					ollamaToolCalls = append(ollamaToolCalls, ollamaapi.ToolCall{
						Function: ollamaapi.ToolCallFunction{
							Name:      tc.Name,
							Arguments: args,
						},
					})
				}
			}
			ollamaMsg.ToolCalls = ollamaToolCalls
		}
		
		ollamaMessages = append(ollamaMessages, ollamaMsg)
	}
	
	return ollamaMessages
}

// MessageFromOllama converts an Ollama message to our agnostic format
func MessageFromOllama(msg ollamaapi.Message) ChatMessage {
	m := ChatMessage{
		Role:    msg.Role,
		Content: msg.Content,
	}
	
	for _, tc := range msg.ToolCalls {
		argsJSON, _ := json.Marshal(tc.Function.Arguments)
		m.ToolCalls = append(m.ToolCalls, ChatMessageToolCall{
			ID:        "", // Ollama doesn't have tool call IDs
			Name:      tc.Function.Name,
			Arguments: string(argsJSON),
		})
	}
	
	return m
}