package bot

import (
	"log/slog"
	"sync/atomic"

	"github.com/alexschlessinger/pollytool/sessions"
	"github.com/alexschlessinger/pollytool/skills"
	"github.com/alexschlessinger/pollytool/tools"

	"pkdindustries/soulshack/internal/config"
	"pkdindustries/soulshack/internal/core"
	"pkdindustries/soulshack/internal/irc"
	"pkdindustries/soulshack/internal/llm"
)

type SystemImpl struct {
	Store        sessions.SessionStore
	Tools        *tools.ToolRegistry
	SkillCatalog *skills.Catalog
	llm          atomic.Value // stores core.LLM
}

func (s *SystemImpl) GetToolRegistry() *tools.ToolRegistry {
	return s.Tools
}

func (s *SystemImpl) GetSessionStore() sessions.SessionStore {
	return s.Store
}

func (s *SystemImpl) GetSkillCatalog() *skills.Catalog {
	return s.SkillCatalog
}

func (s *SystemImpl) GetLLM() core.LLM {
	return s.llm.Load().(core.LLM)
}

func (s *SystemImpl) UpdateLLM(cfg config.APIConfig) error {
	slog.Info("llm_updating")
	s.llm.Store(llm.NewPollyLLM(cfg))
	return nil
}

func NewSystem(c *config.Configuration) core.System {
	s := &SystemImpl{}
	// Initialize empty tool registry
	s.Tools = tools.NewToolRegistry([]tools.Tool{})

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

	// Discover and initialize skills
	if !c.Bot.NoSkills {
		dirs := append([]string{}, c.Bot.SkillDirs...)
		var autoActivate []string
		for _, source := range c.Bot.Skills {
			resolved, err := skills.ResolveSkill(source)
			if err != nil {
				slog.Warn("skill_resolve_failed", "source", source, "error", err)
				continue
			}
			dirs = append(dirs, resolved.Dir)
			autoActivate = append(autoActivate, resolved.Name)
		}

		catalog, err := skills.LoadCatalog(dirs)
		if err != nil {
			slog.Warn("skill_load_failed", "error", err)
		}
		if catalog != nil {
			s.SkillCatalog = catalog
			runtime, err := tools.NewSkillRuntime(catalog, s.Tools)
			if err != nil {
				slog.Warn("skill_runtime_failed", "error", err)
			} else {
				for _, name := range autoActivate {
					if _, err := runtime.Activate(name); err != nil {
						slog.Warn("skill_activate_failed", "skill", name, "error", err)
					}
				}
			}
		}
	}

	// initialize sessions with pollytool's SyncMapSessionStore
	s.Store = sessions.NewSyncMapSessionStore(&sessions.Metadata{
		MaxHistoryTokens: c.Session.MaxContext,
		TTL:              c.Session.TTL,
		SystemPrompt:     c.Bot.Prompt,
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
	if s.SkillCatalog != nil {
		fields = append(fields, "skills_loaded", len(s.SkillCatalog.List()))
	}
	slog.Info("system_initialized", fields...)

	return s
}
