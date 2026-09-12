package llm

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	polly "github.com/alexschlessinger/pollytool/llm"
	"github.com/alexschlessinger/pollytool/messages"
	"github.com/alexschlessinger/pollytool/tools"
	"github.com/alexschlessinger/pollytool/tools/sandbox"
	"pkdindustries/soulshack/internal/core"
	mocktest "pkdindustries/soulshack/internal/testing"
)

type completionFunc func(context.Context, *polly.CompletionRequest) messages.ChatMessage

func (f completionFunc) ChatCompletionStream(ctx context.Context, req *polly.CompletionRequest, processor polly.EventStreamProcessor) <-chan *messages.StreamEvent {
	input := make(chan messages.ChatMessage, 1)
	input <- f(ctx, req)
	close(input)
	return processor.ProcessMessagesToEvents(input)
}

func TestCreateAgentForRegistryTranscriptIsolation(t *testing.T) {
	registry := tools.NewToolRegistry(nil)
	calls := 0
	client := completionFunc(func(ctx context.Context, _ *polly.CompletionRequest) messages.ChatMessage {
		calls++
		if calls == 1 {
			// Run B while A is waiting for its first model response. A must
			// still read its own transcript when its tool call arrives.
			otherClient := completionFunc(func(context.Context, *polly.CompletionRequest) messages.ChatMessage {
				return messages.ChatMessage{Role: messages.MessageRoleAssistant, Content: "noted", StopReason: messages.StopReasonEndTurn}
			})
			otherAgent := CreateAgentForRegistry(otherClient, registry, time.Second)
			defer otherAgent.Close()
			if _, err := otherAgent.Run(ctx, &polly.CompletionRequest{Messages: messages.User("PRIVATE_B_TRANSCRIPT")}, nil); err != nil {
				t.Fatal(err)
			}
			return messages.ChatMessage{
				Role: messages.MessageRoleAssistant, StopReason: messages.StopReasonToolUse,
				ToolCalls: []messages.ChatMessageToolCall{{ID: "recall", Name: "read_transcript", Arguments: "{}"}},
			}
		}
		return messages.ChatMessage{Role: messages.MessageRoleAssistant, Content: "done", StopReason: messages.StopReasonEndTurn}
	})
	agent := CreateAgentForRegistry(client, registry, time.Second)
	defer agent.Close()
	response, err := agent.Run(context.Background(), &polly.CompletionRequest{Messages: messages.User("PRIVATE_A_TRANSCRIPT")}, nil)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, msg := range response.AllMessages {
		if msg.Role == messages.MessageRoleTool && msg.ToolName == "read_transcript" {
			found = true
			if !strings.Contains(msg.Content, "PRIVATE_A_TRANSCRIPT") || strings.Contains(msg.Content, "PRIVATE_B_TRANSCRIPT") {
				t.Fatalf("agent A read the wrong transcript: %s", msg.Content)
			}
		}
	}
	if !found {
		t.Fatal("agent A did not receive its transcript")
	}
	for _, name := range []string{"read_transcript", "view_image"} {
		if _, ok := registry.Get(name); ok {
			t.Errorf("agent added %s to the shared registry", name)
		}
	}
}

func TestCreateAgentForRegistryPreservesImageReadPolicy(t *testing.T) {
	path := filepath.Join(t.TempDir(), "private.png")
	if err := os.WriteFile(path, []byte("private file"), 0600); err != nil {
		t.Fatal(err)
	}
	registry := tools.NewToolRegistry(nil, tools.WithSandboxFactory(sandbox.New, sandbox.Config{DenyPaths: []string{path}}))
	args, err := json.Marshal(map[string]string{"source": path})
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	client := completionFunc(func(context.Context, *polly.CompletionRequest) messages.ChatMessage {
		calls++
		if calls == 1 {
			return messages.ChatMessage{
				Role: messages.MessageRoleAssistant, StopReason: messages.StopReasonToolUse,
				ToolCalls: []messages.ChatMessageToolCall{{ID: "image", Name: "view_image", Arguments: string(args)}},
			}
		}
		return messages.ChatMessage{Role: messages.MessageRoleAssistant, Content: "done", StopReason: messages.StopReasonEndTurn}
	})
	agent := CreateAgentForRegistry(client, registry, time.Second)
	defer agent.Close()
	response, err := agent.Run(context.Background(), &polly.CompletionRequest{Messages: messages.User("view the image")}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, msg := range response.AllMessages {
		if msg.Role == messages.MessageRoleTool && msg.ToolName == "view_image" {
			if !strings.Contains(msg.Content, "blocked from reads by the sandbox policy") {
				t.Fatalf("view_image did not enforce the read policy: %s", msg.Content)
			}
			return
		}
	}
	t.Fatal("agent did not execute view_image")
}

func TestPollyTrimsOldestExchangesAndKeepsFinalReply(t *testing.T) {
	sys := mocktest.NewMockSystem(t)
	ctx := mocktest.NewMockContext().WithSystem(sys)
	const budget = 2000
	sys.Memory.SetBudget(budget)
	modelCalls := 0
	sys.LLM = &PollyLLM{client: completionFunc(func(_ context.Context, req *polly.CompletionRequest) messages.ChatMessage {
		modelCalls++
		for _, msg := range req.Messages {
			if strings.Contains(msg.Content, "OLDEST_PRIVATE_HISTORY") {
				t.Error("oldest exchange was still sent to the provider")
			}
		}
		return messages.ChatMessage{Role: messages.MessageRoleAssistant, Content: "final reply", StopReason: messages.StopReasonEndTurn}
	})}

	core.WithConversation(ctx, "complete", func(turn *core.Turn) {
		for i := 0; i < 8; i++ {
			content := strings.Repeat("word ", 1000)
			if i == 0 {
				content = "OLDEST_PRIVATE_HISTORY " + content
			}
			turn.Conversation.Append([]messages.ChatMessage{
				{Role: messages.MessageRoleUser, Content: content},
				{Role: messages.MessageRoleAssistant, Content: "old reply"},
			})
		}
		for range Complete(turn, "new question") {
		}
	}, nil)

	if modelCalls != 1 {
		t.Fatalf("model calls = %d", modelCalls)
	}
	history := sys.Conversation(t, ctx.GetConversationKey()).Messages()
	if len(history) == 0 || history[len(history)-1].Content != "final reply" {
		t.Fatalf("final reply was lost: %d messages", len(history))
	}
	// Trimmed to what the budget holds, not the 19 messages that were written.
	if len(history) > 6 {
		t.Fatalf("transcript grew past its budget: %d messages", len(history))
	}
	// The question is written once: the turn appends the user message, then
	// the messages the model generated.
	questions := 0
	for _, msg := range history {
		if msg.Content == "new question" {
			questions++
		}
	}
	if questions != 1 {
		t.Fatalf("user message written %d times", questions)
	}
	for _, msg := range history {
		if strings.Contains(msg.Content, "OLDEST_PRIVATE_HISTORY") {
			t.Fatal("transcript kept an exchange the budget could not hold")
		}
	}

	usage, ok := sys.Conversation(t, ctx.GetConversationKey()).Usage()
	if !ok || usage.Budget != budget || usage.EstimatedTokens <= 0 {
		t.Fatalf("recorded projection = %+v, found=%v", usage, ok)
	}
	// The transcript is trimmed to the same number, so the projection has
	// nothing left to omit.
	if usage.OmittedExchanges != 0 {
		t.Fatalf("request was over budget with a trimmed transcript: %+v", usage)
	}
}
