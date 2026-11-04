package behaviors

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alexschlessinger/pollytool/tools"
	"github.com/lrstanley/girc"
	"pkdindustries/soulshack/internal/config"
	"pkdindustries/soulshack/internal/llm"
	mocktest "pkdindustries/soulshack/internal/testing"
)

func TestURLObservationContextWarningsRespectSilence(t *testing.T) {
	for _, silent := range []bool{false, true} {
		t.Run(fmt.Sprintf("silent=%v", silent), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"id":"test","object":"chat.completion","model":"test","choices":[{"index":0,"message":{"role":"assistant","content":"URL observation"},"finish_reason":"stop"}],"usage":{"prompt_tokens":8000,"completion_tokens":2,"total_tokens":8002}}`)
			}))
			defer server.Close()
			sys := mocktest.NewMockSystem(t)
			sys.Memory.SetBudget(10000)
			// Schemas count toward usage even when no tools execute.
			sys.ToolRegistry.Register(&tools.Func{Name: "fixture", Desc: strings.Repeat("a", 22000)})
			mocktest.SetConfig(t, sys, func(c *config.Configuration) {
				c.Bot.URLWatcherSilent = silent
				c.Model.Model = "openai/test"
				c.Model.Stream = false
				c.Session.MaxContext = 10000
				c.API.OpenAIURL = server.URL
				c.API.OpenAIKey = "test"
			})
			if err := llm.CheckContextBudget(sys.GetConfig(), sys.ToolRegistry); err != nil {
				t.Fatal(err)
			}
			sys.LLM = llm.NewPollyLLM(*sys.GetConfig().API)
			chat := mocktest.NewMockContext().WithSystem(sys).WithAddressed(false)
			for range 2 {
				settled(t, (&URLBehavior{}).execute(chat, &girc.Event{Command: girc.PRIVMSG, Params: []string{"#test", "https://example.com"}}))
			}
			if silent {
				if len(chat.AllActions()) != 0 || chat.ReplyCount() != 0 {
					t.Fatalf("silent observation spoke: replies=%q actions=%q", chat.AllReplies(), chat.AllActions())
				}
			} else {
				if chat.ReplyCount() != 2 {
					t.Fatalf("normal observations did not reply: %q", chat.AllReplies())
				}
				if actions := chat.AllActions(); len(actions) != 1 || actions[0] != "Model input reached 75% of its context budget" {
					t.Fatalf("normal warning changed: %q", actions)
				}
			}
		})
	}
}
