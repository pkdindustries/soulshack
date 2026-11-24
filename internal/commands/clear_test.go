package commands

import (
	"strings"
	"testing"

	"github.com/alexschlessinger/pollytool/messages"

	"pkdindustries/soulshack/internal/memory"
	mocktest "pkdindustries/soulshack/internal/testing"
)

func TestClearCommand_ClearsConversationAndRequiresAdmin(t *testing.T) {
	sys := mocktest.NewMockSystem(t)
	reg := NewRegistry()
	reg.Register(&ClearCommand{})

	conv := sys.Conversation(t, "#test")
	if err := conv.Append([]messages.ChatMessage{{Role: messages.MessageRoleUser, Content: "hello"}}); err != nil {
		t.Fatal(err)
	}
	conv.SetUsage(memory.ContextUsage{EstimatedTokens: 50, Budget: 1000, Compacted: true})

	nonAdmin := mocktest.NewMockContext().WithSystem(sys).WithConversation(conv).WithAdmin(false).WithArgs("/clear")
	reg.Dispatch(nonAdmin.Turn())
	if !strings.Contains(nonAdmin.LastReply(), "permission") {
		t.Fatalf("non-admin reply = %q, want permission denial", nonAdmin.LastReply())
	}
	if conv.Len() != 1 {
		t.Fatalf("non-admin /clear modified history: len=%d", conv.Len())
	}

	admin := mocktest.NewMockContext().WithSystem(sys).WithConversation(conv).WithAdmin(true).WithArgs("/clear")
	reg.Dispatch(admin.Turn())
	if admin.LastReply() != "Conversation cleared." {
		t.Fatalf("admin reply = %q, want %q", admin.LastReply(), "Conversation cleared.")
	}
	if conv.Len() != 0 {
		t.Fatalf("history after /clear = %d, want 0", conv.Len())
	}
	if _, ok := conv.Usage(); ok {
		t.Fatal("usage was not reset after /clear")
	}
}

func TestSetCommand_DoesNotClearConversation(t *testing.T) {
	sys := mocktest.NewMockSystem(t)
	conv := sys.Conversation(t, "#test")
	if err := conv.Append([]messages.ChatMessage{{Role: messages.MessageRoleUser, Content: "keep me"}}); err != nil {
		t.Fatal(err)
	}

	ctx := mocktest.NewMockContext().WithSystem(sys).WithConversation(conv).WithAdmin(true).
		WithArgs("/set", "chunkmax", "300")
	(&SetCommand{}).Execute(ctx.Turn())

	if conv.Len() != 1 {
		t.Fatalf("/set cleared conversation history: len=%d, want 1", conv.Len())
	}
}

func TestRegistry_NeedsConversation(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&HelpCommand{registry: reg})
	reg.Register(&VersionCommand{Version: "v1"})
	reg.Register(&GetCommand{})
	reg.Register(&SetCommand{})
	reg.Register(&ToolsCommand{})
	reg.Register(&AdminCommand{})
	reg.Register(&AgentsCommand{})
	reg.Register(&StatsCommand{})
	reg.Register(&ClearCommand{})
	reg.Register(&CompletionCommand{})

	for _, stateless := range []string{"/help", "/version", "/get", "/set", "/tools", "/admins", "/agents"} {
		if reg.NeedsConversation(stateless) {
			t.Errorf("%s should not require a conversation turn", stateless)
		}
	}
	for _, stateful := range []string{"/stats", "/clear", "hello"} {
		if !reg.NeedsConversation(stateful) {
			t.Errorf("%q should require a conversation turn", stateful)
		}
	}
}
