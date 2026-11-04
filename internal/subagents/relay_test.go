package subagents

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"pkdindustries/soulshack/internal/config"
	"pkdindustries/soulshack/internal/core"
	"pkdindustries/soulshack/internal/llm"
	mocktest "pkdindustries/soulshack/internal/testing"
)

func TestFailedRelayFallsBackEvenAfterPartialOutput(t *testing.T) {
	for _, partial := range []bool{false, true} {
		t.Run(fmt.Sprintf("partial=%v", partial), func(t *testing.T) {
			sys, ctx, model, _ := newFixture(t)
			seed(t, ctx)
			model.Responses = nil
			if partial {
				model.Responses = []string{"here is part of the answer"}
			}
			model.Error = errors.New("relay failed")
			deliver(ctx, core.SubagentSpec{Label: "lookup", Config: sys.GetConfig()}, "the complete child report")
			replies := ctx.AllReplies()
			if replies[len(replies)-1] != "[lookup] the complete child report" {
				t.Fatalf("report was not delivered: %q", replies)
			}
		})
	}
}

// Exercise the production stream, not just a mock that closes without output.
func TestFailedPollyRelayDeliversChildReport(t *testing.T) {
	for _, failure := range []string{"provider", "context limit", "canceled"} {
		t.Run(failure, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprint(w, `{"error":{"message":"relay rejected","type":"invalid_request_error","code":"invalid_request"}}`)
			}))
			defer server.Close()
			sys := mocktest.NewMockSystem(t)
			mocktest.SetConfig(t, sys, func(c *config.Configuration) {
				c.Model.Model = "openai/test"
				c.API.OpenAIURL = server.URL
				c.API.OpenAIKey = "test"
				c.API.Timeout = time.Second
				if failure == "context limit" {
					c.Bot.Prompt = strings.Repeat("a", 8000)
				}
			})
			if failure == "context limit" {
				sys.Memory.SetBudget(2500)
			}
			sys.LLM = llm.NewPollyLLM(*sys.GetConfig().API)
			ctx := mocktest.NewMockContext().WithSystem(sys).WithSource("alice")
			seed(t, ctx)
			if failure == "canceled" {
				parent, cancel := context.WithCancel(context.Background())
				cancel()
				ctx.Parent = parent
			}
			deliver(ctx, core.SubagentSpec{Label: "lookup", Config: sys.GetConfig()}, "the successful child report")
			replies := ctx.AllReplies()
			if len(replies) == 0 || replies[len(replies)-1] != "[lookup] the successful child report" {
				t.Fatalf("report was not delivered: %q", replies)
			}
			if failure != "canceled" {
				prefix := "Error:"
				if failure == "context limit" {
					prefix = "Stopped:"
				}
				if !strings.HasPrefix(replies[0], prefix) {
					t.Fatalf("expected %s notice before fallback, got %q", prefix, replies)
				}
			}
		})
	}
}
