package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"
	"time"

	ai "github.com/sashabaranov/go-openai"
)

var (
	once   sync.Once
	client *ai.Client
)

func Complete(ctx *ChatContext, role string, msg string) {
	ctx.Session.AddMessage(role, msg)

	respch := ChatCompletionStreamTask(ctx, &CompletionRequest{
		Client:      ctx.AI,
		Timeout:     ctx.Session.Config.ClientTimeout,
		Model:       ctx.Session.Config.Model,
		MaxTokens:   ctx.Session.Config.MaxTokens,
		Messages:    ctx.Session.GetHistory(),
		Tempurature: ctx.Session.Config.Tempurature,
	})

	chunker := &Chunker{
		Buffer: &bytes.Buffer{},
		Length: ctx.Session.Config.ChunkMax,
		Delay:  ctx.Session.Config.ChunkDelay,
		Quote:  ctx.Session.Config.ChunkQuoted,
		Last:   time.Now(),
	}

	chunkch := chunker.ChunkingFilter(respch)

	all := strings.Builder{}
	for reply := range chunkch {
		if reply.Err != nil {
			ctx.Client.Cmd.Reply(*ctx.Event, "error: "+reply.Err.Error())
			continue
		}
		all.WriteString(reply.Content)
		ctx.Client.Cmd.Reply(*ctx.Event, reply.Content)
	}

	ctx.Session.AddMessage(RoleAssistant, all.String())
}

// CompletionRequest holds all the necessary fields to make a completion request
type CompletionRequest struct {
	Timeout     time.Duration
	Tempurature float32
	Model       string
	MaxTokens   int
	Client      *ai.Client
	Messages    []ai.ChatCompletionMessage
}

// StreamResponse is used to handle both content and error in the streaming response
type StreamResponse struct {
	Content string
	Err     error
}

// ChatCompletionStreamTask handles streaming completions asynchronously
func ChatCompletionStreamTask(ctx context.Context, req *CompletionRequest) <-chan StreamResponse {
	ch := make(chan StreamResponse, 10)
	go completionstream(ctx, req, ch)
	return ch
}

func completionstream(ctx context.Context, req *CompletionRequest, ch chan<- StreamResponse) {
	defer close(ch)
	ctx, cancel := context.WithTimeout(ctx, req.Timeout)
	defer cancel()

	log.Printf("completionstream: %v messages", len(req.Messages))

	stream, err := req.Client.CreateChatCompletionStream(ctx, ai.ChatCompletionRequest{
		MaxTokens:   req.MaxTokens,
		Model:       req.Model,
		Messages:    req.Messages,
		Temperature: req.Tempurature,
		Stream:      true,
	})

	if err != nil {
		ch <- StreamResponse{Err: fmt.Errorf("failed to create chat completion stream: %w", err)}
		return
	}
	defer stream.Close()

	for {
		select {
		case <-ctx.Done():
			log.Println("completionstream: context canceled")
			ch <- StreamResponse{Err: fmt.Errorf("api time exceeded: %w", ctx.Err())}
			return
		default:
			response, err := stream.Recv()
			if err != nil {
				if errors.Is(err, io.EOF) {
					log.Println("completionstream: EOF")
					ch <- StreamResponse{Content: "\n"}
				} else {
					ch <- StreamResponse{Err: fmt.Errorf("stream receive error: %w", err)}
				}
				return
			}
			if len(response.Choices) > 0 {
				ch <- StreamResponse{Content: response.Choices[0].Delta.Content}
			}
		}
	}
}

func NewAI(config *ai.ClientConfig) *ai.Client {
	once.Do(func() {
		client = ai.NewClientWithConfig(*config)
	})
	return client
}
