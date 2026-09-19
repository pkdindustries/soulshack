package bot_test

import (
	"testing"

	"github.com/alexschlessinger/pollytool/subagent"

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
			t.Cleanup(func() { sys.GetMemory().Close() })
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

// Offering the spawning tool is what enables delegation, and it happens once,
// at startup.
func TestNewSystemOffersSpawningOnlyWhenEnabled(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		name := "disabled"
		if enabled {
			name = "enabled"
		}
		t.Run(name, func(t *testing.T) {
			cfg := mocktest.DefaultTestConfig()
			cfg.Bot.Subagents = enabled

			sys, err := bot.NewSystem(cfg)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { sys.GetMemory().Close() })

			offered := false
			for _, tool := range sys.GetToolRegistry().All() {
				if tool.GetName() == subagent.ToolName {
					offered = true
				}
			}
			if offered != enabled {
				t.Fatalf("subagents %v but the spawning tool offered = %v", enabled, offered)
			}
			if agents := sys.GetAgents(); (agents != nil) != enabled {
				t.Fatalf("subagents %v but the agent tracker = %v", enabled, agents)
			}
		})
	}
}
