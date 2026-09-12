package commands

import (
	"strings"
	"testing"

	"github.com/alexschlessinger/pollytool/messages"
	"pkdindustries/soulshack/internal/memory"
	mocktest "pkdindustries/soulshack/internal/testing"
)

func TestStatsSeparatesProjectedInputFromRetainedHistory(t *testing.T) {
	sys := mocktest.NewMockSystem(t)
	conversation := sys.Conversation(t, "stats")
	ctx := mocktest.NewMockContext().WithSystem(sys).WithConversation(conversation)
	generated := []messages.ChatMessage{{Role: messages.MessageRoleAssistant, Content: "reply"}}
	generated[0].SetTokenUsage(12345, 42)
	conversation.Append([]messages.ChatMessage{{Role: messages.MessageRoleUser, Content: strings.Repeat("old history ", 1000)}})
	conversation.Append(generated)
	conversation.SetUsage(memory.ContextUsage{EstimatedTokens: 800, Budget: 1000, OmittedExchanges: 3})

	cmd := &StatsCommand{}
	cmd.Execute(ctx.Turn())
	for _, want := range []string{"total token input: 12345", "total token output: 42", "last completed input: ~800/1000 tokens (3 older exchanges omitted)", "stored messages: 2", "idle expiry: after 10m idle"} {
		if !ctx.HasReply(want) {
			t.Errorf("stats missing %q: %v", want, ctx.Replies)
		}
	}

	conversation.Clear()
	cmd.Execute(ctx.Turn())
	if !strings.Contains(ctx.LastReply(), "last completed input: no completed request") {
		t.Fatalf("clear left stale projection: %s", ctx.LastReply())
	}
	if !strings.Contains(ctx.LastReply(), "stored messages: 0") {
		t.Fatalf("clear left stored messages: %s", ctx.LastReply())
	}
}
