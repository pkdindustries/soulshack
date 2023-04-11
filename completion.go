package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"

	ai "github.com/sashabaranov/go-openai"
)

var aiClient *ai.Client

func ChatCompletionTask(ctx *ChatContext) <-chan *string {
	ch := make(chan *string)
	go chatCompletionStream(ctx, ch)
	return ch
}

func chatCompletionStream(cc *ChatContext, channel chan<- *string) {

	defer close(channel)
	logcompletion(cc)

	ctx, cancel := context.WithTimeout(cc, cc.Session.Config.ClientTimeout)
	defer cancel()

	stream, err := aiClient.CreateChatCompletionStream(ctx, ai.ChatCompletionRequest{
		MaxTokens: cc.Session.Config.MaxTokens,
		Model:     cc.Personality.Model,
		Messages:  cc.Session.GetHistory(),
		Stream:    true,
	})

	if err != nil {
		senderror(err, channel)
		return
	}

	defer stream.Close()

	buffer := bytes.Buffer{}
	size := 350

	for {
		response, err := stream.Recv()

		if errors.Is(err, io.EOF) {
			log.Println("completion: finished")
			send(buffer.String(), channel)
			break
		}

		if err != nil {
			senderror(err, channel)
			send(buffer.String(), channel)
			return
		}

		if len(response.Choices) != 0 {
			buffer.WriteString(response.Choices[0].Delta.Content)
		}

		for {
			if ready, chunk := chunkable(&buffer, size); ready {
				send(chunk, channel)
			} else {
				break
			}
		}

	}
}

func chunkable(buffer *bytes.Buffer, chunksize int) (bool, string) {
	index := bytes.IndexByte(buffer.Bytes(), '\n')
	if index != -1 && index < chunksize {
		chunk := buffer.Next(index + 1)
		return true, string(chunk)
	}

	if buffer.Len() < chunksize {
		return false, ""
	}

	chunk := buffer.Next(chunksize)
	return true, string(chunk)
}

func senderror(err error, channel chan<- *string) {
	e := err.Error()
	channel <- &e
}

func send(chunk string, channel chan<- *string) {
	channel <- &chunk
}

func logcompletion(cc *ChatContext) {
	log.Printf("completion: messages %d, bytes %d, maxtokens %d, model %s",
		len(cc.Session.GetHistory()),
		cc.Session.Totalchars,
		cc.Session.Config.MaxTokens,
		cc.Personality.Model)

	if cc.Config.Verbose {
		cc.Session.Debug()
	}
}
