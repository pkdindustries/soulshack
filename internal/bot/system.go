package bot

import (
	"github.com/alexschlessinger/pollytool/sessions"
	"github.com/alexschlessinger/pollytool/tools"
	"go.uber.org/zap"

	"pkdindustries/soulshack/internal/config"
	"pkdindustries/soulshack/internal/core"
	"pkdindustries/soulshack/internal/irc"
)

func NewSystem(c *config.Configuration) core.System {
	s := core.SystemImpl{}
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
		MaxHistory:   c.Session.MaxHistory,
		TTL:          c.Session.TTL,
		SystemPrompt: c.Bot.Prompt,
	})

	// Initialize history store
	history, err := core.NewFileHistory(".history")
	if err != nil {
		zap.S().Warnw("Failed to initialize history store", "error", err)
	} else {
		s.History = history
	}

	return &s
}
