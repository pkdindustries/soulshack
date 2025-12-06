package commands

import (
	"context"
	"strings"
	"testing"

	"github.com/alexschlessinger/pollytool/tools"
	"github.com/google/jsonschema-go/jsonschema"

	mocktest "pkdindustries/soulshack/internal/testing"
)

func TestToolsCommand_ListEmpty(t *testing.T) {
	mockSys := mocktest.NewMockSystem()
	// Registry is empty by default

	ctx := mocktest.NewMockContext().
		WithSystem(mockSys).
		WithArgs("/tools")

	cmd := &ToolsCommand{}
	cmd.Execute(ctx)

	if ctx.ReplyCount() != 1 {
		t.Fatalf("expected 1 reply, got %d", ctx.ReplyCount())
	}
	if !strings.Contains(ctx.LastReply(), "No tools") {
		t.Errorf("expected 'No tools' message, got: %s", ctx.LastReply())
	}
}

func TestToolsCommand_ListTools(t *testing.T) {
	mockSys := mocktest.NewMockSystem()

	// Register and load a mock tool with full namespaced name
	mockSys.ToolRegistry.RegisterNative("native__test_tool", func() tools.Tool {
		return &mockTool{name: "native__test_tool"}
	})
	// LoadToolAuto instantiates the native tool from the factory
	mockSys.ToolRegistry.LoadToolAuto("native__test_tool")

	ctx := mocktest.NewMockContext().
		WithSystem(mockSys).
		WithArgs("/tools")

	cmd := &ToolsCommand{}
	cmd.Execute(ctx)

	if ctx.ReplyCount() != 1 {
		t.Fatalf("expected 1 reply, got %d", ctx.ReplyCount())
	}
	// Tools are grouped by namespace; native tools are prefixed with "native__"
	if !strings.Contains(ctx.LastReply(), "native") {
		t.Errorf("expected tool list to contain namespace 'native', got: %s", ctx.LastReply())
	}
}

func TestToolsCommand_AddRequiresAdmin(t *testing.T) {
	mockSys := mocktest.NewMockSystem()

	ctx := mocktest.NewMockContext().
		WithAdmin(false).
		WithSystem(mockSys).
		WithArgs("/tools", "add", "/some/path")

	cmd := &ToolsCommand{}
	cmd.Execute(ctx)

	if ctx.ReplyCount() != 1 {
		t.Fatalf("expected 1 reply, got %d", ctx.ReplyCount())
	}
	if !strings.Contains(ctx.LastReply(), "permission") {
		t.Errorf("expected permission error, got: %s", ctx.LastReply())
	}
}

func TestToolsCommand_RemoveRequiresAdmin(t *testing.T) {
	mockSys := mocktest.NewMockSystem()

	ctx := mocktest.NewMockContext().
		WithAdmin(false).
		WithSystem(mockSys).
		WithArgs("/tools", "remove", "some_tool")

	cmd := &ToolsCommand{}
	cmd.Execute(ctx)

	if ctx.ReplyCount() != 1 {
		t.Fatalf("expected 1 reply, got %d", ctx.ReplyCount())
	}
	if !strings.Contains(ctx.LastReply(), "permission") {
		t.Errorf("expected permission error, got: %s", ctx.LastReply())
	}
}

// mockTool implements tools.Tool for testing
type mockTool struct {
	name string
}

func (t *mockTool) GetName() string                                              { return t.name }
func (t *mockTool) GetSchema() *jsonschema.Schema                                { return nil }
func (t *mockTool) GetType() string                                              { return "native" }
func (t *mockTool) GetSource() string                                            { return "test" }
func (t *mockTool) Execute(_ context.Context, _ map[string]any) (string, error)  { return "", nil }

var _ tools.Tool = (*mockTool)(nil)
