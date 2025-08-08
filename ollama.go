package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"

	ollamaapi "github.com/ollama/ollama/api"
	ai "github.com/sashabaranov/go-openai"
)

type OllamaClient struct {
	client *ollamaapi.Client
}

func NewOllamaClient(config APIConfig) *OllamaClient {
	// Default to localhost if not specified
	ollamaURL := config.OllamaURL
	if ollamaURL == "" {
		ollamaURL = "http://localhost:11434"
	}

	// Parse URL and create client
	u, err := url.Parse(ollamaURL)
	if err != nil {
		log.Printf("ollama: invalid URL %s: %v", ollamaURL, err)
		u, _ = url.Parse("http://localhost:11434")
	}

	client := ollamaapi.NewClient(u, http.DefaultClient)

	return &OllamaClient{
		client: client,
	}
}

// mustJSON marshals a map[string]interface{} to a JSON string, returning
// an empty object on failure. This is used to populate ai.FunctionCall.Arguments.
func mustJSON(v map[string]interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// stripThinkBlocks removes <think>...</think> blocks from the response content
func stripThinkBlocks(content string) string {
	// Match <think> blocks including nested content, using non-greedy matching
	thinkRegex := regexp.MustCompile(`(?s)<think>.*?</think>`)
	return thinkRegex.ReplaceAllString(content, "")
}

func (o *OllamaClient) ChatCompletionTask(ctx context.Context, req *CompletionRequest, chunker *Chunker) (<-chan []byte, <-chan *ToolCall, <-chan *ai.ChatCompletionMessage) {
	messageChannel := make(chan ai.ChatCompletionMessage, 10)

	go func() {
		defer close(messageChannel)

		// Convert messages to Ollama format
		var messages []ollamaapi.Message
		for _, msg := range req.Session.GetHistory() {
			ollamaMsg := ollamaapi.Message{
				Role:    msg.Role,
				Content: msg.Content,
			}
			
			// Preserve tool calls in assistant messages
			if msg.Role == ai.ChatMessageRoleAssistant && len(msg.ToolCalls) > 0 {
				var ollamaToolCalls []ollamaapi.ToolCall
				for _, tc := range msg.ToolCalls {
					// Parse the arguments back to a map
					var args map[string]interface{}
					if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err == nil {
						ollamaToolCalls = append(ollamaToolCalls, ollamaapi.ToolCall{
							Function: ollamaapi.ToolCallFunction{
								Name:      tc.Function.Name,
								Arguments: args,
							},
						})
					}
				}
				ollamaMsg.ToolCalls = ollamaToolCalls
			}
			
			messages = append(messages, ollamaMsg)
		}

		// Create chat request
		chatReq := &ollamaapi.ChatRequest{
			Model:    req.Model,
			Messages: messages,
			Options: map[string]interface{}{
				"temperature": req.Temperature,
				"top_p":       req.TopP,
				"num_predict": req.MaxTokens,
			},
		}

		// Add tool support if enabled
		if req.ToolsEnabled && len(req.Tools) > 0 {
			var ollamaTools []ollamaapi.Tool
			for _, tool := range req.Tools {
				ollamaTools = append(ollamaTools, ConvertToOllama(tool.GetSchema()))
			}
			chatReq.Tools = ollamaTools
		}

		log.Printf("ollama: chat request to model %s", req.Model)

		// Execute chat - the callback is called for each streamed chunk.
		// Accumulate content and capture any tool calls the model returns.
		var (
			responseContent string
			toolCalls       []ai.ToolCall
		)
		err := o.client.Chat(ctx, chatReq, func(resp ollamaapi.ChatResponse) error {
			// Append streamed content tokens
			if resp.Message.Content != "" {
				responseContent += resp.Message.Content
			}
			// Record tool calls if present on this chunk (found on the message)
			if len(resp.Message.ToolCalls) > 0 {
				// Reset and capture the latest set of tool calls.
				toolCalls = toolCalls[:0]
				for idx, tc := range resp.Message.ToolCalls {
					if parsed := ParseOllamaToolCall(tc); parsed != nil {
						// Convert back to OpenAI-structured tool call for the rest of the pipeline.
						toolCalls = append(toolCalls, ai.ToolCall{
							ID:   fmt.Sprintf("ollama-%d", idx),
							Type: ai.ToolTypeFunction,
							Function: ai.FunctionCall{
								Name:      parsed.Name,
								Arguments: mustJSON(parsed.Args),
							},
						})
					}
				}
			}
			return nil
		})

		if err != nil {
			log.Printf("ollama: chat error: %v", err)
			messageChannel <- ai.ChatCompletionMessage{
				Role:    ai.ChatMessageRoleAssistant,
				Content: "Error communicating with Ollama: " + err.Error(),
			}
			return
		}

		// Strip think blocks from the response content
		cleanContent := stripThinkBlocks(responseContent)
		
		// Send the complete response (may include tool calls with or without content)
		messageChannel <- ai.ChatCompletionMessage{
			Role:      ai.ChatMessageRoleAssistant,
			Content:   cleanContent,
			ToolCalls: toolCalls,
		}

		// Log detailed response information
	contentPreview := cleanContent
	if len(contentPreview) > 200 {
		contentPreview = contentPreview[:200] + "..."
	}
	
	if len(toolCalls) > 0 {
		toolInfo := make([]string, len(toolCalls))
		for i, tc := range toolCalls {
			toolInfo[i] = fmt.Sprintf("%s(%s)", tc.Function.Name, tc.Function.Arguments)
		}
		log.Printf("ollama: completed, content: '%s' (%d chars), tool calls: %d %v", 
			contentPreview, len(cleanContent), len(toolCalls), toolInfo)
	} else if len(cleanContent) == 0 {
		log.Printf("ollama: completed, empty response (no content or tool calls)")
	} else {
		log.Printf("ollama: completed, content: '%s' (%d chars)", 
			contentPreview, len(cleanContent))
	}
	}()

	return chunker.ProcessMessages(messageChannel)
}
