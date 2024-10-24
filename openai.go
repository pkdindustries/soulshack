package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"

	ai "github.com/sashabaranov/go-openai"
)

var _ LLM = (*OpenAIClient)(nil)

type OpenAIClient struct {
	ClientConfig ai.ClientConfig
	Client       *ai.Client
}

func NewOpenAIClient(api APIConfig) *OpenAIClient {
	cfg := ai.DefaultConfig(api.Key)
	if api.URL != "" {
		cfg.BaseURL = api.URL
	}
	return &OpenAIClient{
		ClientConfig: cfg,
		Client:       ai.NewClientWithConfig(cfg),
	}
}

func (o OpenAIClient) ChatCompletionStreamTask(ctx context.Context, req *CompletionRequest) (<-chan StreamResponse, error) {
	messageChannel := make(chan StreamResponse, 10)
	err := o.completionStream(ctx, req, messageChannel)
	return messageChannel, err
}

func (o OpenAIClient) completionStream(ctx context.Context, req *CompletionRequest, respChan chan<- StreamResponse) error {

	timeout, cancel := context.WithTimeout(ctx, req.Timeout)

	stream, err := o.Client.CreateChatCompletionStream(timeout, ai.ChatCompletionRequest{
		MaxCompletionTokens: req.MaxTokens,
		Model:               req.Model,
		Messages:            req.Session.GetHistory(),
		Temperature:         req.Temperature,
		TopP:                req.TopP,
		Stream:              true,
		Tools:               req.ToolRegistry.GetToolDefinitions(),
		ParallelToolCalls:   false,
	})

	if err != nil {
		log.Println("completionStreamTask: failed to create chat completion stream:", err)
		cancel()
		return err
	}

	go func() {
		defer stream.Close()
		defer close(respChan)
		defer cancel()
		log.Println("completionStreamTask: start")
		for {
			select {
			case <-timeout.Done():
				log.Println("completionStreamTask: context canceled")
				return
			default:
				response, err := stream.Recv()
				if err != nil {
					log.Println("completionstreamTask: error", err)
					if errors.Is(err, io.EOF) {
						log.Println("completionstreamTask: stream closed")
						return
					}
				}

				if len(response.Choices) > 0 {
					choice := response.Choices[0]
					respChan <- StreamResponse{ChatCompletionStreamChoice: choice}
					if choice.FinishReason != "" {
						log.Println("completionstreamTask:", choice.FinishReason)
						return
					}
				}

			}
		}
	}()
	return nil
}

func (o OpenAIClient) ChatCompletionTask(ctx context.Context, req *CompletionRequest) (<-chan StreamResponse, error) {
	respChannel := make(chan StreamResponse, 10)
	timeout, cancel := context.WithTimeout(ctx, req.Timeout)
	defer close(respChannel)
	defer cancel()
	log.Println("completionTask: start")
	response, err := o.Client.CreateChatCompletion(timeout, ai.ChatCompletionRequest{
		MaxCompletionTokens: req.MaxTokens,
		Model:               req.Model,
		Messages:            req.Session.GetHistory(),
		Temperature:         req.Temperature,
		TopP:                req.TopP,
		Tools:               req.ToolRegistry.GetToolDefinitions(),
		ParallelToolCalls:   false,
	})

	if err != nil {
		log.Println("completionTask: failed to create chat completion:", err)
		return nil, fmt.Errorf("failed to create chat completion: %w", err)
	}

	if len(response.Choices) == 0 {
		return nil, fmt.Errorf("empty completion response")
	}

	msgs := make([]ai.ChatCompletionMessage, 0)
	msg := ai.ChatCompletionMessage{
		Role:       response.Choices[0].Message.Role,
		Content:    response.Choices[0].Message.Content,
		ToolCalls:  response.Choices[0].Message.ToolCalls,
		ToolCallID: response.Choices[0].Message.ToolCallID,
	}

	msgs = append(msgs, msg)

	for _, message := range msgs {
		respChannel <- StreamResponse{
			ChatCompletionStreamChoice: ai.ChatCompletionStreamChoice{
				Delta: ai.ChatCompletionStreamChoiceDelta{
					Role:      message.Role,
					Content:   message.Content,
					ToolCalls: message.ToolCalls,
				},
			},
		}
	}
	log.Println("completionTask: done")
	return respChannel, nil
}
