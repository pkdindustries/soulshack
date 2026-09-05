package bot

import (
	"fmt"
	"log/slog"
	"sync/atomic"

	"github.com/alexschlessinger/pollytool/sessions"
	"github.com/alexschlessinger/pollytool/tools"
	"github.com/alexschlessinger/pollytool/tools/sandbox"

	"pkdindustries/soulshack/internal/config"
	"pkdindustries/soulshack/internal/core"
	"pkdindustries/soulshack/internal/irc"
	"pkdindustries/soulshack/internal/llm"
)

type SystemImpl struct {
	Sessions sessions.SessionStore
	Tools    *tools.ToolRegistry
	llm      atomic.Value // stores core.LLM
}

func (s *SystemImpl) GetToolRegistry() *tools.ToolRegistry {
	return s.Tools
}

func (s *SystemImpl) GetSessions() sessions.SessionStore {
	return s.Sessions
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
	s := &SystemImpl{}

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

	// initialize sessions with pollytool's in-memory SQLite store
	store, err := sessions.OpenStore(sessions.StoreConfig{
		Mode: sessions.ModeMemory,
		DefaultMetadata: &sessions.Metadata{
			MaxHistoryTokens: c.Session.MaxContext,
			TTL:              c.Session.TTL,
			SystemPrompt:     c.Bot.Prompt,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open session store: %w", err)
	}
	s.Sessions = store

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
