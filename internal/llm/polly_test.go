package llm

import (
	"context"
	"strings"
	"testing"
	"time"

	polly "github.com/alexschlessinger/pollytool/llm"
	"github.com/alexschlessinger/pollytool/messages"
	"github.com/alexschlessinger/pollytool/tools"
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

// The bot offers only the tools soulshack registered. Polly's private
// built-ins (read_transcript, view_image, and the artifact readers) are
// removed, and removing them must not disturb the caller's own tools.
func TestCreateAgentForRegistryDropsPollyBuiltins(t *testing.T) {
	registry := tools.NewToolRegistry([]tools.Tool{}, tools.WithUnsafeNoSandbox())
	registry.Register(&tools.Func{
		Name: "irc__action",
		Desc: "send an action",
		Run:  func(context.Context, tools.Args) (string, error) { return "ok", nil },
	})

	var offered []tools.Tool
	client := completionFunc(func(_ context.Context, req *polly.CompletionRequest) messages.ChatMessage {
		offered = req.Tools
		return messages.ChatMessage{Role: messages.MessageRoleAssistant, Content: "done", StopReason: messages.StopReasonEndTurn}
	})
	agent := CreateAgentForRegistry(client, registry, time.Second)
	defer agent.Close()
	if _, err := agent.Run(context.Background(), &polly.CompletionRequest{Messages: messages.User("hello")}, nil); err != nil {
		t.Fatal(err)
	}

	names := make(map[string]bool, len(offered))
	for _, tool := range offered {
		names[tool.GetName()] = true
	}
	for _, builtin := range polly.BuiltinToolNames() {
		if names[builtin] {
			t.Errorf("agent offered polly built-in %s", builtin)
		}
	}
	if !names["irc__action"] {
		t.Errorf("agent did not offer the caller's tools; got %v", names)
	}
	if _, ok := registry.Get("irc__action"); !ok {
		t.Error("agent removed a tool from the shared registry")
	}
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
