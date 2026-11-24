package bot

import (
	"errors"
	"strings"
	"testing"

	"github.com/alexschlessinger/pollytool/tools/sandbox"

	mocktest "pkdindustries/soulshack/internal/testing"
)

func TestNewSystemFailsWhenSandboxUnavailable(t *testing.T) {
	orig := newSandbox
	newSandbox = func(sandbox.Config) (sandbox.Sandbox, error) {
		return nil, errors.New("bwrap missing")
	}
	t.Cleanup(func() { newSandbox = orig })

	cfg := mocktest.DefaultTestConfig()
	cfg.Bot.Sandbox = true
	sys, err := NewSystem(cfg)
	if err == nil {
		sys.GetMemory().Close()
		t.Fatal("expected NewSystem to fail when sandbox setup fails")
	}
	if !strings.Contains(err.Error(), "bwrap missing") {
		t.Fatalf("error = %v, want underlying sandbox error", err)
	}
}
