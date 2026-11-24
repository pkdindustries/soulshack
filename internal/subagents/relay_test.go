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
	"unicode/utf8"

	"pkdindustries/soulshack/internal/config"
	"pkdindustries/soulshack/internal/core"
	"pkdindustries/soulshack/internal/irc"
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

type chunkingContext struct {
	core.ChatContextInterface
	limit int
}

func (c *chunkingContext) NewChunkWriter(output chan<- string) core.ChunkWriter {
	return irc.NewChunker(output, c.limit)
}

func (c *chunkingContext) Background(timeout time.Duration) (core.ChatContextInterface, context.CancelFunc) {
	bg, cancel := c.ChatContextInterface.Background(timeout)
	return &chunkingContext{ChatContextInterface: bg, limit: c.limit}, cancel
}

func TestVerbatimFallbackFramesThroughChunkWriter(t *testing.T) {
	sys, base, _, _ := newFixture(t)
	// Unseeded conversation triggers immediate verbatim fallback.
	const limit = 32
	ctx := &chunkingContext{ChatContextInterface: base, limit: limit}
	report := "first line with café\nsecond line that is much longer than thirty-two bytes of UTF-8 text"

	deliver(ctx, core.SubagentSpec{Label: "lookup", Config: sys.GetConfig()}, report)

	replies := base.AllReplies()
	if len(replies) < 3 {
		t.Fatalf("expected multiple framed chunks, got %q", replies)
	}
	for i, chunk := range replies {
		if len(chunk) > limit {
			t.Errorf("chunk %d exceeds limit %d (%d bytes): %q", i, limit, len(chunk), chunk)
		}
		if strings.Contains(chunk, "\n") {
			t.Errorf("chunk %d contains newline: %q", i, chunk)
		}
		if !utf8.ValidString(chunk) {
			t.Errorf("chunk %d is invalid UTF-8: %q", i, chunk)
		}
	}
	joined := strings.Join(replies, " ")
	for _, want := range []string{"[lookup]", "café", "second", "UTF-8", "text"} {
		if !strings.Contains(joined, want) {
			t.Errorf("framed output missing %q: %q", want, joined)
		}
	}
}
