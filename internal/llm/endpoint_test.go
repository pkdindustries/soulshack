package llm

import (
	"context"
	"testing"

	polly "github.com/alexschlessinger/pollytool/llm"
	"github.com/alexschlessinger/pollytool/messages"
	"pkdindustries/soulshack/internal/config"
	"pkdindustries/soulshack/internal/core"
	mocktest "pkdindustries/soulshack/internal/testing"
)

func TestProviderEndpointOverridesIgnoreProviderCase(t *testing.T) {
	for _, tc := range []struct{ model, endpoint string }{
		{"openai/Local-Model", "http://127.0.0.1:8080/v1"},
		{"OpenAI/Local-Model", "http://127.0.0.1:8080/v1"},
		{"OPENAI/Local-Model", "http://127.0.0.1:8080/v1"},
		{"Ollama/Local-Model", "http://127.0.0.1:11434"},
		{"Anthropic/Other-Model", ""},
		{"openai", ""},
	} {
		t.Run(tc.model, func(t *testing.T) {
			sys := mocktest.NewMockSystem(t)
			mocktest.SetConfig(t, sys, func(c *config.Configuration) {
				c.Model.Model = tc.model
				c.Bot.SubagentModel = tc.model
				c.API.OpenAIURL = "http://127.0.0.1:8080/v1"
				c.API.OllamaURL = "http://127.0.0.1:11434"
			})
			calls := 0
			model := &PollyLLM{client: completionFunc(func(_ context.Context, req *polly.CompletionRequest) messages.ChatMessage {
				calls++
				if req.BaseURL != tc.endpoint {
					t.Errorf("BaseURL=%q, want %q", req.BaseURL, tc.endpoint)
				}
				if req.Model != tc.model {
					t.Errorf("model changed from %q to %q", tc.model, req.Model)
				}
				return messages.ChatMessage{Role: messages.MessageRoleAssistant, Content: "done", StopReason: messages.StopReasonEndTurn}
			})}
			sys.LLM = model
			ctx := mocktest.NewMockContext().WithSystem(sys)
			core.WithConversation(ctx, "completion", func(turn *core.Turn) {
				if !Drain(Complete(turn, "hello"), func(string) {}) {
					t.Error("completion did not answer")
				}
			}, func() { t.Fatal("turn did not start") })
			_, err := model.RunSubagent(context.Background(), core.SubagentSpec{
				Task: "hello", Registry: sys.ToolRegistry, Config: sys.GetConfig(),
			})
			if err != nil {
				t.Fatal(err)
			}
			if calls != 2 {
				t.Fatalf("provider calls=%d, want parent and child", calls)
			}
		})
	}
}
