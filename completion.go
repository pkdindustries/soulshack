package main

import (
	"context"
	"errors"
	"io"
	"log"
	"strings"

	ai "github.com/sashabaranov/go-openai"
	vip "github.com/spf13/viper"
)

var aiClient *ai.Client

func ChatCompletionTask(ctx *ChatContext) <-chan *string {
	ch := make(chan *string)
	go getChatCompletionStream(ctx, ch)
	return ch
}

func getChatCompletionStream(cc *ChatContext, channel chan *string) {
	defer close(channel)
	log.Printf("completing: messages %d, maxtokens %d, model %s",
		len(cc.Session.History),
		vip.GetInt("maxtokens"),
		vip.GetString("model"),
	)

	if vip.GetBool("verbose") {
		cc.Session.Debug()
	}

	ctx, cancel := context.WithTimeout(cc, vip.GetDuration("timeout"))
	defer cancel()

	stream, err := aiClient.CreateChatCompletionStream(ctx, ai.ChatCompletionRequest{
		MaxTokens: vip.GetInt("maxtokens"),
		Model:     vip.GetString("model"),
		Messages:  cc.Session.History,
		Stream:    true,
	})

	if err != nil {
		e := err.Error()
		channel <- &e
		return
	}

	defer stream.Close()

	accumulated := ""
	chunkSize := 350

	for {
		response, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			log.Println("completionstream finished")
			break
		}

		if err != nil {
			log.Printf("completionstream error: %v\n", err)
			a := accumulated + "\n"
			channel <- &a
			e := err.Error()
			channel <- &e
			return
		}

		if len(response.Choices) != 0 {
			accumulated += response.Choices[0].Delta.Content

			for len(accumulated) >= chunkSize || strings.Contains(accumulated, "\n") {
				chunk := ""
				newlineIndex := strings.Index(accumulated, "\n")
				if newlineIndex != -1 && newlineIndex < chunkSize {
					chunk = accumulated[:newlineIndex+1]
				} else {
					chunk = accumulated[:chunkSize]
				}

				//log.Println("stream chunk: ", chunk)
				channel <- &chunk
				accumulated = accumulated[len(chunk):]
			}
		}
	}

	// Send the remaining content if any
	if len(accumulated) > 0 {
		//log.Println("stream remaining content: ", accumulated)
		channel <- &accumulated
	}
}
