package behaviors

import (
	"github.com/lrstanley/girc"
	"go.uber.org/zap"

	"pkdindustries/soulshack/internal/irc"
)

// ConnectedBehavior joins the configured channel when the bot connects
type ConnectedBehavior struct{}

func (b *ConnectedBehavior) Name() string {
	return "connected"
}

func (b *ConnectedBehavior) Events() []string {
	return []string{girc.CONNECTED}
}

func (b *ConnectedBehavior) Check(ctx irc.ChatContextInterface, event *girc.Event) bool {
	return true
}

func (b *ConnectedBehavior) Execute(ctx irc.ChatContextInterface, event *girc.Event) {
	cfg := ctx.GetConfig()
	zap.S().Infow("channel_joining", "channel", cfg.Server.Channel)
	if cfg.Server.ChannelKey != "" {
		ctx.JoinWithKey(cfg.Server.Channel, cfg.Server.ChannelKey)
	} else {
		ctx.Join(cfg.Server.Channel)
	}
}
