package commands

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/alexschlessinger/pollytool/schema"
	"github.com/alexschlessinger/pollytool/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"pkdindustries/soulshack/internal/config"
	"pkdindustries/soulshack/internal/llm"
	mocktest "pkdindustries/soulshack/internal/testing"
)

func TestToolsCommand_ListEmpty(t *testing.T) {
	mockSys := mocktest.NewMockSystem(t)
	// Registry is empty by default

	ctx := mocktest.NewTurnContext(t, mockSys, "test").
		WithArgs("/tools", "list")

	cmd := &ToolsCommand{}
	cmd.Execute(ctx.Turn())

	if ctx.ReplyCount() != 1 {
		t.Fatalf("expected 1 reply, got %d", ctx.ReplyCount())
	}
	if !strings.Contains(ctx.LastReply(), "No tools") {
		t.Errorf("expected 'No tools' message, got: %s", ctx.LastReply())
	}
}

func TestToolsCommand_ListTools(t *testing.T) {
	mockSys := mocktest.NewMockSystem(t)

	// Register and load a mock tool with full namespaced name
	mockSys.ToolRegistry.RegisterNative("native__test_tool", func() tools.Tool {
		return &mockTool{name: "native__test_tool"}
	})
	// LoadToolAuto instantiates the native tool from the factory
	mockSys.ToolRegistry.LoadToolAuto("native__test_tool")

	ctx := mocktest.NewTurnContext(t, mockSys, "test").
		WithArgs("/tools", "list")

	cmd := &ToolsCommand{}
	cmd.Execute(ctx.Turn())

	if ctx.ReplyCount() != 1 {
		t.Fatalf("expected 1 reply, got %d", ctx.ReplyCount())
	}
	// Tools are grouped by namespace; native tools are prefixed with "native__"
	if !strings.Contains(ctx.LastReply(), "native") {
		t.Errorf("expected tool list to contain namespace 'native', got: %s", ctx.LastReply())
	}
}

func TestToolsCommand_AddRequiresAdmin(t *testing.T) {
	mockSys := mocktest.NewMockSystem(t)

	ctx := mocktest.NewTurnContext(t, mockSys, "test").
		WithAdmin(false).
		WithArgs("/tools", "add", "/some/path")

	cmd := &ToolsCommand{}
	cmd.Execute(ctx.Turn())

	if ctx.ReplyCount() != 1 {
		t.Fatalf("expected 1 reply, got %d", ctx.ReplyCount())
	}
	if !strings.Contains(ctx.LastReply(), "permission") {
		t.Errorf("expected permission error, got: %s", ctx.LastReply())
	}
}

func TestToolsCommand_RemoveRequiresAdmin(t *testing.T) {
	mockSys := mocktest.NewMockSystem(t)

	ctx := mocktest.NewTurnContext(t, mockSys, "test").
		WithAdmin(false).
		WithArgs("/tools", "remove", "some_tool")

	cmd := &ToolsCommand{}
	cmd.Execute(ctx.Turn())

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

func (t *mockTool) GetName() string                                             { return t.name }
func (t *mockTool) GetSchema() *schema.ToolSchema                               { return nil }
func (t *mockTool) GetType() string                                             { return "native" }
func (t *mockTool) GetSource() string                                           { return "test" }
func (t *mockTool) Execute(_ context.Context, _ map[string]any) (string, error) { return "", nil }

var _ tools.Tool = (*mockTool)(nil)

func TestToolsCommand_AddRefusesToolsThatOutgrowMaxContext(t *testing.T) {
	mockSys := mocktest.NewMockSystem(t)
	mockSys.ToolRegistry = tools.NewToolRegistry([]tools.Tool{}, tools.WithUnsafeNoSandbox(), tools.WithNativeTools())
	least := llm.MinContext(mockSys.GetConfig(), mockSys.GetToolRegistry())
	mocktest.SetConfig(t, mockSys, func(c *config.Configuration) { c.Session.MaxContext = least })

	ctx := mocktest.NewTurnContext(t, mockSys, "test").
		WithAdmin(true).
		WithArgs("/tools", "add", "bash")
	(&ToolsCommand{}).Execute(ctx.Turn())

	if !strings.HasPrefix(ctx.LastReply(), "Not added:") {
		t.Fatalf("reply = %q", ctx.LastReply())
	}
	if _, ok := mockSys.ToolRegistry.Get("bash"); ok {
		t.Fatal("the refused tool stayed loaded")
	}
}

func TestToolsCommand_Reload(t *testing.T) {
	for _, rejected := range []bool{true, false} {
		name := "fits"
		if rejected {
			name = "over budget"
		}
		t.Run(name, func(t *testing.T) {
			sys := mocktest.NewMockSystem(t)
			registry := sys.ToolRegistry
			defer registry.Close()
			description, reply := "Look up a value.", "original"
			registry.RegisterNative("example__lookup", func() tools.Tool {
				return lookupTool(description, reply)
			})
			if _, err := registry.LoadToolAuto("example__lookup"); err != nil {
				t.Fatal(err)
			}
			original, _ := registry.Get("example__lookup")
			least := llm.MinContext(sys.GetConfig(), registry)
			mocktest.SetConfig(t, sys, func(c *config.Configuration) { c.Session.MaxContext = least })
			reply = "replacement"
			if rejected {
				description = strings.Repeat("Large replacement schema. ", 1000)
			}

			ctx := mocktest.NewTurnContext(t, sys, "test").
				WithAdmin(true).
				WithArgs("/tools", "add", "example__lookup")
			(&ToolsCommand{}).Execute(ctx.Turn())

			prefix, want := "Added:", "replacement"
			if rejected {
				prefix, want = "Not added:", "original"
			}
			if !strings.HasPrefix(ctx.LastReply(), prefix) {
				t.Fatalf("reply = %q, want %q", ctx.LastReply(), prefix)
			}
			loaded, ok := registry.Get("example__lookup")
			if !ok {
				t.Fatal("reload removed the working tool")
			}
			if (loaded == original) != rejected {
				t.Fatalf("original instance retained = %t, want %t", loaded == original, rejected)
			}
			if got, err := loaded.Execute(context.Background(), nil); err != nil || got != want {
				t.Fatalf("tool result = %q, %v; want %q", got, err, want)
			}
		})
	}
}

func TestToolsCommand_ReloadSeesChangesWhileLoading(t *testing.T) {
	for _, update := range []string{"tools", "config"} {
		t.Run(update, func(t *testing.T) {
			sys := mocktest.NewMockSystem(t)
			registry := sys.ToolRegistry
			defer registry.Close()
			original := lookupTool("Look up a value.", "original")
			registry.Register(original)
			least := llm.MinContext(sys.GetConfig(), registry)
			mocktest.SetConfig(t, sys, func(c *config.Configuration) { c.Session.MaxContext = least })

			loading, resume := make(chan struct{}), make(chan struct{})
			registry.RegisterNative("example__lookup", func() tools.Tool {
				close(loading)
				<-resume
				return lookupTool("Look up a value.", "replacement")
			})
			ctx := mocktest.NewTurnContext(t, sys, "test").
				WithAdmin(true).
				WithArgs("/tools", "add", "example__lookup")
			done := make(chan struct{})
			go func() {
				defer close(done)
				(&ToolsCommand{}).Execute(ctx.Turn())
			}()
			<-loading
			// The old instance stays visible mid-load.
			visible, _ := registry.Get("example__lookup")
			unrelated := &tools.Func{Name: "other__tool", Desc: strings.Repeat("An unrelated tool. ", 1000)}
			if update == "tools" {
				registry.Register(unrelated)
			} else {
				mocktest.SetConfig(t, sys, func(c *config.Configuration) {
					c.Bot.Prompt = strings.Repeat("A larger prompt. ", 1000)
				})
			}
			close(resume)
			<-done

			if !strings.HasPrefix(ctx.LastReply(), "Not added:") {
				t.Fatalf("reply = %q", ctx.LastReply())
			}
			if current, _ := registry.Get("example__lookup"); current != original || visible != original {
				t.Fatal("the original was replaced during a rejected load")
			}
			if update == "tools" {
				if current, _ := registry.Get("other__tool"); current != unrelated {
					t.Fatal("rejected load reverted an unrelated registry update")
				}
			} else if !strings.HasPrefix(sys.GetConfig().Bot.Prompt, "A larger prompt.") {
				t.Fatal("rejected load reverted an unrelated config update")
			}
		})
	}
}

func TestToolsCommand_MCPReloadPreservesClientLifecycle(t *testing.T) {
	for _, rejected := range []bool{true, false} {
		name := "fits"
		if rejected {
			name = "over budget"
		}
		t.Run(name, func(t *testing.T) {
			server := mcp.NewServer(&mcp.Implementation{Name: "tools-test", Version: "1"}, nil)
			setDescription := func(description string) {
				server.AddTool(&mcp.Tool{
					Name: "lookup", Description: description,
					InputSchema: map[string]any{"type": "object"},
				}, func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
					return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "still usable"}}}, nil
				})
			}
			setDescription("Look up a value.")
			var closed atomic.Int32
			handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil)
			httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost && r.Header.Get("Mcp-Session-Id") == "" {
					active := 0
					for range server.Sessions() {
						active++
					}
					if active > 1 {
						t.Errorf("live reload started before its trial client closed: %d active sessions", active)
					}
				}
				if r.Method == http.MethodDelete {
					closed.Add(1)
				}
				handler.ServeHTTP(w, r)
			}))
			defer func() {
				httpServer.CloseClientConnections()
				httpServer.Close()
			}()
			data, err := json.Marshal(tools.MCPServersConfig{MCPServers: map[string]tools.MCPConfig{
				"example": {Transport: "streamable", URL: httpServer.URL},
			}})
			if err != nil {
				t.Fatal(err)
			}
			configPath := filepath.Join(t.TempDir(), "mcp.json")
			if err := os.WriteFile(configPath, data, 0600); err != nil {
				t.Fatal(err)
			}

			sys := mocktest.NewMockSystem(t)
			registry := sys.ToolRegistry
			defer registry.Close()
			if _, err := registry.LoadToolAuto(configPath); err != nil {
				t.Fatal(err)
			}
			original, _ := registry.Get("example__lookup")
			least := llm.MinContext(sys.GetConfig(), registry)
			mocktest.SetConfig(t, sys, func(c *config.Configuration) { c.Session.MaxContext = least })
			if rejected {
				setDescription(strings.Repeat("Large replacement schema. ", 1000))
			}
			ctx := mocktest.NewTurnContext(t, sys, "test").
				WithAdmin(true).
				WithArgs("/tools", "add", configPath)
			(&ToolsCommand{}).Execute(ctx.Turn())

			prefix, wantClosed := "Added:", int32(2) // trial and replaced original
			if rejected {
				prefix, wantClosed = "Not added:", 1 // trial only
			}
			if !strings.HasPrefix(ctx.LastReply(), prefix) {
				t.Fatalf("reply = %q, want %q", ctx.LastReply(), prefix)
			}
			loaded, ok := registry.Get("example__lookup")
			if !ok || (loaded == original) != rejected {
				t.Fatal("reload did not retain/replace the original as expected")
			}
			if got, err := loaded.Execute(context.Background(), nil); err != nil || !strings.Contains(got, "still usable") {
				t.Fatalf("tool result = %q, %v", got, err)
			}
			if got := closed.Load(); got != wantClosed {
				t.Fatalf("closed clients = %d, want %d", got, wantClosed)
			}
			registry.Remove("example__lookup")
			if got := closed.Load(); got != wantClosed+1 {
				t.Fatalf("remove did not close the retained client's connection: closed = %d", got)
			}
		})
	}
}

func lookupTool(description, reply string) *tools.Func {
	return &tools.Func{
		Name: "example__lookup",
		Desc: description,
		Run:  func(context.Context, tools.Args) (string, error) { return reply, nil },
	}
}
