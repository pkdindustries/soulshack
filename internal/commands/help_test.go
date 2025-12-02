package commands

import (
	"strings"
	"testing"

	"pkdindustries/soulshack/internal/irc"
	mocktest "pkdindustries/soulshack/internal/testing"
)

func TestHelpCommand_ListsAllForAdmin(t *testing.T) {
	registry := NewRegistry()

	// Register both admin-only and public commands
	adminCmd := &testCommand{name: "/admin", adminOnly: true}
	publicCmd := &testCommand{name: "/public", adminOnly: false}

	registry.Register(adminCmd)
	registry.Register(publicCmd)

	helpCmd := NewHelpCommand(registry)

	ctx := mocktest.NewMockContext().
		WithAdmin(true).
		WithArgs("/help")

	helpCmd.Execute(ctx)

	if ctx.ReplyCount() != 1 {
		t.Fatalf("expected 1 reply, got %d", ctx.ReplyCount())
	}

	reply := ctx.LastReply()

	// Admin should see both commands
	if !strings.Contains(reply, "/admin") {
		t.Errorf("admin should see /admin command, got: %s", reply)
	}
	if !strings.Contains(reply, "/public") {
		t.Errorf("admin should see /public command, got: %s", reply)
	}
}

func TestHelpCommand_HidesAdminOnlyForNonAdmin(t *testing.T) {
	registry := NewRegistry()

	// Register both admin-only and public commands
	adminCmd := &testCommand{name: "/admin", adminOnly: true}
	publicCmd := &testCommand{name: "/public", adminOnly: false}

	registry.Register(adminCmd)
	registry.Register(publicCmd)

	helpCmd := NewHelpCommand(registry)

	ctx := mocktest.NewMockContext().
		WithAdmin(false).
		WithArgs("/help")

	helpCmd.Execute(ctx)

	if ctx.ReplyCount() != 1 {
		t.Fatalf("expected 1 reply, got %d", ctx.ReplyCount())
	}

	reply := ctx.LastReply()

	// Non-admin should NOT see admin command
	if strings.Contains(reply, "/admin") {
		t.Errorf("non-admin should NOT see /admin command, got: %s", reply)
	}
	// But should see public command
	if !strings.Contains(reply, "/public") {
		t.Errorf("non-admin should see /public command, got: %s", reply)
	}
}

// testCommand is a simple command for testing
type testCommand struct {
	name      string
	adminOnly bool
}

func (c *testCommand) Name() string                           { return c.name }
func (c *testCommand) AdminOnly() bool                        { return c.adminOnly }
func (c *testCommand) Execute(ctx irc.ChatContextInterface)   {}
