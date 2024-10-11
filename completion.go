package main

import (
	"bytes"
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

type StreamResponse struct {
	Message ai.ChatCompletionMessage
	Err     error
}

func Complete(ctx *ChatContext, msg ai.ChatCompletionMessage) {
	ctx.Session.AddMessage(msg)
	messageChan, toolChan := ChatCompletionStreamTask(ctx, newCompletionRequest(ctx))
	chunkChan := newChunker().Filter(messageChan)
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

func newCompletionRequest(ctx *ChatContext) *CompletionRequest {
	return &CompletionRequest{
		Client:      ctx.AI,
		Timeout:     BotConfig.ClientTimeout,
		Model:       BotConfig.Model,
		MaxTokens:   BotConfig.MaxTokens,
		Messages:    ctx.Session.GetHistory(),
		Temperature: BotConfig.Temperature,
		TopP:        BotConfig.TopP,
		Tools:       BotConfig.Tools,
	}
}

func newChunker() *Chunker {
	return &Chunker{
		Buffer:    &bytes.Buffer{},
		Length:    BotConfig.ChunkMax,
		Delay:     BotConfig.ChunkDelay,
		Quote:     BotConfig.ChunkQuoted,
		Last:      time.Now(),
		Tokenizer: BotConfig.Tokenizer,
	}
}

func ChatCompletionStreamTask(ctx *ChatContext, req *CompletionRequest) (<-chan StreamResponse, <-chan ai.ToolCall) {
	messageChannel := make(chan StreamResponse, 10)
	toolChannel := make(chan ai.ToolCall, 10)
	go completionStream(ctx, req, messageChannel, toolChannel)
	return messageChannel, toolChannel
}

func completionStream(ctx *ChatContext, req *CompletionRequest, messageChannel chan<- StreamResponse, toolChannel chan<- ai.ToolCall) {
	defer close(messageChannel)
	defer close(toolChannel)

	timeout, cancel := context.WithTimeout(ctx, req.Timeout)
	defer cancel()

	stream, err := createChatCompletionStream(timeout, req)
	if err != nil {
		messageChannel <- StreamResponse{Err: fmt.Errorf("failed to create chat completion stream: %w", err)}
		return
	}
	defer stream.Close()

	processStream(timeout, stream, messageChannel, toolChannel)
}

func createChatCompletionStream(ctx context.Context, req *CompletionRequest) (*ai.ChatCompletionStream, error) {
	tools := make([]ai.Tool, 0)
	if req.Tools {
		tools = BotConfig.ToolRegistry.GetToolDefinitions()
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

func processStream(ctx context.Context, stream *ai.ChatCompletionStream, messageChannel chan<- StreamResponse, toolChannel chan<- ai.ToolCall) {
	var assembling bool
	var partialToolCall ai.ToolCall

	for {
		select {
		case <-ctx.Done():
			log.Println("completionStream: context canceled")
			messageChannel <- StreamResponse{Err: fmt.Errorf("api timeout exceeded: %w", ctx.Err())}
			return
		default:
			response, err := stream.Recv()
			if err != nil {
				handleStreamError(err, messageChannel, partialToolCall)
				return
			}

			if len(response.Choices) > 0 {
				choice := response.Choices[0]
				assembling, partialToolCall = handleChoice(choice, assembling, partialToolCall, messageChannel, toolChannel)
			}
		}
	}
}

func handleReply(ctx *ChatContext, reply StreamResponse) {
	if reply.Err != nil {
		log.Println("error:", reply.Err)
		ctx.Client.Cmd.Reply(*ctx.Event, "error:"+reply.Err.Error())
		return
	}
	ctx.Client.Cmd.Reply(*ctx.Event, reply.Message.Content)
	ctx.Session.AddMessage(reply.Message)
}

func handleToolCall(ctx *ChatContext, toolCall ai.ToolCall) {
	log.Printf("Function Call Received: %v", toolCall)

	soultool, err := BotConfig.ToolRegistry.GetToolByName(toolCall.Function.Name)
	if err != nil {
		ctx.Client.Cmd.Reply(*ctx.Event, "error: "+err.Error())
		return
	}

	toolmsg, err := soultool.Execute(*ctx, toolCall)
	if err != nil {
		log.Printf("Error executing function: %v", err)
	}

	ctx.Session.AddMessage(ai.ChatCompletionMessage{
		Role:      ai.ChatMessageRoleAssistant,
		ToolCalls: []ai.ToolCall{toolCall},
	})
	Complete(ctx, toolmsg)
}

func handleStreamError(err error, messageChannel chan<- StreamResponse, partialToolCall ai.ToolCall) {
	if errors.Is(err, io.EOF) {
		log.Println("completionStream: finished")
		msg := ai.ChatCompletionMessage{
			Role:      ai.ChatMessageRoleAssistant,
			Content:   "\n",
			ToolCalls: []ai.ToolCall{partialToolCall},
		}
		messageChannel <- StreamResponse{Message: msg}
	} else {
		messageChannel <- StreamResponse{Err: fmt.Errorf("stream receive error: %w", err)}
	}
}

func handleChoice(choice ai.ChatCompletionStreamChoice, assemblingToolCall bool, partialToolCall ai.ToolCall,
	messageChannel chan<- StreamResponse, toolChannel chan<- ai.ToolCall) (bool, ai.ToolCall) {

	if choice.FinishReason == ai.FinishReasonToolCalls {
		log.Println("completionStream:", choice.FinishReason)
		log.Println("completionStream: tool call assembled", partialToolCall)
		toolChannel <- partialToolCall
		assemblingToolCall = false
	}

	if len(choice.Delta.ToolCalls) > 0 {
		assemblingToolCall, partialToolCall = assembleToolCall(choice.Delta.ToolCalls[0], assemblingToolCall, partialToolCall)
	}

	if len(choice.Delta.Content) > 0 {
		msg := ai.ChatCompletionMessage{Role: ai.ChatMessageRoleAssistant, Content: choice.Delta.Content}
		messageChannel <- StreamResponse{Message: msg}
	}

	return assemblingToolCall, partialToolCall
}

func assembleToolCall(toolCall ai.ToolCall, assemblingToolCall bool, partialToolCall ai.ToolCall) (bool, ai.ToolCall) {
	if !assemblingToolCall {
		log.Println("completionStream: assembling tool call", toolCall)
		partialToolCall = toolCall
		assemblingToolCall = true
	}
	partialToolCall.Function.Arguments += toolCall.Function.Arguments
	return assemblingToolCall, partialToolCall
}
