package completion

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"time"

	ai "github.com/sashabaranov/go-openai"
	vip "github.com/spf13/viper"
)

type CompletionRequest struct {
	Timeout   time.Duration
	Temp      float64
	Model     string
	MaxTokens int
	Client    *ai.Client
	Messages  []ai.ChatCompletionMessage
}

func ChatCompletionStreamTask(ctx context.Context, req *CompletionRequest) <-chan *string {
	ch := make(chan *string)
	go completionstream(ctx, req, ch)
	return ch
}

func ChatCompletionTask(ctx context.Context, req *CompletionRequest) (*string, error) {
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

func Completion(ctx context.Context, req *CompletionRequest, msg string) string {
	cr := ai.CompletionRequest{
		MaxTokens: req.MaxTokens,
		Model:     req.Model,
		Prompt:    msg,
	}

	resp, err := req.Client.CreateCompletion(ctx, cr)
	if err != nil {
		return ""
	}

	return resp.Choices[0].Text
}

func completionstream(ctx context.Context, req *CompletionRequest, ch chan<- *string) {
	defer close(ch)
	log.Printf("completionstream: %v\n", len(req.Messages))
	ctx, cancel := context.WithTimeout(ctx, req.Timeout)
	defer cancel()

	stream, err := req.Client.CreateChatCompletionStream(ctx, ai.ChatCompletionRequest{
		MaxTokens: req.MaxTokens,
		Model:     req.Model,
		Messages:  req.Messages,
		Stream:    true,
	})

	if err != nil {
		log.Printf("completionstream: %v\n", err)
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

func GetAI() *ai.Client {
	return ai.NewClient(vip.GetString("openaikey"))
}
