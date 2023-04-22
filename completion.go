package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	ai "github.com/sashabaranov/go-openai"
)

type CompletionRequest struct {
	Timeout   time.Duration
	Model     string
	MaxTokens int
	Client    *ai.Client
	Messages  []ai.ChatCompletionMessage
}

func CompletionStreamTask(ctx context.Context, req *CompletionRequest) <-chan *string {
	ch := make(chan *string)
	go stream(ctx, req, ch)
	return ch
}

func CompletionTask(ctx context.Context, req *CompletionRequest) (*string, error) {
	ctx, cancel := context.WithTimeout(ctx, req.Timeout)
	defer cancel()

	response, err := req.Client.CreateChatCompletion(ctx, ai.ChatCompletionRequest{
		MaxTokens: req.MaxTokens,
		Model:     req.Model,
		Messages:  req.Messages,
	})

	if err != nil {
		return nil, err
	}

	if len(response.Choices) == 0 {
		return nil, errors.New("no choices")
	}

	return &response.Choices[0].Message.Content, nil
}

func stream(ctx context.Context, req *CompletionRequest, ch chan<- *string) {
	defer close(ch)

	ctx, cancel := context.WithTimeout(ctx, req.Timeout)
	defer cancel()

	stream, err := req.Client.CreateChatCompletionStream(ctx, ai.ChatCompletionRequest{
		MaxTokens: req.MaxTokens,
		Model:     req.Model,
		Messages:  req.Messages,
		Stream:    true,
	})

	if err != nil {
		ch <- strp(err.Error())
		return
	}
	defer stream.Close()

	for {
		response, err := stream.Recv()
		fmt.Printf(".")
		if err != nil {
			if errors.Is(err, io.EOF) {
				ch <- strp("\n")
			} else {
				ch <- strp(err.Error())
			}
			break
		}
		if len(response.Choices) > 0 {
			ch <- strp(response.Choices[0].Delta.Content)
		}
	}
}

func strp(s string) *string {
	return &s
}
