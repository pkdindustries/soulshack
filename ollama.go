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
)

type OllamaClient struct {
	client *ollamaapi.Client
}

// authTransport adds Bearer token authentication to HTTP requests
type authTransport struct {
	Token string
	Base  http.RoundTripper
}

func (t *authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+t.Token)
	return t.Base.RoundTrip(req)
}

func NewOllamaClient(config APIConfig) *OllamaClient {
	ollamaURL := config.OllamaURL

	// Parse URL and create client
	u, err := url.Parse(ollamaURL)
	if err != nil {
		log.Printf("ollama: invalid URL %s: %v", ollamaURL, err)
		// Fall back to default if parsing fails
		u, _ = url.Parse("http://localhost:11434")
	}

	// Create HTTP client with optional Bearer token authentication
	httpClient := http.DefaultClient
	if config.OllamaKey != "" {
		httpClient = &http.Client{
			Transport: &authTransport{
				Token: config.OllamaKey,
				Base:  http.DefaultTransport,
			},
		}
		log.Printf("ollama: using Bearer token authentication")
	}

	client := ollamaapi.NewClient(u, httpClient)

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

func (o *OllamaClient) ChatCompletionTask(ctx context.Context, req *CompletionRequest, chunker *Chunker) (<-chan []byte, <-chan *ToolCall, <-chan *ChatMessage) {
	messageChannel := make(chan ChatMessage, 10)

	go func() {
		defer close(messageChannel)

		// Convert messages to Ollama format
		messages := MessagesToOllama(req.Session.GetHistory())

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

		// Add tool support if available
		if len(req.Tools) > 0 {
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
			toolCalls       []ChatMessageToolCall
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
						// Convert to agnostic tool call format
						toolCalls = append(toolCalls, ChatMessageToolCall{
							ID:        fmt.Sprintf("ollama-%d", idx),
							Name:      parsed.Name,
							Arguments: mustJSON(parsed.Args),
						})
					}
				}
			}
			return nil
		})

		if err != nil {
			log.Printf("ollama: chat error: %v", err)
			messageChannel <- ChatMessage{
				Role:    MessageRoleAssistant,
				Content: "Error communicating with Ollama: " + err.Error(),
			}
			return
		}

		// Strip think blocks from the response content
		cleanContent := stripThinkBlocks(responseContent)
		
		// Send the complete response (may include tool calls with or without content)
		messageChannel <- ChatMessage{
			Role:      MessageRoleAssistant,
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
			toolInfo[i] = fmt.Sprintf("%s(%s)", tc.Name, tc.Arguments)
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
