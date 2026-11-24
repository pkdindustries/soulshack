package bot

import (
	"testing"

	mocktest "pkdindustries/soulshack/internal/testing"
)

func TestCommandRegistryRegistersAgentsOnlyWhenEnabled(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		cfg := mocktest.DefaultTestConfig()
		cfg.Bot.Subagents = enabled
		sys, err := NewSystem(cfg)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { sys.GetMemory().Close() })

		reg := newCommandRegistry(sys)
		if _, ok := reg.Get("/clear"); !ok {
			t.Fatal("/clear was not registered")
		}
		_, ok := reg.Get("/agents")
		if ok != enabled {
			t.Fatalf("subagents=%v: /agents registered = %v, want %v", enabled, ok, enabled)
		}
	}
}
