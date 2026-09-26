package llm

import (
	"context"
	"strings"
	"testing"
	"time"

	polly "github.com/alexschlessinger/pollytool/llm"
	"github.com/alexschlessinger/pollytool/messages"
	"github.com/alexschlessinger/pollytool/subagent"
	"github.com/alexschlessinger/pollytool/tools"
	"pkdindustries/soulshack/internal/config"
	"pkdindustries/soulshack/internal/core"
	mocktest "pkdindustries/soulshack/internal/testing"
)

type completionFunc func(context.Context, *polly.CompletionRequest) messages.ChatMessage

func (f completionFunc) ChatCompletionStream(ctx context.Context, req *polly.CompletionRequest, processor polly.EventStreamProcessor) <-chan *messages.StreamEvent {
	input := make(chan messages.ChatMessage, 1)
	input <- f(ctx, req)
	close(input)
	return processor.ProcessMessagesToEvents(ctx, input)
}

// When a request omits older exchanges, the marker it leaves points the model
// at read_transcript and the artifact readers, and a stored conversation's
// agent offers exactly those tools.
func TestCompletionOmissionRecommendsOfferedTools(t *testing.T) {
	sys := mocktest.NewMockSystem(t)
	sys.Memory.SetBudget(3000)
	// The system prompt and the old exchange together overflow the budget,
	// so projection omits the exchange from the request.
	mocktest.SetConfig(t, sys, func(c *config.Configuration) { c.Bot.Prompt = strings.Repeat("a", 6000) })
	ctx := mocktest.NewMockContext().WithSystem(sys)
	modelCalls := 0
	sys.LLM = &PollyLLM{client: completionFunc(func(_ context.Context, req *polly.CompletionRequest) messages.ChatMessage {
		modelCalls++
		offered := map[string]bool{}
		for _, tool := range req.Tools {
			offered[tool.GetName()] = true
		}
		recommended := map[string]bool{}
		for _, msg := range req.Messages {
			for _, name := range polly.BuiltinToolNames() {
				if strings.Contains(msg.Content, name) {
					recommended[name] = true
				}
			}
		}
		for name := range recommended {
			if !offered[name] {
				t.Errorf("projection recommends %s, which the request does not offer", name)
			}
		}
		if !recommended["read_transcript"] {
			t.Error("projection does not point at read_transcript for the omitted exchange")
		}
		return messages.ChatMessage{Role: messages.MessageRoleAssistant, Content: "done", StopReason: messages.StopReasonEndTurn}
	})}
	core.WithConversation(ctx, "seed", func(turn *core.Turn) {
		turn.Conversation.Append([]messages.ChatMessage{
			{Role: messages.MessageRoleUser, Content: strings.Repeat("old", 1600)},
			{Role: messages.MessageRoleAssistant, Content: "old answer"},
		})
	}, nil)
	for i := range 2 {
		ctx.Actions = nil
		core.WithConversation(ctx, "complete", func(turn *core.Turn) {
			for range Complete(turn, "new question") {
			}
			usage, ok := turn.Conversation.Usage()
			if !ok || usage.OmittedExchanges != 1 {
				t.Fatalf("usage = %+v, recorded=%v", usage, ok)
			}
		}, nil)
		if i == 0 {
			if len(ctx.Actions) != 1 || ctx.Actions[0] != "Model input omitted 1 older exchanges" {
				t.Fatalf("omission warning = %v", ctx.Actions)
			}
		} else if len(ctx.Actions) != 0 {
			t.Fatalf("omission warning repeated: %v", ctx.Actions)
		}
	}
	if modelCalls != 2 {
		t.Fatalf("model calls = %d, want 2", modelCalls)
	}
}

// Without an artifact store, as for a child, the agent offers only the tools
// soulshack registered. Polly's private built-ins are removed, and removing
// them must not disturb the caller's own tools.
func TestCreateAgentWithoutAStoreDropsPollyBuiltins(t *testing.T) {
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
	agent := CreateAgent(client, registry, polly.AgentConfig{MaxIterations: turnMaxIterations, ToolTimeout: time.Second})
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

// A stored conversation's agent gets polly's built-ins, so the model can open
// the receipts projection leaves for stored tool output and read back the
// exchanges its budget left out.
func TestCreateAgentWithAStoreOffersPollyBuiltins(t *testing.T) {
	sys := mocktest.NewMockSystem(t)
	var offered []tools.Tool
	client := completionFunc(func(_ context.Context, req *polly.CompletionRequest) messages.ChatMessage {
		offered = req.Tools
		return messages.ChatMessage{Role: messages.MessageRoleAssistant, Content: "done", StopReason: messages.StopReasonEndTurn}
	})
	conversation := sys.Conversation(t, "#chan")
	agent := CreateAgent(client, sys.ToolRegistry, polly.AgentConfig{MaxIterations: turnMaxIterations, ArtifactStore: conversation.Artifacts()})
	defer agent.Close()
	if _, err := agent.Run(context.Background(), &polly.CompletionRequest{Messages: messages.User("hello")}, nil); err != nil {
		t.Fatal(err)
	}

	names := make(map[string]bool, len(offered))
	for _, tool := range offered {
		names[tool.GetName()] = true
	}
	for _, builtin := range polly.BuiltinToolNames() {
		if !names[builtin] {
			t.Errorf("agent did not offer polly built-in %s; got %v", builtin, names)
		}
	}
}

// A stored conversation keeps every exchange; the budget bounds only what a
// request sends, so the oldest exchange stays in the transcript without
// reaching the provider.
func TestPollyOmitsOldestExchangesButKeepsThemStored(t *testing.T) {
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
	conversation := sys.Conversation(t, ctx.GetConversationKey())
	history := conversation.Messages()
	// The 16 seeded messages, the question, and the reply.
	if len(history) != 18 || history[len(history)-1].Content != "final reply" {
		t.Fatalf("stored transcript = %d messages, want 18 ending in the final reply", len(history))
	}
	if !strings.Contains(history[0].Content, "OLDEST_PRIVATE_HISTORY") {
		t.Fatal("the oldest exchange was dropped from the stored transcript")
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

	usage, ok := conversation.Usage()
	if !ok || usage.Budget != budget || usage.EstimatedTokens <= 0 || usage.EstimatedTokens > budget {
		t.Fatalf("recorded projection = %+v, found=%v", usage, ok)
	}
	if usage.OmittedExchanges == 0 {
		t.Fatalf("request fit the budget without omitting anything: %+v", usage)
	}
}

// The running commentary announces the spawning tool like any other, so with
// the setting on a delegation shows twice: the call, then what it delegated.
// irc__action stays skipped, since announcing it would print the message it
// is announcing.
func TestToolActionsAnnounceSpawningButNotActions(t *testing.T) {
	sys := mocktest.NewMockSystem(t)
	mocktest.SetConfig(t, sys, func(c *config.Configuration) { c.Bot.ShowToolActions = true })
	ctx := mocktest.NewMockContext().WithSystem(sys)

	core.WithConversation(ctx, "tools", func(turn *core.Turn) {
		handler := newCallbackHandler(turn, turn.NewChunkWriter(make(chan string, 1)), turn.GetConfig())
		handler.build().OnToolStart([]messages.ChatMessageToolCall{
			{Name: "irc__action"},
			{Name: subagent.ToolName},
			{Name: "irc__names"},
		})
	}, nil)

	actions := ctx.AllActions()
	if len(actions) != 1 || actions[0] != "calling "+subagent.ToolName+", names" {
		t.Fatalf("unexpected announcement: %q", actions)
	}
}
