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
	conversation.SetUsage(memory.ContextUsage{EstimatedTokens: 800, Budget: 1000, Compacted: true})

	cmd := &StatsCommand{}
	cmd.Execute(ctx.Turn())
	for _, want := range []string{"total token input: 12345", "total token output: 42", "last completed input: ~800/1000 tokens (older exchanges compacted)", "stored messages: 2", "idle expiry: after 10m idle"} {
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

func TestStatsExcludesInternalCompactionMarkersAndNotesCompactedRestart(t *testing.T) {
	sys := mocktest.NewMockSystem(t)
	conversation := sys.Conversation(t, "stats-compacted")
	ctx := mocktest.NewMockContext().WithSystem(sys).WithConversation(conversation)

	if err := conversation.Append([]messages.ChatMessage{
		{Role: messages.MessageRoleUser, Content: "(nick:alice) hi"},
		messages.Compaction{Summary: "older summary"}.Message(),
		{Role: messages.MessageRoleAssistant, Content: "hello"},
	}); err != nil {
		t.Fatal(err)
	}

	// No Usage recorded yet (as right after restart).
	(&StatsCommand{}).Execute(ctx.Turn())
	reply := ctx.LastReply()
	for _, want := range []string{
		"last completed input: no completed request (older exchanges compacted)",
		"stored messages: 2 (user: 1, assistant: 1, tool: 0)",
	} {
		if !strings.Contains(reply, want) {
			t.Errorf("stats reply %q missing %q", reply, want)
		}
	}
}
