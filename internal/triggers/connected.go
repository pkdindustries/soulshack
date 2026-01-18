package triggers

import (
	"github.com/lrstanley/girc"
	"go.uber.org/zap"

	"pkdindustries/soulshack/internal/irc"
)

// ConnectedTrigger joins the configured channel when the bot connects
type ConnectedTrigger struct{}

func (t *ConnectedTrigger) Name() string {
	return "connected"
}

func (t *ConnectedTrigger) Events() []string {
	return []string{girc.CONNECTED}
}

func (t *ConnectedTrigger) Check(ctx irc.ChatContextInterface, event *girc.Event) bool {
	return true
}

func (t *ConnectedTrigger) Execute(ctx irc.ChatContextInterface, event *girc.Event) {
	cfg := ctx.GetConfig()
	zap.S().Infow("channel_joining", "channel", cfg.Server.Channel)
	if cfg.Server.ChannelKey != "" {
		ctx.JoinWithKey(cfg.Server.Channel, cfg.Server.ChannelKey)
	} else {
		ctx.Join(cfg.Server.Channel)
	}
}
