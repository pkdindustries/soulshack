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
	Timeout     time.Duration
	Temperature float32
	TopP        float32
	Model       string
	MaxTokens   int
	Client      *ai.Client
	Messages    []ai.ChatCompletionMessage
	Tools       bool
}

func NewCompletionRequest(ctx *ChatContext) *CompletionRequest {
	return &CompletionRequest{
		Client:      ctx.AI,
		Timeout:     Config.ClientTimeout,
		Model:       Config.Model,
		MaxTokens:   Config.MaxTokens,
		Messages:    ctx.Session.GetHistory(),
		Temperature: Config.Temperature,
		TopP:        Config.TopP,
		Tools:       Config.Tools,
	}
}

type StreamResponse struct {
	Message ai.ChatCompletionStreamChoice
	Err     error
}

func Complete(ctx *ChatContext, msg ai.ChatCompletionMessage) {
	ctx.Session.AddMessage(msg)
	messageChan := ChatCompletionStreamTask(ctx, NewCompletionRequest(ctx))
	chunkChan, toolChan := NewChunker().Filter(messageChan)
	processCompletionStreams(ctx, chunkChan, toolChan)
}

func processCompletionStreams(ctx *ChatContext, chunkChan <-chan StreamResponse, toolChan <-chan ai.ToolCall) {
	for chunkChan != nil || toolChan != nil {
		select {
		case reply, ok := <-chunkChan:
			if !ok {
				chunkChan = nil
				continue
			}
			handleReply(ctx, reply)
		case toolCall, ok := <-toolChan:
			if !ok {
				toolChan = nil
				continue
			}
			handleToolCall(ctx, toolCall)
		}
	}
}

func handleReply(ctx *ChatContext, reply StreamResponse) {
	if reply.Err != nil {
		log.Println("error:", reply.Err)
		ctx.Client.Cmd.Reply(*ctx.Event, "error:"+reply.Err.Error())
		return
	}
	ctx.Client.Cmd.Reply(*ctx.Event, reply.Message.Delta.Content)
	ctx.Session.AddMessage(ai.ChatCompletionMessage{
		Role:    reply.Message.Delta.Role,
		Content: reply.Message.Delta.Content,
	})
}

func handleToolCall(ctx *ChatContext, toolCall ai.ToolCall) {
	log.Printf("Tool Call Received: %v", toolCall)

	soultool, err := Config.ToolRegistry.GetToolByName(toolCall.Function.Name)
	if err != nil {
		ctx.Client.Cmd.Reply(*ctx.Event, "error: "+err.Error())
		return
	}

	toolmsg, err := soultool.Execute(*ctx, toolCall)
	if err != nil {
		log.Printf("Error executing tool: %v", err)
	}

	ctx.Session.AddMessage(ai.ChatCompletionMessage{
		Role:      ai.ChatMessageRoleAssistant,
		ToolCalls: []ai.ToolCall{toolCall},
	})
	Complete(ctx, toolmsg)
}

func ChatCompletionStreamTask(ctx *ChatContext, req *CompletionRequest) <-chan StreamResponse {
	messageChannel := make(chan StreamResponse, 10)
	go completionStream(ctx, req, messageChannel)
	return messageChannel
}

func completionStream(ctx *ChatContext, req *CompletionRequest, messageChannel chan<- StreamResponse) {
	defer close(messageChannel)

	timeout, cancel := context.WithTimeout(ctx, req.Timeout)
	defer cancel()

	stream, err := createChatCompletionStream(timeout, req)
	if err != nil {
		messageChannel <- StreamResponse{Err: fmt.Errorf("failed to create chat completion stream: %w", err)}
		return
	}
	defer stream.Close()

	processStream(timeout, stream, messageChannel)
}

func createChatCompletionStream(ctx context.Context, req *CompletionRequest) (*ai.ChatCompletionStream, error) {
	tools := make([]ai.Tool, 0)
	if req.Tools {
		tools = Config.ToolRegistry.GetToolDefinitions()
	}

	return req.Client.CreateChatCompletionStream(ctx, ai.ChatCompletionRequest{
		MaxCompletionTokens: req.MaxTokens,
		Model:               req.Model,
		Messages:            req.Messages,
		Temperature:         req.Temperature,
		TopP:                req.TopP,
		Stream:              true,
		Tools:               tools,
		ParallelToolCalls:   false,
	})
}

func processStream(ctx context.Context, stream *ai.ChatCompletionStream, messageChannel chan<- StreamResponse) {

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
				handleChoice(choice, messageChannel)
			}
		}
	}
}

func handleStreamError(err error, messageChannel chan<- StreamResponse) {
	if errors.Is(err, io.EOF) {
		log.Println("completionStream: finished")
		msg := ai.ChatCompletionStreamChoice{
			Delta: ai.ChatCompletionStreamChoiceDelta{
				Role:    ai.ChatMessageRoleAssistant,
				Content: "\n",
			},
		}
		messageChannel <- StreamResponse{Message: msg}
	} else {
		messageChannel <- StreamResponse{Err: fmt.Errorf("stream receive error: %w", err)}
	}
}

func handleChoice(choice ai.ChatCompletionStreamChoice, messageChannel chan<- StreamResponse) {

	messageChannel <- StreamResponse{Message: choice}

}
