package main

import (
	"context"
	"fmt"
	"log"
	"time"

	ai "github.com/sashabaranov/go-openai"
)

type LLM interface {
	ChatCompletionTask(context.Context, *CompletionRequest, *Chunker) (<-chan []byte, <-chan *ToolCall, <-chan *ai.ChatCompletionMessage)
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
	Tools       []Tool
}

func NewCompletionRequest(config *Configuration, session Session, tools []Tool) *CompletionRequest {

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
	sys := ctx.GetSystem()
	// Get all tools from registry
	var tools []Tool
	if sys.GetToolRegistry() != nil {
		tools = sys.GetToolRegistry().All()
	}
	req := NewCompletionRequest(config, session, tools)
	llm := sys.GetLLM()

    textChan, toolChan, msgChan := llm.ChatCompletionTask(ctx, req, NewChunker(config))

	outputChan := make(chan string, 10)

    go func() {
        defer close(outputChan)

        for {
            select {
            case _, ok := <-toolChan:
                // Consume toolChan to avoid blocking the producer; we will
                // execute tool calls based on the assistant message we receive
                // on msgChan which contains the authoritative ToolCalls.
                if !ok {
                    toolChan = nil
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
                    log.Printf("complete: received message from msgChan, role: %s, content length: %d, tool calls: %d", 
                        msg.Role, len(msg.Content), len(msg.ToolCalls))
                    // Persist the assistant message first.
                    session.AddMessage(*msg)

                    // If the assistant included content alongside tool calls,
                    // emit that content before executing the tools to preserve
                    // conversational order. For non-tool messages, content is
                    // emitted via textChan already.
                    if len(msg.ToolCalls) > 0 && msg.Content != "" {
                        outputChan <- msg.Content
                    }

                    // If the message included tool calls, execute them now based
                    // on the tool call data embedded in the message itself. This
                    // avoids races between toolChan and msgChan arrival order.
                    if len(msg.ToolCalls) > 0 {
                        for _, otc := range msg.ToolCalls {
                            if tc, err := ParseOpenAIToolCall(otc); err == nil {
                                toolch, _ := handleToolCall(ctx, tc)
                                for r := range toolch {
                                    outputChan <- r
                                }
                            } else {
                                log.Printf("complete: failed to parse tool call: %v", err)
                            }
                        }
                    }
                }
            }

            if toolChan == nil && textChan == nil && msgChan == nil {
                break
            }
        }
    }()

	return outputChan, nil
}

func handleToolCall(ctx ChatContextInterface, toolCall *ToolCall) (<-chan string, error) {
	log.Printf("Tool Call Received: %s(%v)", toolCall.Name, toolCall.Args)
	sys := ctx.GetSystem()
	tool, ok := sys.GetToolRegistry().Get(toolCall.Name)
	if !ok {
		log.Printf("Tool not found: %s", toolCall.Name)
		return nil, fmt.Errorf("tool not found: %s", toolCall.Name)
	}

	// Set context for contextual tools (like IRC tools)
	if contextualTool, ok := tool.(ContextualTool); ok {
		contextualTool.SetContext(ctx)
	}

	result, err := tool.Execute(toolCall.Args)
	if err != nil {
		log.Printf("error executing tool %s: %v", toolCall.Name, err)
		result = fmt.Sprintf("Error: %v", err)
	} else {
		// Log tool output (truncate if too long)
		outputPreview := result
		if len(outputPreview) > 200 {
			outputPreview = outputPreview[:200] + "..."
		}
		log.Printf("Tool %s output: %s", toolCall.Name, outputPreview)
	}

	// Add tool result linked to the initiating assistant tool call.
	ctx.GetSession().AddMessage(ai.ChatCompletionMessage{
		Role:       ai.ChatMessageRoleTool,
		Content:    result,
		ToolCallID: toolCall.ID,
	})
	return complete(ctx)
}
