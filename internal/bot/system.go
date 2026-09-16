package bot

import (
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

func NewSystem(c *config.Configuration) (core.System, error) {
	s := &SystemImpl{Config: config.NewStore(c)}

	// Optionally enable platform sandboxing for shell/bash/MCP tools.
	var regOpts []tools.RegistryOption
	if c.Bot.Sandbox {
		baseCfg := sandbox.DefaultConfig()
		if _, err := sandbox.New(baseCfg); err != nil {
			slog.Warn("sandbox_unavailable", "error", err)
			regOpts = append(regOpts, tools.WithUnsafeNoSandbox())
		} else {
			regOpts = append(regOpts, tools.WithSandboxFactory(sandbox.New, baseCfg))
			slog.Info("sandbox_enabled")
		}
	} else {
		regOpts = append(regOpts, tools.WithUnsafeNoSandbox())
	}
	s.Tools = tools.NewToolRegistry([]tools.Tool{}, regOpts...)

	// Register native IRC tools with polly's registry
	irc.RegisterIRCTools(s.Tools)

	// Whether the model can delegate is fixed here, at startup: offering the
	// tool is what enables it, and a tool cannot be taken back from a turn
	// that is already using it.
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

	// Conversations live in this process only: an in-memory transcript with
	// an idle expiry.
	s.Memory = memory.New(memory.Config{
		Budget: c.Session.MaxContext,
		TTL:    c.Session.TTL,
	})

	// Initialize LLM
	s.UpdateLLM(*c.API)

	// Log startup summary
	fields := []any{
		"model", c.Model.Model,
		"tools_loaded", len(s.Tools.All()),
		"max_context", c.Session.MaxContext,
	}
	if toolErrors > 0 {
		fields = append(fields, "tool_errors", toolErrors)
	}
	slog.Info("system_initialized", fields...)

	return s, nil
}
