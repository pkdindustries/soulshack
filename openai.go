package main

import (
	"context"
	"fmt"
	"log"

	ai "github.com/sashabaranov/go-openai"
)

var _ LLM = (*OpenAIClient)(nil)

type OpenAIClient struct {
	ClientConfig ai.ClientConfig
	Client       *ai.Client
}

func NewOpenAIClient(api APIConfig) *OpenAIClient {
	cfg := ai.DefaultConfig(api.OpenAIKey)
	if api.OpenAIURL != "" {
		cfg.BaseURL = api.OpenAIURL
	}
	return &OpenAIClient{
		ClientConfig: cfg,
		Client:       ai.NewClientWithConfig(cfg),
	}
}

func (o OpenAIClient) ChatCompletionTask(ctx context.Context, req *CompletionRequest, chunker *Chunker) (<-chan []byte, <-chan *ToolCall, <-chan *ai.ChatCompletionMessage) {
	messageChannel := make(chan ai.ChatCompletionMessage, 10)
	o.completion(ctx, req, messageChannel)
	return chunker.ProcessMessages(messageChannel)
}

func (o OpenAIClient) completion(ctx context.Context, req *CompletionRequest, respChannel chan<- ai.ChatCompletionMessage) error {
	timeout, cancel := context.WithTimeout(ctx, req.Timeout)
	defer close(respChannel)
	defer cancel()
	log.Println("completionTask: start")

	ccr := ai.ChatCompletionRequest{
		MaxCompletionTokens: req.MaxTokens,
		Model:               req.Model,
		Messages:            req.Session.GetHistory(),
		Temperature:         req.Temperature,
		TopP:                req.TopP,
	}

	if req.ToolsEnabled && len(req.Tools) > 0 {
		var openaiTools []ai.Tool
		for _, tool := range req.Tools {
			openaiTools = append(openaiTools, ConvertToOpenAI(tool.GetSchema()))
		}
		ccr.Tools = openaiTools
	}

	response, err := o.Client.CreateChatCompletion(timeout, ccr)

	if err != nil {
		log.Println("completionTask: failed to create chat completion:", err)
		respChannel <- ai.ChatCompletionMessage{
			Role:    ai.ChatMessageRoleAssistant,
			Content: "failed to create chat completion: " + err.Error(),
		}
		return err
	}

	if len(response.Choices) == 0 {
		return fmt.Errorf("empty completion response")
	}

	msg := ai.ChatCompletionMessage{
		Role:       response.Choices[0].Message.Role,
		Content:    response.Choices[0].Message.Content,
		ToolCalls:  response.Choices[0].Message.ToolCalls,
		ToolCallID: response.Choices[0].Message.ToolCallID,
	}

	respChannel <- msg
	
	// Log detailed response information
	contentPreview := msg.Content
	if len(contentPreview) > 200 {
		contentPreview = contentPreview[:200] + "..."
	}
	
	if len(msg.ToolCalls) > 0 {
		toolInfo := make([]string, len(msg.ToolCalls))
		for i, tc := range msg.ToolCalls {
			toolInfo[i] = fmt.Sprintf("%s(%s)", tc.Function.Name, tc.Function.Arguments)
		}
		log.Printf("openai: completed, content: '%s' (%d chars), tool calls: %d %v", 
			contentPreview, len(msg.Content), len(msg.ToolCalls), toolInfo)
	} else if len(msg.Content) == 0 {
		log.Printf("openai: completed, empty response (no content or tool calls)")
	} else {
		log.Printf("openai: completed, content: '%s' (%d chars)", 
			contentPreview, len(msg.Content))
	}
	return nil
}
