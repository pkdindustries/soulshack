package testing

import (
	"time"

	"github.com/alexschlessinger/pollytool/llm"
	"github.com/alexschlessinger/pollytool/sessions"
	"github.com/alexschlessinger/pollytool/tools"

	"pkdindustries/soulshack/internal/config"
	"pkdindustries/soulshack/internal/core"
)

// MockLLM implements core.LLM for testing
type MockLLM struct {
	Responses []string      // Chunks to send
	Delay     time.Duration // Delay between chunks (0 = immediate)
	Error     error         // Error to return (sent as final chunk)
}

// ChatCompletionStream implements core.LLM
func (m *MockLLM) ChatCompletionStream(req *llm.CompletionRequest, ctx core.ChatContextInterface) <-chan []byte {
	ch := make(chan []byte, len(m.Responses)+1)
	go func() {
		defer close(ch)
		for _, resp := range m.Responses {
			if m.Delay > 0 {
				select {
				case <-time.After(m.Delay):
				case <-ctx.Done():
					return
				}
			}
			select {
			case <-ctx.Done():
				return
			case ch <- []byte(resp):
			}
		}
		if m.Error != nil {
			ch <- []byte("Error: " + m.Error.Error())
		}
	}()
	return ch
}

// Verify MockLLM implements core.LLM
var _ core.LLM = (*MockLLM)(nil)

// MockSystem implements core.System for testing
type MockSystem struct {
	ToolRegistry *tools.ToolRegistry
	SessionStore sessions.SessionStore
	LLM          core.LLM
}

// NewMockSystem creates a MockSystem with sensible defaults
func NewMockSystem() *MockSystem {
	return &MockSystem{
		ToolRegistry: tools.NewToolRegistry([]tools.Tool{}),
		SessionStore: sessions.NewSyncMapSessionStore(&sessions.Metadata{
			MaxHistory:   50,
			TTL:          time.Minute * 10,
			SystemPrompt: "You are a test bot.",
		}),
		LLM: &MockLLM{
			Responses: []string{"Hello from mock LLM"},
		},
	}
}

// GetToolRegistry implements core.System
func (m *MockSystem) GetToolRegistry() *tools.ToolRegistry {
	return m.ToolRegistry
}

// GetSessionStore implements core.System
func (m *MockSystem) GetSessionStore() sessions.SessionStore {
	return m.SessionStore
}

// GetLLM implements core.System
func (m *MockSystem) GetLLM() core.LLM {
	return m.LLM
}

// UpdateLLM implements core.System
func (m *MockSystem) UpdateLLM(cfg config.APIConfig) error {
	// No-op for tests
	return nil
}

// Verify MockSystem implements core.System
var _ core.System = (*MockSystem)(nil)
