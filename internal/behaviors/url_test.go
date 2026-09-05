package behaviors

import (
	"context"
	"github.com/alexschlessinger/pollytool/messages"
	"pkdindustries/soulshack/internal/core"
	"testing"

	"github.com/lrstanley/girc"

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

func TestSilentURLUsesTemporarySession(t *testing.T) {
	sys := mocktest.NewMockSystem(t)
	model := &mocktest.MockLLM{Responses: []string{"silent result"}}
	sys.LLM = model
	ctx := mocktest.NewMockContext().WithSystem(sys).WithAddressed(false)
	ctx.GetConfig().Bot.URLWatcherSilent = true
	core.WithConversation(ctx, "seed", func(turn core.ChatContextInterface) {
		if err := turn.GetSession().AddMessage(turn, messages.ChatMessage{Role: messages.MessageRoleUser, Content: "channel history"}); err != nil {
			t.Fatal(err)
		}
	}, nil)
	behavior := &URLBehavior{}
	behavior.Execute(ctx, &girc.Event{Command: girc.PRIVMSG, Params: []string{"#test", "https://example.com"}})
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
	keys, err := sys.Sessions.List(context.Background())
	if err != nil || len(keys) != 1 || keys[0] != ctx.GetLockKey() {
		t.Fatalf("temporary session leaked: %v, %v", keys, err)
	}
	core.WithConversation(ctx, "verify", func(turn core.ChatContextInterface) {
		history, err := turn.GetSession().GetHistory(turn)
		if err != nil || history[len(history)-1].Content != "channel history" {
			t.Fatalf("silent URL changed channel history: %v, %v", history, err)
		}
	}, nil)
}
