package behaviors

import (
	"testing"

	"pkdindustries/soulshack/internal/commands"
	"pkdindustries/soulshack/internal/core"
	mocktest "pkdindustries/soulshack/internal/testing"
)

func TestStatelessCommandsBypassConversationQueueAndDoNotCreateSessions(t *testing.T) {
	sys := mocktest.NewMockSystem(t)
	reg := commands.NewRegistry()
	reg.Register(&commands.VersionCommand{Version: "v1.0.0"})
	reg.Register(&commands.GetCommand{})
	reg.Register(&commands.CompletionCommand{})

	// Running /version on a fresh key must not create a conversation entry in Memory.
	pm := mocktest.NewMockContext().WithSystem(sys).WithSource("stranger").WithPrivate(true).WithArgs("/version")
	dispatch(pm, "addressed", reg)
	if pm.LastReply() != "v1.0.0" {
		t.Fatalf("/version reply = %q, want v1.0.0", pm.LastReply())
	}
	if keys := sys.Memory.Keys(); len(keys) != 0 {
		t.Fatalf("stateless command opened conversation keys: %v", keys)
	}

	// While a turn holds #test's queue, stateless commands still answer immediately.
	holder := mocktest.NewMockContext().WithSystem(sys)
	core.WithConversation(holder, "busy", func(*core.Turn) {
		ctx := mocktest.NewMockContext().WithSystem(sys).WithArgs("/version")
		dispatch(ctx, "addressed", reg)
		if ctx.LastReply() != "v1.0.0" {
			t.Fatalf("stateless command blocked or failed while busy: %q", ctx.LastReply())
		}
	}, nil)
}
