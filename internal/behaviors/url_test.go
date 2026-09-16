package behaviors

import (
	"github.com/alexschlessinger/pollytool/messages"
	"pkdindustries/soulshack/internal/core"
	"testing"
	"time"

	"github.com/lrstanley/girc"

	pollyllm "github.com/alexschlessinger/pollytool/llm"
	"github.com/alexschlessinger/pollytool/subagent"

	"pkdindustries/soulshack/internal/subagents"

	"pkdindustries/soulshack/internal/config"
	mocktest "pkdindustries/soulshack/internal/testing"
)

func TestURLBehavior_Check_BasicURL(t *testing.T) {
	behavior := &URLBehavior{}

	tests := []struct {
		name    string
		message string
		want    bool
	}{
		{"http URL", "http://example.com", true},
		{"https URL", "https://example.com/path", true},
		{"https with query", "https://example.com?foo=bar", true},
		{"https with fragment", "https://example.com#section", true},
		{"URL mid-message", "check out https://example.com please", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := mocktest.NewMockContext().
				WithURLWatcher(true).
				WithAddressed(false)

			event := &girc.Event{
				Command: girc.PRIVMSG,
				Params:  []string{"#test", tt.message},
			}

			got := behavior.Check(ctx, event)
			if got != tt.want {
				t.Errorf("URLBehavior.Check(%q) = %v, want %v", tt.message, got, tt.want)
			}
		})
	}
}

func TestURLBehavior_Check_URLWatcherDisabled(t *testing.T) {
	behavior := &URLBehavior{}

	ctx := mocktest.NewMockContext().
		WithURLWatcher(false).
		WithAddressed(false)

	event := &girc.Event{
		Command: girc.PRIVMSG,
		Params:  []string{"#test", "https://example.com"},
	}

	got := behavior.Check(ctx, event)
	if got != false {
		t.Error("expected false when URLWatcher is disabled")
	}
}

func TestURLBehavior_Check_AddressedMessage(t *testing.T) {
	behavior := &URLBehavior{}

	ctx := mocktest.NewMockContext().
		WithURLWatcher(true).
		WithAddressed(true)

	event := &girc.Event{
		Command: girc.PRIVMSG,
		Params:  []string{"#test", "https://example.com"},
	}

	got := behavior.Check(ctx, event)
	if got != false {
		t.Error("expected false when message is addressed to bot")
	}
}

func TestURLBehavior_Check_NoURL(t *testing.T) {
	behavior := &URLBehavior{}

	tests := []struct {
		name    string
		message string
	}{
		{"plain text", "hello world"},
		{"empty message", ""},
		{"URL-like text", "example.com/path"},
		{"ftp URL", "ftp://files.example.com"},
		{"malformed", "http:/missing-slash.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := mocktest.NewMockContext().
				WithURLWatcher(true).
				WithAddressed(false)

			event := &girc.Event{
				Command: girc.PRIVMSG,
				Params:  []string{"#test", tt.message},
			}

			got := behavior.Check(ctx, event)
			if got != false {
				t.Errorf("URLBehavior.Check(%q) = %v, want false", tt.message, got)
			}
		})
	}
}

func TestURLBehavior_Events(t *testing.T) {
	behavior := &URLBehavior{}
	events := behavior.Events()

	if len(events) != 1 || events[0] != girc.PRIVMSG {
		t.Errorf("URLBehavior.Events() = %v, want [%s]", events, girc.PRIVMSG)
	}
}

func TestURLBehavior_Name(t *testing.T) {
	behavior := &URLBehavior{}
	if behavior.Name() != "url" {
		t.Errorf("URLBehavior.Name() = %q, want %q", behavior.Name(), "url")
	}
}

// settled waits for a background observation, with a bound so a broken one
// fails the test instead of hanging it.
func settled(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the observation never finished")
	}
}

func TestSilentURLUsesTemporaryConversation(t *testing.T) {
	sys := mocktest.NewMockSystem(t)
	model := &mocktest.MockLLM{Responses: []string{"silent result"}}
	sys.LLM = model
	ctx := mocktest.NewMockContext().WithSystem(sys).WithAddressed(false)
	mocktest.SetConfig(t, sys, func(c *config.Configuration) { c.Bot.URLWatcherSilent = true })
	core.WithConversation(ctx, "seed", func(turn *core.Turn) {
		turn.Conversation.Append([]messages.ChatMessage{{Role: messages.MessageRoleUser, Content: "channel history"}})
	}, nil)
	behavior := &URLBehavior{}
	settled(t, behavior.execute(ctx, &girc.Event{Command: girc.PRIVMSG, Params: []string{"#test", "https://example.com"}}))
	if model.LastRequest == nil {
		t.Fatal("silent URL did not run")
	}
	for _, msg := range model.LastRequest.Messages {
		if msg.Content == "channel history" {
			t.Fatal("silent URL received channel history")
		}
	}
	if ctx.ReplyCount() != 0 {
		t.Fatalf("silent URL replied: %v", ctx.Replies)
	}
	keys := sys.Memory.Keys()
	if len(keys) != 1 || keys[0] != ctx.GetConversationKey() {
		t.Fatalf("temporary conversation leaked: %v", keys)
	}
	core.WithConversation(ctx, "verify", func(turn *core.Turn) {
		history := turn.Conversation.Messages()
		if len(history) != 1 || history[0].Content != "channel history" {
			t.Fatalf("silent URL changed channel history: %v", history)
		}
	}, nil)
}

// Nobody asked the bot to read the link, so the turn that reads it cannot
// decide to spend more on it. It is also the one turn that may be running on a
// scratch transcript, where a child's report would have nowhere to go but the
// channel it was supposed to stay out of.
func TestURLObservationCannotDelegate(t *testing.T) {
	for _, silent := range []bool{true, false} {
		sys := mocktest.NewMockSystem(t)
		model := &mocktest.MockLLM{Responses: []string{"that link is about birds"}}
		sys.LLM = model
		subagents.Register(sys.ToolRegistry, subagents.NewTracker())
		ctx := mocktest.NewMockContext().WithSystem(sys).WithAddressed(false)
		mocktest.SetConfig(t, sys, func(c *config.Configuration) { c.Bot.URLWatcherSilent = silent })

		behavior := &URLBehavior{}
		settled(t, behavior.execute(ctx, &girc.Event{Command: girc.PRIVMSG, Params: []string{"#test", "https://example.com"}}))

		if model.LastRequest == nil {
			t.Fatalf("silent=%v: the observation did not run", silent)
		}
		for _, tool := range model.LastRequest.Tools {
			if tool.GetName() == subagent.ToolName {
				t.Fatalf("silent=%v: a URL observation was offered the spawning tool", silent)
			}
		}
	}
}

// The event handler must not wait on the reading. A link someone pasted is not
// a reason for the bot to stop answering the channel.
func TestURLObservationDoesNotHoldTheHandler(t *testing.T) {
	sys := mocktest.NewMockSystem(t)
	reading := make(chan struct{})
	release := make(chan struct{})
	sys.LLM = &blockingLLM{reading: reading, release: release}
	ctx := mocktest.NewMockContext().WithSystem(sys).WithAddressed(false)

	behavior := &URLBehavior{}
	done := behavior.execute(ctx, &girc.Event{Command: girc.PRIVMSG, Params: []string{"#test", "https://example.com"}})

	// execute has already returned while the model is still reading.
	<-reading
	select {
	case <-done:
		t.Fatal("the observation finished before the model did")
	default:
	}
	close(release)
	settled(t, done)
}

// blockingLLM holds a completion open until it is released.
type blockingLLM struct {
	mocktest.MockLLM
	reading, release chan struct{}
}

func (b *blockingLLM) ChatCompletionStream(turn *core.Turn, req *pollyllm.CompletionRequest) <-chan string {
	out := make(chan string)
	go func() {
		defer close(out)
		close(b.reading)
		<-b.release
	}()
	return out
}
