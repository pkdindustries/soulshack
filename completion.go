package main

import (
	"context"
	"errors"
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

func CompletionTask(ctx context.Context, req *CompletionRequest) <-chan *string {
	ch := make(chan *string)
	go completionstream(ctx, req, ch)
	return ch
}

func completionstream(ctx context.Context, req *CompletionRequest, ch chan<- *string) {
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
