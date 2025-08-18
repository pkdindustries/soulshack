package main

import (
	"context"
	"log"
	"time"

	"github.com/alexschlessinger/pollytool/messages"
	"github.com/alexschlessinger/pollytool/tools"
)

type LLM interface {
	// New simplified interface - single byte channel output
	ChatCompletionStream(context.Context, *CompletionRequest, ChatContextInterface) <-chan []byte
}

type CompletionRequest struct {
	APIKey      string
	BaseURL     string
	Timeout     time.Duration
	Temperature float32
	TopP        float32
	Model       string
	MaxTokens   int
	Session     Session
	Tools       []tools.Tool
}

func NewCompletionRequest(config *Configuration, session Session, tools []tools.Tool) *CompletionRequest {
	return &CompletionRequest{
		APIKey:      config.API.OpenAIKey,
		BaseURL:     config.API.OpenAIURL,
		Timeout:     config.API.Timeout,
		Model:       config.Model.Model,
		MaxTokens:   config.Model.MaxTokens,
		Session:     session,
		Temperature: config.Model.Temperature,
		TopP:        config.Model.TopP,
		Tools:       tools,
	}
}

func CompleteWithText(ctx ChatContextInterface, msg string) (<-chan string, error) {
	cmsg := messages.ChatMessage{
		Role:    messages.MessageRoleUser,
		Content: msg,
	}
	log.Printf("complete: %s %.64s...", cmsg.Role, cmsg.Content)
	ctx.GetSession().AddMessage(cmsg)

	return complete(ctx)
}

func complete(ctx ChatContextInterface) (<-chan string, error) {
	session := ctx.GetSession()
	config := ctx.GetConfig()
	sys := ctx.GetSystem()
	
	// Get all tools from registry
	var allTools []tools.Tool
	if sys.GetToolRegistry() != nil {
		allTools = sys.GetToolRegistry().All()
	}
	
	req := NewCompletionRequest(config, session, allTools)
	llm := sys.GetLLM()

	// Get the byte stream from the new interface
	byteChan := llm.ChatCompletionStream(context.Background(), req, ctx)

	// Convert bytes to strings for IRC output
	outputChan := make(chan string, 10)

	go func() {
		defer close(outputChan)
		
		for bytes := range byteChan {
			outputChan <- string(bytes)
		}
	}()

	return outputChan, nil
}