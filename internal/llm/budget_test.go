package llm

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"
	"testing"

	polly "github.com/alexschlessinger/pollytool/llm"
	"github.com/alexschlessinger/pollytool/messages"
	"github.com/alexschlessinger/pollytool/tools"

	"pkdindustries/soulshack/internal/config"
	"pkdindustries/soulshack/internal/core"
	mocktest "pkdindustries/soulshack/internal/testing"
)

func TestMinContextGrowsWithPromptAndTools(t *testing.T) {
	cfg := mocktest.DefaultTestConfig()
	registry := tools.NewToolRegistry([]tools.Tool{}, tools.WithUnsafeNoSandbox())
	base := MinContext(cfg, registry)
	if base <= messageFloorTokens+polly.PageFloorTokens {
		t.Fatalf("minimum %d leaves out polly's floor for the prompt and tools", base)
	}

	cfg.Bot.Prompt = strings.Repeat("be terse. ", 400)
	withPrompt := MinContext(cfg, registry)
	if withPrompt <= base {
		t.Fatalf("a longer prompt did not raise the minimum: %d -> %d", base, withPrompt)
	}

	registry.Register(&tools.Func{
		Name: "lookup",
		Desc: strings.Repeat("looks something up. ", 50),
		Run:  func(context.Context, tools.Args) (string, error) { return "", nil },
	})
	if withTool := MinContext(cfg, registry); withTool <= withPrompt {
		t.Fatalf("another tool did not raise the minimum: %d -> %d", withPrompt, withTool)
	}
}

func TestCheckContextBudget(t *testing.T) {
	cfg := mocktest.DefaultTestConfig()
	registry := tools.NewToolRegistry([]tools.Tool{}, tools.WithUnsafeNoSandbox())
	least := MinContext(cfg, registry)

	for budget, ok := range map[int]bool{0: true, least: true, least - 1: false, 1000: false} {
		cfg.Session.MaxContext = budget
		if err := CheckContextBudget(cfg, registry); (err == nil) != ok {
			t.Errorf("maxcontext %d (minimum %d): err = %v", budget, least, err)
		}
	}
}

// The message's share of the room has a floor and a ceiling in every case:
// half the room in between, never so much that no floor-sized page is left.
func TestMessageShare(t *testing.T) {
	const floor = 3000
	least := floor + messageFloorTokens + polly.PageFloorTokens
	for name, tc := range map[string]struct{ budget, want int }{
		"unlimited":      {0, polly.DefaultInlineToolResultTokens},
		"huge":           {1_000_000, polly.DefaultInlineToolResultTokens},
		"at the minimum": {least, messageFloorTokens},
		"below it":       {least - 500, messageFloorTokens},
		"in between":     {floor + 8000, 4000},
		"just above":     {least + 100, messageFloorTokens + 100},
	} {
		if got := messageShare(tc.budget, floor); got != tc.want {
			t.Errorf("%s: share of a %d budget = %d, want %d", name, tc.budget, got, tc.want)
		}
	}
}

func TestBoundMessageStoresWhatExceedsItsShare(t *testing.T) {
	sys := mocktest.NewMockSystem(t)
	store := sys.Conversation(t, "#chan").Artifacts()
	short := messages.ChatMessage{Role: messages.MessageRoleUser, Content: "(nick:alice) hello"}
	long := messages.ChatMessage{Role: messages.MessageRoleUser, Content: "HEAD " + strings.Repeat("report line\n", 3000) + " TAIL"}

	for name, tc := range map[string]struct {
		store   bool
		share   int
		msg     messages.ChatMessage
		bounded bool
	}{
		"short":         {true, 1000, short, false},
		"no store":      {false, 1000, long, false},
		"long":          {true, 1000, long, true},
		"at the floor":  {true, messageFloorTokens, long, true},
		"under ceiling": {true, polly.DefaultInlineToolResultTokens, long, false},
	} {
		t.Run(name, func(t *testing.T) {
			s := store
			if !tc.store {
				s = nil
			}
			got, err := boundMessage(context.Background(), s, tc.msg, tc.share)
			if err != nil {
				t.Fatal(err)
			}
			if !tc.bounded {
				if got.Content != tc.msg.Content || len(got.Parts) != 0 {
					t.Fatal("message changed though it needed no bound")
				}
				return
			}
			if tokens := messages.EstimateMessageTokens(got); tokens > tc.share {
				t.Fatalf("bounded message is %d tokens, over its share of %d", tokens, tc.share)
			}
			if !strings.HasPrefix(got.Content, "HEAD") || !strings.Contains(got.Content, "TAIL") || !strings.Contains(got.Content, "read_artifact") {
				t.Fatalf("bounded message lost its head, tail or receipt: %q", got.Content)
			}
			if len(got.Parts) != 1 || got.Parts[0].Artifact == nil {
				t.Fatalf("bounded message does not refer to its artifact: %+v", got.Parts)
			}
			r, err := store.Open(context.Background(), got.Parts[0].Artifact.ID)
			if err != nil {
				t.Fatal(err)
			}
			defer r.Close()
			if stored, _ := io.ReadAll(r); string(stored) != tc.msg.Content {
				t.Fatal("the artifact does not hold the whole message")
			}
		})
	}
}

var artifactID = regexp.MustCompile(`sha256:[0-9a-f]{64}`)

// At exactly MinContext, the worst a turn can bring still fits: a long history
// the budget cannot hold, an incoming message far over its share, and the
// model paging back through both, in parallel and one at a time, before
// answering. Reads beyond the room are refused, not overflowed.
func TestTurnAtMinContextNeverRunsOutOfRoom(t *testing.T) {
	sys := mocktest.NewMockSystem(t)
	cfg := sys.GetConfig()
	budget := MinContext(cfg, sys.GetToolRegistry())
	mocktest.SetConfig(t, sys, func(c *config.Configuration) { c.Session.MaxContext = budget })
	sys.Memory.SetBudget(budget)
	ctx := mocktest.NewMockContext().WithSystem(sys)

	calls := 0
	sys.LLM = &PollyLLM{client: completionFunc(func(_ context.Context, req *polly.CompletionRequest) messages.ChatMessage {
		calls++
		// Polly finishes a turn whose next batch has no room without
		// tools; the answer is all there is to give then.
		if len(req.Tools) == 0 {
			return messages.ChatMessage{Role: messages.MessageRoleAssistant, Content: "final answer", StopReason: messages.StopReasonEndTurn}
		}
		call := func(name, args string) messages.ChatMessage {
			return messages.ChatMessage{
				Role: messages.MessageRoleAssistant, StopReason: messages.StopReasonToolUse,
				ToolCalls: []messages.ChatMessageToolCall{{ID: fmt.Sprintf("call%d", calls), Name: name, Arguments: args}},
			}
		}
		switch calls {
		case 1:
			batch := call("read_transcript", `{}`)
			batch.ToolCalls = append(batch.ToolCalls,
				messages.ChatMessageToolCall{ID: "call1b", Name: "read_transcript", Arguments: `{"offset":60}`},
				messages.ChatMessageToolCall{ID: "call1c", Name: "read_transcript", Arguments: `{"offset":120}`},
			)
			return batch
		case 2:
			return call("read_transcript", `{"offset":150}`)
		case 3:
			last := req.Messages[len(req.Messages)-1]
			for i := len(req.Messages) - 1; i >= 0; i-- {
				if req.Messages[i].Role == messages.MessageRoleUser {
					last = req.Messages[i]
					break
				}
			}
			id := artifactID.FindString(last.Content)
			if id == "" {
				t.Errorf("the long message was sent without a receipt: %.200q", last.Content)
			}
			return call("read_artifact", fmt.Sprintf(`{"id":%q}`, id))
		}
		return messages.ChatMessage{Role: messages.MessageRoleAssistant, Content: "final answer", StopReason: messages.StopReasonEndTurn}
	})}

	core.WithConversation(ctx, "seed", func(turn *core.Turn) {
		for i := range 20 {
			turn.Conversation.Append([]messages.ChatMessage{
				{Role: messages.MessageRoleUser, Content: fmt.Sprintf("(nick:alice) message %d %s", i, strings.Repeat("words ", 400))},
				{Role: messages.MessageRoleAssistant, Content: strings.Repeat("reply ", 200)},
			})
		}
	}, nil)

	var chunks []string
	core.WithConversation(ctx, "complete", func(turn *core.Turn) {
		report := "(agent:research) " + strings.Repeat("finding with a long explanation\n", 4000)
		for chunk := range Complete(turn, report) {
			chunks = append(chunks, chunk)
		}
		usage, ok := turn.Conversation.Usage()
		if !ok || usage.EstimatedTokens > budget {
			t.Errorf("last request = %+v, recorded=%v, budget %d", usage, ok, budget)
		}
	}, nil)

	if calls < 2 || calls > 5 {
		t.Fatalf("model calls = %d; replies %q", calls, chunks)
	}
	if got := strings.Join(chunks, " "); !strings.Contains(got, "final answer") || strings.Contains(got, "Stopped") || strings.Contains(got, "Error") {
		t.Fatalf("turn did not finish cleanly: %q", got)
	}
}

// When a request cannot fit anyway, as when the prompt and tools alone
// outgrow maxcontext, the channel hears a plain account of it and what the
// turn did is kept.
func TestContextLimitIsReportedAndTheTurnKept(t *testing.T) {
	sys := mocktest.NewMockSystem(t)
	sys.Memory.SetBudget(2500)
	// Polly's schemas leave less than this prompt needs.
	mocktest.SetConfig(t, sys, func(c *config.Configuration) { c.Bot.Prompt = strings.Repeat("a", 8000) })
	ctx := mocktest.NewMockContext().WithSystem(sys)
	sys.LLM = &PollyLLM{client: completionFunc(func(context.Context, *polly.CompletionRequest) messages.ChatMessage {
		t.Error("a request over the budget reached the provider")
		return messages.ChatMessage{Role: messages.MessageRoleAssistant, Content: "unreachable", StopReason: messages.StopReasonEndTurn}
	})}

	var chunks []string
	core.WithConversation(ctx, "complete", func(turn *core.Turn) {
		for chunk := range Complete(turn, "(nick:alice) hello") {
			chunks = append(chunks, chunk)
		}
	}, nil)

	if got := strings.Join(chunks, " "); !strings.HasPrefix(got, "Stopped: this needs about") {
		t.Fatalf("reply = %q", got)
	}
	core.WithConversation(ctx, "verify", func(turn *core.Turn) {
		history := turn.Conversation.Messages()
		if len(history) != 1 || history[0].Content != "(nick:alice) hello" {
			t.Fatalf("stored transcript = %+v", history)
		}
	}, nil)
}

// windowClient is a provider whose context window is smaller than maxcontext:
// it rejects a request it counts over the window, in DeepSeek's words, and
// answers the rest.
type windowClient struct {
	window          int
	calls, rejected int
}

func (c *windowClient) ChatCompletionStream(ctx context.Context, req *polly.CompletionRequest, processor polly.EventStreamProcessor) <-chan *messages.StreamEvent {
	c.calls++
	input := 0
	for _, msg := range req.Messages {
		input += polly.EstimateMessageTokens(msg)
	}
	if input+req.MaxTokens > c.window {
		c.rejected++
		events := make(chan *messages.StreamEvent, 1)
		events <- &messages.StreamEvent{Type: messages.EventTypeError, Error: fmt.Errorf("openai api error 400 (invalid_request_error): This model's maximum context length is %d tokens. However, you requested %d tokens (%d in the messages, %d in the completion).", c.window, input+req.MaxTokens, input, req.MaxTokens)}
		close(events)
		return events
	}
	return completionFunc(func(context.Context, *polly.CompletionRequest) messages.ChatMessage {
		return messages.ChatMessage{Role: messages.MessageRoleAssistant, Content: "final answer", StopReason: messages.StopReasonEndTurn}
	}).ChatCompletionStream(ctx, req, processor)
}

// A model whose window is smaller than maxcontext rejects the request, and
// the turn still gets its answer from a smaller one.
func TestSmallerModelWindowIsAnsweredWithASmallerRequest(t *testing.T) {
	sys := mocktest.NewMockSystem(t)
	sys.Memory.SetBudget(40_000)
	ctx := mocktest.NewMockContext().WithSystem(sys)
	client := &windowClient{window: 6_000}
	sys.LLM = &PollyLLM{client: client}

	core.WithConversation(ctx, "seed", func(turn *core.Turn) {
		for i := range 20 {
			turn.Conversation.Append([]messages.ChatMessage{
				{Role: messages.MessageRoleUser, Content: fmt.Sprintf("(nick:alice) message %d %s", i, strings.Repeat("words ", 400))},
				{Role: messages.MessageRoleAssistant, Content: strings.Repeat("reply ", 200)},
			})
		}
	}, nil)

	var chunks []string
	core.WithConversation(ctx, "complete", func(turn *core.Turn) {
		for chunk := range Complete(turn, "(nick:alice) and now?") {
			chunks = append(chunks, chunk)
		}
	}, nil)

	if got := strings.Join(chunks, " "); !strings.Contains(got, "final answer") || strings.Contains(got, "Stopped") || strings.Contains(got, "Error") {
		t.Fatalf("reply = %q", got)
	}
	if client.calls != 2 || client.rejected != 1 {
		t.Fatalf("%d requests, %d rejected; want the first rejected and one retry", client.calls, client.rejected)
	}
}

// A turn that runs out of iterations keeps the tool calls it made.
func TestTurnThatRunsOutOfIterationsKeepsItsWork(t *testing.T) {
	sys := mocktest.NewMockSystem(t)
	sys.ToolRegistry.Register(&tools.Func{
		Name: "lookup",
		Desc: "look something up",
		Run:  func(context.Context, tools.Args) (string, error) { return "found it", nil },
	})
	ctx := mocktest.NewMockContext().WithSystem(sys)
	calls := 0
	sys.LLM = &PollyLLM{client: completionFunc(func(context.Context, *polly.CompletionRequest) messages.ChatMessage {
		calls++
		return messages.ChatMessage{
			Role: messages.MessageRoleAssistant, StopReason: messages.StopReasonToolUse,
			ToolCalls: []messages.ChatMessageToolCall{{ID: fmt.Sprintf("call%d", calls), Name: "lookup", Arguments: `{}`}},
		}
	})}

	core.WithConversation(ctx, "complete", func(turn *core.Turn) {
		for range Complete(turn, "(nick:alice) keep looking") {
		}
	}, nil)

	core.WithConversation(ctx, "verify", func(turn *core.Turn) {
		results := 0
		for _, msg := range turn.Conversation.Messages() {
			if msg.Role == messages.MessageRoleTool && msg.Content == "found it" {
				results++
			}
		}
		if results != turnMaxIterations {
			t.Fatalf("kept %d of %d tool results", results, turnMaxIterations)
		}
	}, nil)
}

// However the model runs its tool loop, a turn at MinContext answers: it may
// keep calling tools with ever larger arguments and ever more to say, and
// polly refuses or drops what the budget cannot take.
func TestToolLoopAtMinContextAlwaysAnswers(t *testing.T) {
	sys := mocktest.NewMockSystem(t)
	sys.ToolRegistry.Register(&tools.Func{
		Name: "lookup",
		Desc: "look something up",
		Run:  func(context.Context, tools.Args) (string, error) { return strings.Repeat("result ", 2000), nil },
	})
	budget := MinContext(sys.GetConfig(), sys.GetToolRegistry())
	sys.Memory.SetBudget(budget)
	ctx := mocktest.NewMockContext().WithSystem(sys)

	calls := 0
	sys.LLM = &PollyLLM{client: completionFunc(func(_ context.Context, req *polly.CompletionRequest) messages.ChatMessage {
		calls++
		if len(req.Tools) == 0 {
			return messages.ChatMessage{Role: messages.MessageRoleAssistant, Content: "final answer", StopReason: messages.StopReasonEndTurn}
		}
		response := messages.ChatMessage{Role: messages.MessageRoleAssistant, StopReason: messages.StopReasonToolUse, Content: strings.Repeat("still working ", 20*calls)}
		for i := range 3 {
			name, args := "lookup", fmt.Sprintf(`{"q":%q}`, strings.Repeat("q", 100*calls))
			if i == 1 {
				name, args = "read_transcript", `{}`
			}
			response.ToolCalls = append(response.ToolCalls, messages.ChatMessageToolCall{ID: fmt.Sprintf("c%d_%d", calls, i), Name: name, Arguments: args})
		}
		return response
	})}

	var chunks []string
	core.WithConversation(ctx, "complete", func(turn *core.Turn) {
		for chunk := range Complete(turn, "(nick:alice) dig into this") {
			chunks = append(chunks, chunk)
		}
	}, nil)
	if got := strings.Join(chunks, " "); !strings.Contains(got, "final answer") || strings.Contains(got, "Stopped") || strings.Contains(got, "Error") {
		t.Fatalf("turn at the minimum (%d tokens) did not answer cleanly after %d model calls: %q", budget, calls, got)
	}
}
