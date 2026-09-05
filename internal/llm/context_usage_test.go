package llm

import (
	"testing"

	"github.com/alexschlessinger/pollytool/messages"
	"pkdindustries/soulshack/internal/core"
	mocktest "pkdindustries/soulshack/internal/testing"
)

func TestContextUsageWarningsFollowProjection(t *testing.T) {
	for _, tc := range []struct {
		name              string
		previous, current core.ContextUsage
		want              string
	}{
		{"below threshold", core.ContextUsage{}, core.ContextUsage{EstimatedTokens: 500, Budget: 1000}, ""},
		{"75 percent", core.ContextUsage{}, core.ContextUsage{EstimatedTokens: 760, Budget: 1000}, "Model input reached 75% of its context budget; stored history is retained"},
		{"90 percent", core.ContextUsage{EstimatedTokens: 760, Budget: 1000}, core.ContextUsage{EstimatedTokens: 910, Budget: 1000}, "Model input reached 90% of its context budget; stored history is retained"},
		{"no repeat", core.ContextUsage{EstimatedTokens: 760, Budget: 1000}, core.ContextUsage{EstimatedTokens: 800, Budget: 1000}, ""},
		{"unlimited", core.ContextUsage{}, core.ContextUsage{EstimatedTokens: 100000}, ""},
		{"omitted history", core.ContextUsage{}, core.ContextUsage{EstimatedTokens: 500, Budget: 1000, OmittedExchanges: 4}, "Model input omitted 4 older exchanges; stored history is retained"},
		{"no repeated omission", core.ContextUsage{EstimatedTokens: 500, Budget: 1000, OmittedExchanges: 4}, core.ContextUsage{EstimatedTokens: 500, Budget: 1000, OmittedExchanges: 5}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := mocktest.NewMockContext()
			history := []messages.ChatMessage{{Role: messages.MessageRoleAssistant, Content: "old reply"}}
			history[0].SetTokenUsage(1000000, 1000000)
			core.RecordContextUsage(history, tc.previous)
			checkContextUsage(ctx, tc.current, history)
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
