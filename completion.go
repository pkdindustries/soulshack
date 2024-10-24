package main

import (
	"context"
	"log"
	"time"

	ai "github.com/sashabaranov/go-openai"
)

type LLM interface {
	ChatCompletionTask(context.Context, *CompletionRequest) (<-chan StreamResponse, error)
	ChatCompletionStreamTask(context.Context, *CompletionRequest) (<-chan StreamResponse, error)
}

type CompletionRequest struct {
	APIKey       string
	BaseURL      string
	Timeout      time.Duration
	Temperature  float32
	TopP         float32
	Model        string
	MaxTokens    int
	Session      Session
	ToolRegistry *ToolRegistry
	Tools        bool
	Stream       bool
}

func NewCompletionRequest(session Session, config *Configuration) *CompletionRequest {
	return &CompletionRequest{
		APIKey:       config.API.Key,
		BaseURL:      config.API.URL,
		Timeout:      config.API.Timeout,
		Model:        config.Model.Model,
		MaxTokens:    config.Model.MaxTokens,
		Session:      session,
		Temperature:  config.Model.Temperature,
		TopP:         config.Model.TopP,
		Tools:        config.Bot.Tools,
		ToolRegistry: config.ToolRegistry,
		Stream:       config.API.Stream,
	}
}

type StreamResponse struct {
	ai.ChatCompletionStreamChoice
}

func CompleteWithText(ctx ChatContextInterface, msg string) (<-chan string, error) {
	cmsg := ai.ChatCompletionMessage{
		Role:    ai.ChatMessageRoleUser,
		Content: msg,
	}
	log.Printf("complete: %s %.64s...", cmsg.Role, cmsg.Content)
	ctx.GetSession().AddMessage(cmsg)

	return complete(ctx)
}

func complete(ctx ChatContextInterface) (<-chan string, error) {
	session := ctx.GetSession()
	config := ctx.GetConfig()
	req := NewCompletionRequest(session, config)
	llm := config.LLM

	var respChan <-chan StreamResponse
	var err error
	if req.Stream {
		respChan, err = llm.ChatCompletionStreamTask(ctx, req)
	} else {
		respChan, err = llm.ChatCompletionTask(ctx, req)
	}

	if err != nil {
		log.Printf("error completing chat: %v", err)
		return nil, err
	}

	textChan, toolChan, msgChan := NewChunker(config).FilterTask(respChan)

	outputChan := make(chan string, 10)

	go func() {
		defer close(outputChan)

		for {
			select {
			case toolCall, ok := <-toolChan:
				if !ok {
					toolChan = nil
				} else {
					toolch, _ := handleToolCall(ctx, toolCall)
					for r := range toolch {
						outputChan <- r
					}
				}
			case reply, ok := <-textChan:
				if !ok {
					textChan = nil
				} else {
					outputChan <- string(reply)
				}
			case msg, ok := <-msgChan:
				if !ok {
					msgChan = nil
				} else {
					session.AddMessage(*msg)
				}
			}

			if toolChan == nil && textChan == nil {
				break
			}
		}
	}()

	return outputChan, nil
}

func handleToolCall(ctx ChatContextInterface, toolCall *ai.ToolCall) (<-chan string, error) {
	log.Printf("Tool Call Received: %v", toolCall)
	config := ctx.GetConfig()
	soultool, err := config.ToolRegistry.GetToolByName(toolCall.Function.Name)
	if err != nil {
		log.Printf("Error getting tool registration: %v", err)
		return nil, err
	}

	toolmsg, err := soultool.Execute(ctx, *toolCall)
	if err != nil {
		log.Printf("error executing tool: %v", err)
	}

	ctx.GetSession().AddMessage(ai.ChatCompletionMessage{
		Role:      ai.ChatMessageRoleAssistant,
		ToolCalls: []ai.ToolCall{*toolCall},
	})
	ctx.GetSession().AddMessage(toolmsg)
	return complete(ctx)
}
