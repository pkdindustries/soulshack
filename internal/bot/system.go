package bot

import (
	"fmt"
	"log/slog"
	"sync/atomic"

	"github.com/alexschlessinger/pollytool/tools"
	"github.com/alexschlessinger/pollytool/tools/sandbox"

	"pkdindustries/soulshack/internal/config"
	"pkdindustries/soulshack/internal/core"
	"pkdindustries/soulshack/internal/irc"
	"pkdindustries/soulshack/internal/llm"
	"pkdindustries/soulshack/internal/memory"
	"pkdindustries/soulshack/internal/subagents"
)

type SystemImpl struct {
	Memory *memory.Memory
	Tools  *tools.ToolRegistry
	Config *config.Store
	Agents core.Agents
	llm    atomic.Value // stores core.LLM
}

func (s *SystemImpl) GetConfig() *config.Configuration {
	return s.Config.Snapshot()
}

func (s *SystemImpl) UpdateConfig(fn func(*config.Configuration) error) error {
	return s.Config.Update(fn)
}

func (s *SystemImpl) GetToolRegistry() *tools.ToolRegistry {
	return s.Tools
}

func (s *SystemImpl) GetMemory() *memory.Memory {
	return s.Memory
}

func (s *SystemImpl) GetAgents() core.Agents {
	return s.Agents
}

func (s *SystemImpl) GetLLM() core.LLM {
	return s.llm.Load().(core.LLM)
}

func (s *SystemImpl) UpdateLLM(cfg config.APIConfig) error {
	slog.Info("llm_updating")
	s.llm.Store(llm.NewPollyLLM(cfg))
	return nil
}

// newSandbox is a hook for tests to simulate sandbox setup failure.
var newSandbox = sandbox.New

func NewSystem(c *config.Configuration) (core.System, error) {
	s := &SystemImpl{Config: config.NewStore(c)}

	// Optionally enable platform sandboxing for shell/bash/MCP tools.
	regOpts := []tools.RegistryOption{tools.WithNativeTools()}
	if c.Bot.Sandbox {
		baseCfg := sandbox.DefaultConfig()
		if _, err := newSandbox(baseCfg); err != nil {
			return nil, fmt.Errorf("initialize sandbox: %w", err)
		}
		regOpts = append(regOpts, tools.WithSandboxFactory(newSandbox, baseCfg))
		slog.Info("sandbox_enabled")
	} else {
		regOpts = append(regOpts, tools.WithUnsafeNoSandbox())
	}
	s.Tools = tools.NewToolRegistry([]tools.Tool{}, regOpts...)
	// WithNativeTools auto-registers view_image; remove it so all native tools are opt-in via --tool.
	s.Tools.Remove("view_image")

	// Register native IRC tools with polly's registry (opt-in via --tool)
	irc.RegisterIRCTools(s.Tools)

	// Fixed at startup: a tool can't be withdrawn from a running turn.
	if c.Bot.Subagents {
		tracker := subagents.NewTracker()
		s.Agents = tracker
		subagents.Register(s.Tools, tracker)
		slog.Info("subagents_enabled",
			"model", c.Bot.SubagentModel,
			"timeout", c.Bot.SubagentTimeout,
			"max", c.Bot.SubagentMax,
			"max_per_chat", c.Bot.SubagentMaxPerChat,
		)
	}

	// Load all tools from configuration (polly now handles native, shell, and MCP tools)
	toolErrors := 0
	if len(c.Bot.Tools) > 0 {
		for _, toolSpec := range c.Bot.Tools {
			if _, err := s.Tools.LoadToolAuto(toolSpec); err != nil {
				slog.Warn("tool_load_failed", "tool", toolSpec, "error", err)
				toolErrors++
				continue
			}
		}
	}

	// Fail fast on a maxcontext that can't fit the prompt and tools.
	if err := llm.CheckContextBudget(c, s.Tools); err != nil {
		_ = s.Tools.Close()
		return nil, err
	}

	mem, err := memory.New(memory.Config{
		Budget: c.Session.MaxContext,
		TTL:    c.Session.TTL,
		Path:   c.Session.Path,
	})
	if err != nil {
		_ = s.Tools.Close()
		return nil, err
	}
	s.Memory = mem

	// Initialize LLM
	s.UpdateLLM(*c.API)

	// Log startup summary
	fields := []any{
		"model", c.Model.Model,
		"tools_loaded", len(s.Tools.All()),
		"max_context", c.Session.MaxContext,
		"session_db", c.Session.Path,
		"session_ttl", c.Session.TTL,
	}
	if toolErrors > 0 {
		fields = append(fields, "tool_errors", toolErrors)
	}
	slog.Info("system_initialized", fields...)

	return s, nil
}
