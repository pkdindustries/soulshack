package bot

import (
	"sync/atomic"

	"github.com/alexschlessinger/pollytool/sessions"
	"github.com/alexschlessinger/pollytool/tools"
	"go.uber.org/zap"

	"pkdindustries/soulshack/internal/config"
	"pkdindustries/soulshack/internal/core"
	"pkdindustries/soulshack/internal/irc"
	"pkdindustries/soulshack/internal/llm"
)

type SystemImpl struct {
	Store sessions.SessionStore
	Tools *tools.ToolRegistry
	llm   atomic.Value // stores core.LLM
}

func (s *SystemImpl) GetToolRegistry() *tools.ToolRegistry {
	return s.Tools
}

func (s *SystemImpl) GetSessionStore() sessions.SessionStore {
	return s.Store
}

func (s *SystemImpl) GetLLM() core.LLM {
	return s.llm.Load().(core.LLM)
}

func (s *SystemImpl) UpdateLLM(cfg config.APIConfig) error {
	zap.S().Info("Updating LLM client...")
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
	if len(c.Bot.Tools) > 0 {
		for _, toolSpec := range c.Bot.Tools {
			if _, err := s.Tools.LoadToolAuto(toolSpec); err != nil {
				zap.S().Warnw("Warning loading tool", "tool", toolSpec, "error", err)
				continue
			}
		}
	}
	zap.S().Infow("Loaded tools", "count", len(s.Tools.All()))

	// initialize sessions with pollytool's SyncMapSessionStore
	zap.S().Info("Initialized session store: syncmap")

	s.Store = sessions.NewSyncMapSessionStore(&sessions.Metadata{
		MaxHistoryTokens: c.Session.MaxContext,
		TTL:              c.Session.TTL,
		SystemPrompt:     c.Bot.Prompt,
	})

	// Initialize LLM
	s.UpdateLLM(*c.API)

	return s
}
