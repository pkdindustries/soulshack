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
	Messages     []ai.ChatCompletionMessage
	Tools        bool
	ToolRegistry *ToolRegistry
}

func NewCompletionRequest(ctx ChatContext) *CompletionRequest {
	return &CompletionRequest{
		Client:       ctx.AI,
		Timeout:      Config.ClientTimeout,
		Model:        Config.Model,
		MaxTokens:    Config.MaxTokens,
		Messages:     ctx.Session.GetHistory(),
		Temperature:  Config.Temperature,
		TopP:         Config.TopP,
		Tools:        Config.Tools,
		ToolRegistry: Config.ToolRegistry,
	}
}

type StreamResponse struct {
	Message ai.ChatCompletionStreamChoice
	Err     error
}

type StreamText struct {
	Text string
}

func (s *StreamResponse) String() string {
	if s.Err != nil {
		return fmt.Sprintf("Error: %v", s.Err)
	}
	return s.Message.Delta.Content
}

func Complete(ctx ChatContext, msg ai.ChatCompletionMessage) <-chan StreamResponse {
	ctx.Session.AddMessage(msg)
	log.Printf("complete: %s %.32s...", msg.Role, msg.Content)
	log.Printf("session: %d lines %d chars", len(ctx.Session.History), ctx.Session.TotalChars)
	messageChan := ChatCompletionStreamTask(ctx, NewCompletionRequest(ctx))
	chunkChan, toolChan := NewChunker().Filter(messageChan)
	return processCompletionStreams(ctx, chunkChan, toolChan)
}

func processCompletionStreams(ctx ChatContext, chunkChan <-chan StreamResponse, toolChan <-chan ai.ToolCall) <-chan StreamResponse {
	outputChan := make(chan StreamResponse, 10)
	go func() {
		defer close(outputChan)
		for chunkChan != nil || toolChan != nil {
			select {
			case reply, ok := <-chunkChan:
				if !ok {
					chunkChan = nil
					continue
				}
				handleReply(ctx, reply, outputChan)
			case toolCall, ok := <-toolChan:
				if !ok {
					toolChan = nil
					continue
				}
				handleToolCall(ctx, toolCall, outputChan)
			}
		}
	}()
	return outputChan
}

func handleReply(ctx ChatContext, reply StreamResponse, outputChan chan<- StreamResponse) {
	if reply.Err != nil {
		log.Println("handleReply:", reply.Err)
	} else {
		ctx.Session.AddMessage(ai.ChatCompletionMessage{
			Role:    reply.Message.Delta.Role,
			Content: reply.Message.Delta.Content,
		})
	}
	outputChan <- reply
}

func handleToolCall(ctx ChatContext, toolCall ai.ToolCall, outputChan chan<- StreamResponse) {
	log.Printf("Tool Call Received: %v", toolCall)

	soultool, err := Config.ToolRegistry.GetToolByName(toolCall.Function.Name)
	if err != nil {
		log.Printf("Error getting tool registration: %v", err)
		ctx.Client.Cmd.Actionf(Config.Channel, "tool error")
		return
	}

	toolmsg, err := soultool.Execute(ctx, toolCall)
	if err != nil {
		log.Printf("Error executing tool: %v", err)
		ctx.Client.Cmd.Actionf(Config.Channel, "tool error")
	}

	ctx.Session.AddMessage(ai.ChatCompletionMessage{
		Role:      ai.ChatMessageRoleAssistant,
		ToolCalls: []ai.ToolCall{toolCall},
	})

	for response := range Complete(ctx, toolmsg) {
		outputChan <- response
	}
}

func ChatCompletionStreamTask(ctx ChatContext, req *CompletionRequest) <-chan StreamResponse {
	messageChannel := make(chan StreamResponse, 10)
	go completionStream(ctx, req, messageChannel)
	return messageChannel
}

func completionStream(ctx ChatContext, req *CompletionRequest, messageChannel chan<- StreamResponse) {
	defer close(messageChannel)

	timeout, cancel := context.WithTimeout(ctx, req.Timeout)
	defer cancel()

	tools := make([]ai.Tool, 0)
	if req.Tools {
		tools = Config.ToolRegistry.GetToolDefinitions()
	}

	stream, err := req.Client.CreateChatCompletionStream(timeout, ai.ChatCompletionRequest{
		MaxCompletionTokens: req.MaxTokens,
		Model:               req.Model,
		Messages:            req.Messages,
		Temperature:         req.Temperature,
		TopP:                req.TopP,
		Stream:              true,
		Tools:               tools,
		ParallelToolCalls:   false,
	})

	if err != nil {
		log.Println("completionStream: failed to create chat completion stream:", err)
		messageChannel <- StreamResponse{Err: fmt.Errorf("failed to create chat completion stream: %w", err)}
		return
	}
	defer stream.Close()

	for {
		select {
		case <-ctx.Done():
			log.Println("completionStream: context canceled")
			messageChannel <- StreamResponse{Err: fmt.Errorf("api timeout exceeded: %w", ctx.Err())}
			return
		default:
			response, err := stream.Recv()

			if err != nil {
				handleStreamError(err, messageChannel)
				return
			}

			if len(response.Choices) > 0 {
				choice := response.Choices[0]
				messageChannel <- StreamResponse{Message: choice}
			}
		}
	}
}

func handleStreamError(err error, messageChannel chan<- StreamResponse) {
	log.Println("handleStreamError:", err)

	if errors.Is(err, io.EOF) {
		msg := ai.ChatCompletionStreamChoice{
			Delta: ai.ChatCompletionStreamChoiceDelta{
				Role:    ai.ChatMessageRoleAssistant,
				Content: "\n",
			},
		}
		messageChannel <- StreamResponse{Message: msg}
	} else {
		messageChannel <- StreamResponse{Err: fmt.Errorf("stream receive error: %w", err), Message: ai.ChatCompletionStreamChoice{
			Delta: ai.ChatCompletionStreamChoiceDelta{
				Role:    ai.ChatMessageRoleAssistant,
				Content: fmt.Sprintf("i'm having trouble talking to my endpoint: %v", err),
			},
		}}
	}
}
