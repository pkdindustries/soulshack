package bot_test

import (
	"testing"

	"pkdindustries/soulshack/internal/bot"
	mocktest "pkdindustries/soulshack/internal/testing"
)

func TestNewSystemLoadsUnsandboxedTools(t *testing.T) {
	for _, atStartup := range []bool{true, false} {
		name := "runtime"
		if atStartup {
			name = "startup"
		}
		t.Run(name, func(t *testing.T) {
			cfg := mocktest.DefaultTestConfig()
			cfg.Bot.Sandbox = false
			if atStartup {
				cfg.Bot.Tools = []string{"bash"}
			}
			sys, err := bot.NewSystem(cfg)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { sys.GetSessions().Close() })
			registry := sys.GetToolRegistry()
			if !atStartup {
				if _, err := registry.LoadToolAuto("bash"); err != nil {
					t.Fatal(err)
				}
			}
			if _, ok := registry.Get("bash"); !ok {
				t.Fatal("bash was not loaded with sandboxing disabled")
			}
		})
	}
}
