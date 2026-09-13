package llm

import (
	"context"
	"strings"
	"testing"

	polly "github.com/alexschlessinger/pollytool/llm"
	"github.com/alexschlessinger/pollytool/messages"
	"pkdindustries/soulshack/internal/config"
	"pkdindustries/soulshack/internal/core"
	"pkdindustries/soulshack/internal/memory"
	mocktest "pkdindustries/soulshack/internal/testing"
)

func TestCompletionWarnsWhenContextUsageIncreases(t *testing.T) {
	sys := mocktest.NewMockSystem(t)
	sys.Memory.SetBudget(1000)
	ctx := mocktest.NewMockContext().WithSystem(sys)
	sys.LLM = &PollyLLM{client: completionFunc(func(context.Context, *polly.CompletionRequest) messages.ChatMessage {
		return messages.ChatMessage{Role: messages.MessageRoleAssistant, Content: "done", StopReason: messages.StopReasonEndTurn}
	})}

	for _, tc := range []struct {
		promptBytes int
		want        string
	}{
		{2000, ""},
		{3200, "Model input reached 75% of its context budget"},
		{3200, ""},
		{3600, "Model input reached 90% of its context budget"},
		{3600, ""},
	} {
		mocktest.SetConfig(t, sys, func(c *config.Configuration) { c.Bot.Prompt = strings.Repeat("a", tc.promptBytes) })
		ctx.Actions = nil
		core.WithConversation(ctx, "complete", func(turn *core.Turn) {
			for range Complete(turn, "hello") {
			}
		}, nil)
		if tc.want == "" {
			if len(ctx.Actions) != 0 {
				t.Fatalf("unexpected warnings: %v", ctx.Actions)
			}
		} else if len(ctx.Actions) != 1 || ctx.Actions[0] != tc.want {
			t.Fatalf("warnings = %v, want %q", ctx.Actions, tc.want)
		}
	}
}

func TestContextUsageWarningsFollowProjection(t *testing.T) {
	for _, tc := range []struct {
		name              string
		previous, current memory.ContextUsage
		want              string
	}{
		{"below threshold", memory.ContextUsage{}, memory.ContextUsage{EstimatedTokens: 500, Budget: 1000}, ""},
		{"75 percent", memory.ContextUsage{}, memory.ContextUsage{EstimatedTokens: 760, Budget: 1000}, "Model input reached 75% of its context budget"},
		{"90 percent", memory.ContextUsage{EstimatedTokens: 760, Budget: 1000}, memory.ContextUsage{EstimatedTokens: 910, Budget: 1000}, "Model input reached 90% of its context budget"},
		{"no repeat", memory.ContextUsage{EstimatedTokens: 760, Budget: 1000}, memory.ContextUsage{EstimatedTokens: 800, Budget: 1000}, ""},
		{"unlimited", memory.ContextUsage{}, memory.ContextUsage{EstimatedTokens: 100000}, ""},
		{"omitted history", memory.ContextUsage{}, memory.ContextUsage{EstimatedTokens: 500, Budget: 1000, OmittedExchanges: 4}, "Model input omitted 4 older exchanges"},
		{"no repeated omission", memory.ContextUsage{EstimatedTokens: 500, Budget: 1000, OmittedExchanges: 4}, memory.ContextUsage{EstimatedTokens: 500, Budget: 1000, OmittedExchanges: 5}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sys := mocktest.NewMockSystem(t)
			ctx := mocktest.NewMockContext().WithSystem(sys).WithConversation(sys.Conversation(t, "test"))
			turn := ctx.Turn()
			if tc.previous != (memory.ContextUsage{}) {
				turn.Conversation.SetUsage(tc.previous)
			}

			checkContextUsage(turn, tc.current)

			if tc.want == "" {
				if len(ctx.Actions) != 0 {
					t.Fatalf("unexpected warning: %v", ctx.Actions)
				}
			} else if len(ctx.Actions) != 1 || ctx.Actions[0] != tc.want {
				t.Fatalf("warning = %v, want %q", ctx.Actions, tc.want)
			}
		})
	}
}
