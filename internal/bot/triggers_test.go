package bot

import (
	"testing"

	mocktest "pkdindustries/soulshack/internal/testing"
)

func TestCheckURLTrigger_BasicURL(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    bool
	}{
		{"http URL", "http://example.com", true},
		{"https URL", "https://example.com/path", true},
		{"https with query", "https://example.com?foo=bar", true},
		{"https with fragment", "https://example.com#section", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := mocktest.NewMockContext().
				WithURLWatcher(true).
				WithAddressed(false)

			got := CheckURLTrigger(ctx, tt.message)
			if got != tt.want {
				t.Errorf("CheckURLTrigger(%q) = %v, want %v", tt.message, got, tt.want)
			}
		})
	}
}

func TestCheckURLTrigger_URLWatcherDisabled(t *testing.T) {
	ctx := mocktest.NewMockContext().
		WithURLWatcher(false).
		WithAddressed(false)

	got := CheckURLTrigger(ctx, "https://example.com")
	if got != false {
		t.Error("expected false when URLWatcher is disabled")
	}
}

func TestCheckURLTrigger_AddressedMessage(t *testing.T) {
	ctx := mocktest.NewMockContext().
		WithURLWatcher(true).
		WithAddressed(true)

	got := CheckURLTrigger(ctx, "https://example.com")
	if got != false {
		t.Error("expected false when message is addressed to bot")
	}
}

func TestCheckURLTrigger_NoURL(t *testing.T) {
	tests := []struct {
		name    string
		message string
	}{
		{"plain text", "hello world"},
		{"empty message", ""},
		{"URL mid-message", "check out https://example.com please"},
		{"URL-like text", "example.com/path"},
		{"ftp URL", "ftp://files.example.com"},
		{"malformed", "http:/missing-slash.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := mocktest.NewMockContext().
				WithURLWatcher(true).
				WithAddressed(false)

			got := CheckURLTrigger(ctx, tt.message)
			if got != false {
				t.Errorf("CheckURLTrigger(%q) = %v, want false", tt.message, got)
			}
		})
	}
}
