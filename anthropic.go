package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	ai "github.com/sashabaranov/go-openai"
)

var _ LLM = (*AnthropicClient)(nil)

type AnthropicContentBlock struct {
	Type  string          `json:"type"`
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

type AnthropicEvent struct {
	Type  string `json:"type"`
	Delta struct {
		StopReason  string `json:"stop_reason"`
		Type        string `json:"type"`
		Text        string `json:"text,omitempty"`
		PartialJSON string `json:"partial_json,omitempty"`
	} `json:"delta,omitempty"`
	ContentBlock *AnthropicContentBlock `json:"content_block,omitempty"`
}

type AnthropicMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"` // Can be string or []AnthropicContentItem
}

type AnthropicContentItem struct {
	Type        string `json:"type"`
	Text        string `json:"text,omitempty"`
	Tool_Use_ID string `json:"tool_use_id"`
	Content     string `json:"content"`
}

type AnthropicToolCall struct {
	Type      string `json:"type"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
	ID        string `json:"id"`
}

type AnthropicTextDelta struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type AnthropicClient struct {
	Client http.Client
}

// ChatCompletionTask implements LLM.
func (c *AnthropicClient) ChatCompletionTask(context.Context, *CompletionRequest) (<-chan StreamResponse, error) {
	panic("unimplemented")
}

func NewAnthropicClient(api APIConfig) *AnthropicClient {
	return &AnthropicClient{
		Client: http.Client{},
	}
}

func prettyPrint(i map[string]any) string {
	s, _ := json.MarshalIndent(i["messages"], "", " ")
	return string(s)
}

func (c *AnthropicClient) ChatCompletionStreamTask(ctx context.Context, request *CompletionRequest) (<-chan StreamResponse, error) {
	respChan := make(chan StreamResponse, 10)
	messages := request.Session.GetHistory()
	anthropicMessages := convertToAnthropicMessages(messages)
	tools := request.ToolRegistry.GetToolDefinitions()
	var reqBody map[string]interface{}
	if len(tools) > 0 {
		anthropicTools := convertToAnthropicTools(tools)
		reqBody = map[string]interface{}{
			"model":    request.Model,
			"messages": anthropicMessages,
			"tools":    anthropicTools,
			"tool_choice": map[string]interface{}{
				"type":                      "auto",
				"disable_parallel_tool_use": true,
			},
			"stream":     true,
			"max_tokens": request.MaxTokens,
		}
		log.Printf("anthropicclient: request with %d tools", len(anthropicTools))
	} else {
		reqBody = map[string]interface{}{
			"model":      "claude-3-5-sonnet-20241022",
			"messages":   anthropicMessages,
			"stream":     true,
			"max_tokens": 4096,
		}
	}

	reqJSON, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// XXX
	log.Printf("anthropicrequest: %s", prettyPrint(reqBody))

	req, err := http.NewRequestWithContext(ctx, "POST", request.BaseURL+"/v1/messages", strings.NewReader(string(reqJSON)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", request.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	go func() {
		defer resp.Body.Close()
		defer close(respChan)

		reader := bufio.NewReader(resp.Body)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					log.Printf("Error reading stream: %v", err)
				}
				break
			}

			line = strings.TrimSpace(line)
			if line == "" || line == "data: {\"type\": \"ping\"}" {
				continue
			}

			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")
			if data == "" {
				continue
			}

			var event AnthropicEvent
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				log.Printf("Error parsing event JSON: %v", err)
				continue
			}

			switch event.Type {
			case "content_block_start":
				if event.ContentBlock != nil && event.ContentBlock.Type == "tool_use" {
					respChan <- StreamResponse{
						ChatCompletionStreamChoice: ai.ChatCompletionStreamChoice{
							Delta: ai.ChatCompletionStreamChoiceDelta{
								Role: "assistant",
								ToolCalls: []ai.ToolCall{{
									ID: event.ContentBlock.ID,
									Function: ai.FunctionCall{
										Name: event.ContentBlock.Name,
									},
								}},
							},
						},
					}
				}

			case "content_block_delta":
				if event.Delta.Type == "input_json_delta" {
					respChan <- StreamResponse{
						ChatCompletionStreamChoice: ai.ChatCompletionStreamChoice{
							Delta: ai.ChatCompletionStreamChoiceDelta{
								Role: "assistant",
								ToolCalls: []ai.ToolCall{{
									Function: ai.FunctionCall{
										Arguments: event.Delta.PartialJSON,
									},
								}},
							},
						},
					}
				} else if event.Delta.Type == "text_delta" {
					respChan <- StreamResponse{
						ChatCompletionStreamChoice: ai.ChatCompletionStreamChoice{
							Delta: ai.ChatCompletionStreamChoiceDelta{
								Role:    "assistant",
								Content: event.Delta.Text,
							},
						},
					}
				}

			case "message_delta":
				if event.Delta.StopReason == "tool_use" {
					log.Printf("tool_use stop: %s", event.Delta)

					respChan <- StreamResponse{
						ChatCompletionStreamChoice: ai.ChatCompletionStreamChoice{
							FinishReason: "tool_calls",
						},
					}
				}

			case "message_stop":
				log.Printf("Message stop: %s", event.Delta.StopReason)
				respChan <- StreamResponse{
					ChatCompletionStreamChoice: ai.ChatCompletionStreamChoice{
						FinishReason: "stop",
					},
				}
			}
		}
	}()

	return respChan, nil
}

// [optional] Continue the conversation by sending a new message with the role of user, and a content block containing the tool_result type and the following information:
// tool_use_id: The id of the tool use request this is a result for.
// content: The result of the tool, as a string (e.g. "content": "15 degrees") or list of nested content blocks (e.g. "content": [{"type": "text", "text": "15 degrees"}]). These content blocks can use the text or image types.
func convertToAnthropicMessages(messages []ai.ChatCompletionMessage) []AnthropicMessage {
	var anthropicMessages []AnthropicMessage
	for _, msg := range messages {
		role := msg.Role
		if role == "system" || role == "tool" {
			role = "user"
		}

		// Handle tool results
		if msg.ToolCallID != "" {
			anthropicMessages = append(anthropicMessages, AnthropicMessage{
				Role: role,
				Content: []map[string]interface{}{{
					"type":        "tool_result",
					"content":     msg.Content,
					"tool_use_id": msg.ToolCallID,
				}},
			})
			continue
		}

		if msg.ToolCalls != nil {
			for _, toolCall := range msg.ToolCalls {
				log.Printf("tool_use thunker: %v", toolCall)
				var input map[string]interface{}
				if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &input); err != nil {
					log.Printf("Error unmarshaling tool arguments: %v", err)
					continue
				}

				anthropicMessages = append(anthropicMessages, AnthropicMessage{
					Role: role,
					Content: []map[string]interface{}{{
						"type":  "tool_use",
						"name":  toolCall.Function.Name,
						"input": input,
						"id":    toolCall.ID,
					}},
				})
			}
		}

		if msg.Content != "" {
			anthropicMessages = append(anthropicMessages, AnthropicMessage{
				Role:    role,
				Content: msg.Content,
			})
		}
	}
	return anthropicMessages
}

func convertToAnthropicTools(tools []ai.Tool) []interface{} {
	var anthropicTools []interface{}

	for _, tool := range tools {
		if tool.Function != nil {
			// Convert the raw parameters to a map
			var paramsMap map[string]interface{}
			paramsBytes, err := json.Marshal(tool.Function.Parameters)
			if err != nil {
				log.Printf("anthropictools: marshaling parameters: %v", err)
				continue // Skip this tool if we can't marshal parameters
			}
			if err := json.Unmarshal(paramsBytes, &paramsMap); err != nil {
				log.Printf("anthropictools: unmarshaling parameters: %v", err)
				continue // Skip this tool if we can't unmarshal parameters
			}

			functionDef := map[string]interface{}{
				"name":        tool.Function.Name,
				"description": tool.Function.Description,
				"input_schema": map[string]interface{}{
					"type":       "object",
					"properties": paramsMap["properties"],
					"required":   paramsMap["required"],
				},
			}

			anthropicTools = append(anthropicTools, functionDef)
		}
	}

	return anthropicTools
}
