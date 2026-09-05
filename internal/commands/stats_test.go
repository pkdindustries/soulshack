package commands

import (
	"context"
	"strings"
	"testing"

	"github.com/alexschlessinger/pollytool/messages"
	"pkdindustries/soulshack/internal/core"
	mocktest "pkdindustries/soulshack/internal/testing"
)

func TestStatsSeparatesProjectedInputFromRetainedHistory(t *testing.T) {
	sys := mocktest.NewMockSystem(t)
	session := sys.AcquireSession(t, "stats")
	ctx := mocktest.NewMockContext().WithSystem(sys).WithSession(session)
	generated := []messages.ChatMessage{{Role: messages.MessageRoleAssistant, Content: "reply"}}
	generated[0].SetTokenUsage(12345, 42)
	core.RecordContextUsage(generated, core.ContextUsage{EstimatedTokens: 800, Budget: 1000, OmittedExchanges: 3})
	if err := session.AddMessage(ctx, messages.ChatMessage{Role: messages.MessageRoleUser, Content: strings.Repeat("old history ", 1000)}); err != nil {
		t.Fatal(err)
	}
	if err := session.AddMessages(ctx, generated); err != nil {
		t.Fatal(err)
	}
	cmd := &StatsCommand{}
	cmd.Execute(ctx)
	for _, want := range []string{"total token input: 12345", "total token output: 42", "last completed input: ~800/1000 tokens (3 older exchanges omitted)", "stored messages: 3", "idle expiry: after 10m idle"} {
		if !ctx.HasReply(want) {
			t.Errorf("stats missing %q: %v", want, ctx.Replies)
		}
	}
	if err := session.Clear(context.Background()); err != nil {
		t.Fatal(err)
	}
	cmd.Execute(ctx)
	if !strings.Contains(ctx.LastReply(), "last completed input: no completed request") {
		t.Fatalf("clear left stale projection: %s", ctx.LastReply())
	}
}
