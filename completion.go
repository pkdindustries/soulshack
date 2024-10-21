package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"time"

	ai "github.com/sashabaranov/go-openai"
)

type CompletionRequest struct {
	Timeout      time.Duration
	Temperature  float32
	TopP         float32
	Model        string
	MaxTokens    int
	Client       *ai.Client
	Session      Session
	ToolRegistry *ToolRegistry
	Tools        bool
	Stream       bool
}

func NewCompletionRequest(session Session, config *Configuration) *CompletionRequest {
	return &CompletionRequest{
		Client:       config.Client,
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
	request := NewCompletionRequest(session, config)
	var respChan <-chan StreamResponse
	var err error
	if request.Stream {
		respChan, err = ChatCompletionStreamTask(ctx, request)
	} else {
		respChan, err = ChatCompletionTask(ctx, request)
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
					request.Session.AddMessage(*msg)
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

func ChatCompletionTask(ctx context.Context, req *CompletionRequest) (<-chan StreamResponse, error) {
	respChannel := make(chan StreamResponse, 10)
	timeout, cancel := context.WithTimeout(ctx, req.Timeout)
	defer close(respChannel)
	defer cancel()
	log.Println("completionTask: start")
	response, err := req.Client.CreateChatCompletion(timeout, ai.ChatCompletionRequest{
		MaxCompletionTokens: req.MaxTokens,
		Model:               req.Model,
		Messages:            req.Session.GetHistory(),
		Temperature:         req.Temperature,
		TopP:                req.TopP,
		Tools:               req.ToolRegistry.GetToolDefinitions(),
		ParallelToolCalls:   false,
	})

	if err != nil {
		log.Println("completionTask: failed to create chat completion:", err)
		return nil, fmt.Errorf("failed to create chat completion: %w", err)
	}

	if len(response.Choices) == 0 {
		return nil, fmt.Errorf("empty completion response")
	}

	msgs := make([]ai.ChatCompletionMessage, 0)
	msg := ai.ChatCompletionMessage{
		Role:       response.Choices[0].Message.Role,
		Content:    response.Choices[0].Message.Content,
		ToolCalls:  response.Choices[0].Message.ToolCalls,
		ToolCallID: response.Choices[0].Message.ToolCallID,
	}

	msgs = append(msgs, msg)

	for _, message := range msgs {
		respChannel <- StreamResponse{
			ChatCompletionStreamChoice: ai.ChatCompletionStreamChoice{
				Delta: ai.ChatCompletionStreamChoiceDelta{
					Role:      message.Role,
					Content:   message.Content,
					ToolCalls: message.ToolCalls,
				},
			},
		}
	}
	log.Println("completionTask: done")
	return respChannel, nil
}

func ChatCompletionStreamTask(ctx ChatContextInterface, req *CompletionRequest) (<-chan StreamResponse, error) {
	messageChannel := make(chan StreamResponse, 10)
	err := completionStream(ctx, req, messageChannel)
	return messageChannel, err
}

func completionStream(ctx ChatContextInterface, req *CompletionRequest, respChan chan<- StreamResponse) error {

	timeout, cancel := context.WithTimeout(ctx, req.Timeout)

	stream, err := req.Client.CreateChatCompletionStream(timeout, ai.ChatCompletionRequest{
		MaxCompletionTokens: req.MaxTokens,
		Model:               req.Model,
		Messages:            req.Session.GetHistory(),
		Temperature:         req.Temperature,
		TopP:                req.TopP,
		Stream:              true,
		Tools:               req.ToolRegistry.GetToolDefinitions(),
		ParallelToolCalls:   false,
	})

	if err != nil {
		log.Println("completionStreamTask: failed to create chat completion stream:", err)
		cancel()
		return err
	}

	go func() {
		defer stream.Close()
		defer close(respChan)
		defer cancel()
		log.Println("completionStreamTask: start")
		for {
			select {
			case <-timeout.Done():
				log.Println("completionStreamTask: context canceled")
				return
			default:
				response, err := stream.Recv()
				if err != nil {
					log.Println("completionstreamTask: error", err)
					if errors.Is(err, io.EOF) {
						log.Println("completionstreamTask: stream closed")
						return
					}
				}

				if len(response.Choices) > 0 {
					choice := response.Choices[0]
					respChan <- StreamResponse{ChatCompletionStreamChoice: choice}
					if choice.FinishReason != "" {
						log.Println("completionstreamTask:", choice.FinishReason)
						return
					}
				}

			}
		}
	}()
	return nil
}
