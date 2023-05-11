package completion

import (
	"context"
	"errors"
	"io"
	"log"
	"sync"
	"time"

	ai "github.com/sashabaranov/go-openai"
	vip "github.com/spf13/viper"
)

var once sync.Once
var aiClient *ai.Client

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
	log.Printf("completiontask: %v messages", len(req.Messages))
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

func Complete(ctx context.Context, req *CompletionRequest, msg string) string {
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
	ctx, cancel := context.WithTimeout(ctx, req.Timeout)
	defer cancel()

	log.Printf("completion: %v messages", len(req.Messages))

	stream, err := req.Client.CreateChatCompletionStream(ctx, ai.ChatCompletionRequest{
		MaxTokens: req.MaxTokens,
		Model:     req.Model,
		Messages:  req.Messages,
		Stream:    true,
	})

	if err != nil {
		log.Printf("completion: %v\n", err)
		ch <- strp(err.Error())
		return
	}
	defer stream.Close()

	for {
		response, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				log.Println("completion: EOF")
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
	once.Do(func() {
		aiClient = ai.NewClient(vip.GetString("openaikey"))
	})
	return aiClient
}
