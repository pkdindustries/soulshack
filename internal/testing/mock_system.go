package testing

import (
	"context"
	"testing"
	"time"

	"github.com/alexschlessinger/pollytool/llm"
	"github.com/alexschlessinger/pollytool/tools"

	"pkdindustries/soulshack/internal/config"
	"pkdindustries/soulshack/internal/core"
	"pkdindustries/soulshack/internal/memory"
)

// MockLLM implements core.LLM for testing
type MockLLM struct {
	Responses   []string               // Chunks to send
	Delay       time.Duration          // Delay between chunks (0 = immediate)
	Error       error                  // Error to return (sent as final chunk)
	LastRequest *llm.CompletionRequest // Captured by ChatCompletionStream

	// Subagent is what RunSubagent does. Nil returns a fixed reply at once,
	// which is enough for a test that only cares that a child ran.
	Subagent func(context.Context, core.SubagentSpec) (core.SubagentResult, error)
}

// RunSubagent implements core.LLM.
func (m *MockLLM) RunSubagent(ctx context.Context, spec core.SubagentSpec) (core.SubagentResult, error) {
	if m.Subagent != nil {
		return m.Subagent(ctx, spec)
	}
	return core.SubagentResult{Text: "mock agent reply"}, nil
}

// ChatCompletionStream implements core.LLM
func (m *MockLLM) ChatCompletionStream(turn *core.Turn, req *llm.CompletionRequest) <-chan string {
	m.LastRequest = req
	ch := make(chan string, len(m.Responses)+1)
	go func() {
		defer close(ch)
		for _, resp := range m.Responses {
			if m.Delay > 0 {
				select {
				case <-time.After(m.Delay):
				case <-turn.Done():
					return
				}
			}
			select {
			case <-turn.Done():
				return
			case ch <- resp:
			}
		}
		if m.Error != nil {
			ch <- "Error: " + m.Error.Error()
		}
	}()
	return ch
}

// Verify MockLLM implements core.LLM
var _ core.LLM = (*MockLLM)(nil)

// MockSystem implements core.System for testing
type MockSystem struct {
	ToolRegistry *tools.ToolRegistry
	Memory       *memory.Memory
	Config       *config.Store
	LLM          core.LLM
	// Agents is nil unless a test enables subagents, matching a bot started
	// without them.
	Agents core.Agents
}

// NewMockSystem creates a MockSystem with sensible defaults
func NewMockSystem(t testing.TB) *MockSystem {
	mem := memory.New(memory.Config{
		Budget: 100000,
		TTL:    time.Minute * 10,
	})
	t.Cleanup(mem.Close)
	return &MockSystem{
		ToolRegistry: tools.NewToolRegistry([]tools.Tool{}),
		Memory:       mem,
		Config:       config.NewStore(DefaultTestConfig()),
		LLM: &MockLLM{
			Responses: []string{"Hello from mock LLM"},
		},
	}
}

// GetToolRegistry implements core.System
func (m *MockSystem) GetToolRegistry() *tools.ToolRegistry {
	return m.ToolRegistry
}

// GetMemory implements core.System
func (m *MockSystem) GetMemory() *memory.Memory {
	return m.Memory
}

// GetConfig implements core.System
func (m *MockSystem) GetConfig() *config.Configuration {
	return m.Config.Snapshot()
}

// UpdateConfig implements core.System
func (m *MockSystem) UpdateConfig(fn func(*config.Configuration) error) error {
	return m.Config.Update(fn)
}

// GetLLM implements core.System
func (m *MockSystem) GetLLM() core.LLM {
	return m.LLM
}

// GetAgents implements core.System
func (m *MockSystem) GetAgents() core.Agents {
	return m.Agents
}

// UpdateLLM implements core.System
func (m *MockSystem) UpdateLLM(cfg config.APIConfig) error {
	// No-op for tests
	return nil
}

// Verify MockSystem implements core.System
var _ core.System = (*MockSystem)(nil)

// SetConfig changes the system's settings, the way /set does.
func SetConfig(t testing.TB, sys *MockSystem, fn func(*config.Configuration)) {
	t.Helper()
	if err := sys.UpdateConfig(func(c *config.Configuration) error {
		fn(c)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// NewTurnContext builds a context ready for command execution: a turn over a
// conversation in this system, which is what Execute needs.
func NewTurnContext(t testing.TB, sys *MockSystem, key string) *MockChatContext {
	t.Helper()
	return NewMockContext().WithSystem(sys).WithConversation(sys.Conversation(t, key))
}

// Conversation supplies a conversation for command-level tests, keyed like a
// turn's would be.
func (m *MockSystem) Conversation(t testing.TB, key string) *memory.Conversation {
	t.Helper()
	var conversation *memory.Conversation
	if !m.Memory.With(context.Background(), key, func(c *memory.Conversation) { conversation = c }) {
		t.Fatal("failed to open test conversation")
	}
	return conversation
}
