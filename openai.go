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
	log.Println("completionTask: done")
	return nil
}
