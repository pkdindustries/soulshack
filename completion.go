package main

import (
	"context"
	"errors"
	"log"
	"time"

	ai "github.com/sashabaranov/go-openai"
	vip "github.com/spf13/viper"
)

var aiClient *ai.Client

func getChatCompletion(chatctx *chatContext, msgs []ai.ChatCompletionMessage) (*string, error) {

	log.Printf("completing: messages %d, characters %d, maxtokens %d, model %s",
		len(msgs),
		sumMessageLengths(msgs),
		vip.GetInt("maxtokens"),
		vip.GetString("model"),
	)

	now := time.Now()
	ctx, cancel := context.WithTimeout(chatctx, vip.GetDuration("timeout"))
	defer cancel()

	resp, err := aiClient.CreateChatCompletion(
		ctx,
		ai.ChatCompletionRequest{
			MaxTokens: vip.GetInt("maxtokens"),
			Model:     vip.GetString("model"),
			Messages:  msgs,
		},
	)

	if err != nil {
		return nil, err
	}

	if len(resp.Choices) == 0 {
		return nil, errors.New("no response")
	}

	log.Printf("completed: %s, %d tokens", time.Since(now), resp.Usage.TotalTokens)
	return &resp.Choices[0].Message.Content, nil
}
