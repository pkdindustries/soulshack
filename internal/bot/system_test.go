package bot_test

import (
	"strings"
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

// A maxcontext turns cannot fit in stops the bot at startup, with the
// minimum in the error, rather than failing turns once it is running.
func TestNewSystemRefusesMaxContextBelowTheMinimum(t *testing.T) {
	cfg := mocktest.DefaultTestConfig()
	cfg.Bot.Sandbox = false
	cfg.Session.MaxContext = 1000
	sys, err := bot.NewSystem(cfg)
	if err == nil {
		sys.GetMemory().Close()
		t.Fatal("started with a maxcontext no turn fits in")
	}
	if !strings.Contains(err.Error(), "maxcontext 1000 is below") {
		t.Fatalf("error = %v", err)
	}
}
