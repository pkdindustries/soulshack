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

func NewCompletionRequest(ctx ChatContextInterface) *CompletionRequest {
	return &CompletionRequest{
		Client:       ctx.GetAI(),
		Timeout:      Config.ClientTimeout,
		Model:        Config.Model,
		MaxTokens:    Config.MaxTokens,
		Messages:     ctx.GetSession().GetHistory(),
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

type ChatText struct {
	Role string
	Text string
}

func Complete(ctx ChatContextInterface, msg ChatText) <-chan ChatText {
	cmsg := ai.ChatCompletionMessage{
		Role:    msg.Role,
		Content: msg.Text,
	}
	return completeWithMessage(ctx, cmsg)
}

func completeWithMessage(ctx ChatContextInterface, msg ai.ChatCompletionMessage) <-chan ChatText {
	ctx.GetSession().AddMessage(msg)
	log.Printf("complete: %s %.32s...", msg.Role, msg.Content)
	log.Printf("session: %d lines %d chars", len(ctx.GetSession().History), ctx.GetSession().TotalChars)
	var respChan <-chan StreamResponse
	if Config.Stream {
		respChan = ChatCompletionStreamTask(ctx, NewCompletionRequest(ctx))
	} else {
		respChan = ChatCompletionTask(ctx, NewCompletionRequest(ctx))
	}

	textChan, toolChan := NewChunker().FilterTask(respChan)
	return processToolsAndText(ctx, textChan, toolChan)
}

func processToolsAndText(ctx ChatContextInterface, textChan <-chan ChatText, toolChan <-chan ai.ToolCall) <-chan ChatText {
	outputChan := make(chan ChatText, 10)
	go func() {
		defer close(outputChan)
		for textChan != nil || toolChan != nil {
			select {
			case reply, ok := <-textChan:
				if !ok {
					textChan = nil
					continue
				}
				ctx.GetSession().AddMessage(ai.ChatCompletionMessage{
					Role:    reply.Role,
					Content: reply.Text,
				})
				outputChan <- reply

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

func handleToolCall(ctx ChatContextInterface, toolCall ai.ToolCall, textChan chan<- ChatText) {
	log.Printf("Tool Call Received: %v", toolCall)

	soultool, err := Config.ToolRegistry.GetToolByName(toolCall.Function.Name)
	if err != nil {
		log.Printf("Error getting tool registration: %v", err)
		ctx.Action(Config.Channel, "tool error")
		return
	}

	toolmsg, err := soultool.Execute(ctx, toolCall)
	if err != nil {
		log.Printf("error executing tool: %v", err)
		ctx.Action(Config.Channel, "tool error!")
	}

	ctx.GetSession().AddMessage(ai.ChatCompletionMessage{
		Role:      ai.ChatMessageRoleAssistant,
		ToolCalls: []ai.ToolCall{toolCall},
	})

	for response := range completeWithMessage(ctx, toolmsg) {
		textChan <- response
	}
}

func ChatCompletionTask(ctx ChatContextInterface, req *CompletionRequest) <-chan StreamResponse {
	messageChannel := make(chan StreamResponse, 10)
	go func() {
		defer close(messageChannel)
		messages, err := completion(ctx, req)
		if err != nil {
			log.Println("ChatCompletionTask: failed to create chat completion:", err)
			messageChannel <- StreamResponse{Err: err}
			return
		}
		for _, message := range messages {
			messageChannel <- StreamResponse{Message: ai.ChatCompletionStreamChoice{
				Delta: ai.ChatCompletionStreamChoiceDelta{
					Role:      message.Role,
					Content:   message.Content,
					ToolCalls: message.ToolCalls,
				},
			}}
			messageChannel <- StreamResponse{Message: ai.ChatCompletionStreamChoice{
				Delta: ai.ChatCompletionStreamChoiceDelta{
					Role:    ai.ChatMessageRoleAssistant,
					Content: "\n",
				},
			}}
		}
	}()
	return messageChannel
}

func ChatCompletionStreamTask(ctx ChatContextInterface, req *CompletionRequest) <-chan StreamResponse {
	messageChannel := make(chan StreamResponse, 10)
	go completionStream(ctx, req, messageChannel)
	return messageChannel
}

func completionStream(ctx ChatContextInterface, req *CompletionRequest, respChan chan<- StreamResponse) {
	defer close(respChan)

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
		respChan <- StreamResponse{Err: fmt.Errorf("failed to create chat completion stream: %w", err)}
		return
	}
	defer stream.Close()

	for {
		select {
		case <-timeout.Done():
			log.Println("completionStream: context canceled")
			respChan <- StreamResponse{Err: fmt.Errorf("api timeout exceeded: %w", ctx.Err())}
			return
		default:
			response, err := stream.Recv()

			if err != nil {
				log.Println("completionstream:", err)
				if errors.Is(err, io.EOF) {
					msg := ai.ChatCompletionStreamChoice{
						Delta: ai.ChatCompletionStreamChoiceDelta{
							Role:    ai.ChatMessageRoleAssistant,
							Content: "\n",
						},
					}
					respChan <- StreamResponse{Message: msg}
					return
				}
			} else if len(response.Choices) > 0 {
				choice := response.Choices[0]
				respChan <- StreamResponse{Message: choice}
			}
		}
	}
}

func completion(ctx ChatContextInterface, req *CompletionRequest) ([]ai.ChatCompletionMessage, error) {
	timeout, cancel := context.WithTimeout(ctx, req.Timeout)
	defer cancel()

	tools := make([]ai.Tool, 0)
	if req.Tools {
		tools = Config.ToolRegistry.GetToolDefinitions()
	}

	response, err := req.Client.CreateChatCompletion(timeout, ai.ChatCompletionRequest{
		MaxCompletionTokens: req.MaxTokens,
		Model:               req.Model,
		Messages:            req.Messages,
		Temperature:         req.Temperature,
		TopP:                req.TopP,
		Tools:               tools,
		ParallelToolCalls:   false,
	})

	if err != nil {
		log.Println("completion: failed to create chat completion:", err)
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
	return msgs, nil
}
