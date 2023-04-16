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
	go completion(ctx, req, ch)
	return ch
}

func completion(cc context.Context, req *CompletionRequest, channel chan<- *string) {

	defer close(channel)
	//log.Println(cc.Stats())

	ctx, cancel := context.WithTimeout(cc, req.Timeout)
	defer cancel()

	stream, err := req.Client.CreateChatCompletionStream(ctx, ai.ChatCompletionRequest{
		MaxTokens: req.MaxTokens,
		Model:     req.Model,
		Messages:  req.Messages,
		Stream:    true,
	})

	if err != nil {
		senderror(err, channel)
		return
	}

	defer stream.Close()
	// chunker := &Chunker{
	// 	Size:    req.Chunkmax,
	// 	Last:    time.Now(),
	// 	Timeout: req.Chunkdelay,
	// 	Buffer:  &bytes.Buffer{},
	// }

	for {
		response, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				send("\n", channel)
			} else {
				senderror(err, channel)
			}
			return
		}
		if len(response.Choices) != 0 {
			//log.Println(response.Choices[0].Delta.Content)
			send(response.Choices[0].Delta.Content, channel)
		}
	}
}

func senderror(err error, channel chan<- *string) {
	e := err.Error()
	channel <- &e
}

func send(chunk string, channel chan<- *string) {
	channel <- &chunk
}
