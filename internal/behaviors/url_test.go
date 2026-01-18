package behaviors

import (
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
