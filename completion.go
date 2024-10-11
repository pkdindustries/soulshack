package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	ai "github.com/sashabaranov/go-openai"
)

var (
	once   sync.Once
	client *ai.Client
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

	messageChan, toolCallChan := ChatCompletionStreamTask(ctx, &CompletionRequest{
		Client:      ctx.AI,
		Timeout:     BotConfig.ClientTimeout,
		Model:       BotConfig.Model,
		MaxTokens:   BotConfig.MaxTokens,
		Messages:    ctx.Session.GetHistory(),
		Temperature: BotConfig.Temperature,
		TopP:        BotConfig.TopP,
		Tools:       BotConfig.Tools,
	})

	chunker := &Chunker{
		Buffer: &bytes.Buffer{},
		Length: BotConfig.ChunkMax,
		Delay:  BotConfig.ChunkDelay,
		Quote:  BotConfig.ChunkQuoted,
		Last:   time.Now(),
	}

	chunkChan := chunker.ChunkingFilter(messageChan)

	for {
		select {
		case reply, ok := <-chunkChan:
			if !ok {
				chunkChan = nil
				continue
			}
			if reply.Err != nil {
				log.Println("error:", reply.Err)
				ctx.Client.Cmd.Reply(*ctx.Event, "error:"+reply.Err.Error())
				continue
			}
			ctx.Client.Cmd.Reply(*ctx.Event, reply.Message.Content)
			ctx.Session.AddMessage(reply.Message)
		case toolCall, ok := <-toolCallChan:
			if !ok {
				toolCallChan = nil
				continue
			}
			log.Printf("Function Call Received: %v", toolCall)

			soultool, err := BotConfig.ToolRegistry.GetToolByName(toolCall.Function.Name)
			if err != nil {
				ctx.Client.Cmd.Reply(*ctx.Event, "error: "+err.Error())
				continue
			}
			toolmsg, err := soultool.Execute(*ctx, toolCall)

			if err != nil {
				log.Printf("Error executing function: %v", err)
				ctx.Client.Cmd.Reply(*ctx.Event, "error: "+err.Error())
				continue
			}
			// wtf
			ctx.Session.AddMessage(ai.ChatCompletionMessage{Role: ai.ChatMessageRoleAssistant, ToolCalls: []ai.ToolCall{toolCall}})
			Complete(ctx, toolmsg)
		}

		if chunkChan == nil && toolCallChan == nil {
			break
		}
	}
}

// ChatCompletionStreamTask handles streaming completions asynchronously
func ChatCompletionStreamTask(ctx *ChatContext, req *CompletionRequest) (<-chan StreamResponse, <-chan ai.ToolCall) {
	messageChannel := make(chan StreamResponse, 10)
	toolChannel := make(chan ai.ToolCall, 10)
	go completionstream(ctx, req, messageChannel, toolChannel)
	return messageChannel, toolChannel
}

func completionstream(ctx *ChatContext, req *CompletionRequest, messageChannel chan<- StreamResponse, toolChannel chan<- ai.ToolCall) {
	defer close(messageChannel)
	defer close(toolChannel)
	timeout, cancel := context.WithTimeout(ctx, req.Timeout)
	defer cancel()

	tools := make([]ai.Tool, 0)
	if req.Tools {
		tools = BotConfig.ToolRegistry.GetToolDefinitions()
	}

	stream, err := req.Client.CreateChatCompletionStream(timeout, ai.ChatCompletionRequest{
		MaxCompletionsTokens: req.MaxTokens,
		Model:                req.Model,
		Messages:             req.Messages,
		Temperature:          req.Temperature,
		TopP:                 req.TopP,
		Stream:               true,
		Tools:                tools,
	})

	if err != nil {
		messageChannel <- StreamResponse{Err: fmt.Errorf("failed to create chat completion stream: %w", err)}
		return
	}
	defer stream.Close()

	var assemblingToolCall bool
	var partialToolCall ai.ToolCall
	for {
		select {
		case <-timeout.Done():
			log.Println("completionstream: context canceled")
			messageChannel <- StreamResponse{Err: fmt.Errorf("api timeout of %s exceeded: %w", req.Timeout.String(), ctx.Err())}
			return
		default:
			response, err := stream.Recv()
			//log.Println("completionstream: response", response)
			if err != nil {
				if errors.Is(err, io.EOF) {
					log.Println("completionstream: finished")
					msg := ai.ChatCompletionMessage{Role: ai.ChatMessageRoleAssistant, Content: "\n", ToolCalls: []ai.ToolCall{partialToolCall}}
					messageChannel <- StreamResponse{Message: msg}
				} else {
					messageChannel <- StreamResponse{Err: fmt.Errorf("stream receive error: %w", err)}
				}
				return
			}

			if len(response.Choices) > 0 {
				choice := response.Choices[0]
				if choice.FinishReason == ai.FinishReasonToolCalls {
					log.Println("completionstream:", choice.FinishReason)
					log.Println("completionstream: tool call assembled", partialToolCall)
					toolChannel <- partialToolCall
					assemblingToolCall = false
				}

				// api streams chunks of the toolcalls, we need to assemble them
				// this is the janky and should probably be part of the chunker?
				if len(choice.Delta.ToolCalls) > 0 {
					toolCall := choice.Delta.ToolCalls[0]
					if !assemblingToolCall {
						log.Println("completionstream: assembling tool call", toolCall)
						partialToolCall = toolCall
						assemblingToolCall = true
					}
					partialToolCall.Function.Arguments += toolCall.Function.Arguments
				}

				if len(choice.Delta.Content) > 0 {
					msg := ai.ChatCompletionMessage{Role: ai.ChatMessageRoleAssistant, Content: choice.Delta.Content}
					messageChannel <- StreamResponse{Message: msg}
				}
			}
		}
	}
}

func GetAIClient(config *ai.ClientConfig) *ai.Client {
	once.Do(func() {
		client = ai.NewClientWithConfig(*config)
	})
	return client
}
