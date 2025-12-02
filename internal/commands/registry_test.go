package commands

import (
	"strings"
	"testing"

	"pkdindustries/soulshack/internal/irc"
	mocktest "pkdindustries/soulshack/internal/testing"
)

// mockCommand is a simple test command
type mockCommand struct {
	name      string
	adminOnly bool
	executed  bool
}

func (c *mockCommand) Name() string    { return c.name }
func (c *mockCommand) AdminOnly() bool { return c.adminOnly }
func (c *mockCommand) Execute(ctx irc.ChatContextInterface) {
	c.executed = true
}

func TestRegistry_CommandRouting(t *testing.T) {
	registry := NewRegistry()

	setCmd := &mockCommand{name: "/set", adminOnly: true}
	getCmd := &mockCommand{name: "/get", adminOnly: false}

	registry.Register(setCmd)
	registry.Register(getCmd)

	// Test /set routing
	ctx := mocktest.NewMockContext().
		WithAdmin(true).
		WithArgs("/set", "key", "value")

	registry.Dispatch(ctx)

	if !setCmd.executed {
		t.Error("expected /set command to be executed")
	}
	if getCmd.executed {
		t.Error("expected /get command NOT to be executed")
	}

	// Reset and test /get routing
	setCmd.executed = false
	getCmd.executed = false

	ctx = mocktest.NewMockContext().
		WithArgs("/get", "key")

	registry.Dispatch(ctx)

	if setCmd.executed {
		t.Error("expected /set command NOT to be executed")
	}
	if !getCmd.executed {
		t.Error("expected /get command to be executed")
	}
}

func TestRegistry_AdminOnlyEnforcement(t *testing.T) {
	registry := NewRegistry()

	adminCmd := &mockCommand{name: "/admin", adminOnly: true}
	registry.Register(adminCmd)

	// Non-admin tries to call admin command
	ctx := mocktest.NewMockContext().
		WithAdmin(false).
		WithArgs("/admin")

	registry.Dispatch(ctx)

	if adminCmd.executed {
		t.Error("admin-only command should NOT be executed for non-admin")
	}
	if ctx.ReplyCount() != 1 {
		t.Fatalf("expected 1 reply (permission error), got %d", ctx.ReplyCount())
	}
	if !strings.Contains(ctx.LastReply(), "permission") {
		t.Errorf("expected permission error, got: %s", ctx.LastReply())
	}

	// Admin can call the command
	adminCmd.executed = false
	ctx = mocktest.NewMockContext().
		WithAdmin(true).
		WithArgs("/admin")

	registry.Dispatch(ctx)

	if !adminCmd.executed {
		t.Error("admin-only command should be executed for admin")
	}
}

func TestRegistry_DefaultCommand(t *testing.T) {
	registry := NewRegistry()

	defaultCmd := &mockCommand{name: "", adminOnly: false} // empty name = default
	registry.Register(defaultCmd)

	// Unknown command should fall through to default
	ctx := mocktest.NewMockContext().
		WithArgs("hello", "world") // not a slash command

	registry.Dispatch(ctx)

	if !defaultCmd.executed {
		t.Error("default command should be executed for unknown commands")
	}
}

func TestRegistry_AllReturnsCommands(t *testing.T) {
	registry := NewRegistry()

	cmd1 := &mockCommand{name: "/one"}
	cmd2 := &mockCommand{name: "/two"}
	cmd3 := &mockCommand{name: "/three"}
	defaultCmd := &mockCommand{name: ""} // default, should NOT be in All()

	registry.Register(cmd1)
	registry.Register(cmd2)
	registry.Register(cmd3)
	registry.Register(defaultCmd)

	all := registry.All()

	if len(all) != 3 {
		t.Errorf("expected 3 commands (excluding default), got %d", len(all))
	}

	// Verify all named commands are present
	names := make(map[string]bool)
	for _, cmd := range all {
		names[cmd.Name()] = true
	}

	for _, expected := range []string{"/one", "/two", "/three"} {
		if !names[expected] {
			t.Errorf("expected command %s in All()", expected)
		}
	}
}

func TestRegistry_GetCommand(t *testing.T) {
	registry := NewRegistry()

	cmd := &mockCommand{name: "/test"}
	registry.Register(cmd)

	// Existing command
	found, ok := registry.Get("/test")
	if !ok {
		t.Error("expected to find /test command")
	}
	if found.Name() != "/test" {
		t.Errorf("expected /test, got %s", found.Name())
	}

	// Non-existing command
	_, ok = registry.Get("/nonexistent")
	if ok {
		t.Error("expected NOT to find /nonexistent command")
	}
}
