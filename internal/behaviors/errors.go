package behaviors

import (
	"fmt"

	"github.com/lrstanley/girc"
	"go.uber.org/zap"

	"pkdindustries/soulshack/internal/irc"
)

// NickErrorBehavior handles nick already in use errors
type NickErrorBehavior struct{}

func (b *NickErrorBehavior) Name() string {
	return "nick_error"
}

func (b *NickErrorBehavior) Events() []string {
	return []string{girc.ERR_NICKNAMEINUSE}
}

func (b *NickErrorBehavior) Check(ctx irc.ChatContextInterface, event *girc.Event) bool {
	return true
}

func (b *NickErrorBehavior) Execute(ctx irc.ChatContextInterface, event *girc.Event) {
	cfg := ctx.GetConfig()
	zap.S().Errorw("nick_in_use", "nick", cfg.Server.Nick)
	ctx.FatalError(fmt.Errorf("nick %q is already in use", cfg.Server.Nick))
}

// ChannelErrorBehavior handles channel join failure errors
type ChannelErrorBehavior struct{}

var channelErrorReasons = map[string]string{
	girc.ERR_NOSUCHCHANNEL:  "channel does not exist",
	girc.ERR_CHANNELISFULL:  "channel is full",
	girc.ERR_INVITEONLYCHAN: "channel is invite-only",
	girc.ERR_BANNEDFROMCHAN: "banned from channel",
	girc.ERR_BADCHANNELKEY:  "bad channel key",
}

func (b *ChannelErrorBehavior) Name() string {
	return "channel_error"
}

func (b *ChannelErrorBehavior) Events() []string {
	return []string{
		girc.ERR_NOSUCHCHANNEL,
		girc.ERR_CHANNELISFULL,
		girc.ERR_INVITEONLYCHAN,
		girc.ERR_BANNEDFROMCHAN,
		girc.ERR_BADCHANNELKEY,
	}
}

func (b *ChannelErrorBehavior) Check(ctx irc.ChatContextInterface, event *girc.Event) bool {
	return true
}

func (b *ChannelErrorBehavior) Execute(ctx irc.ChatContextInterface, event *girc.Event) {
	cfg := ctx.GetConfig()
	channel := cfg.Server.Channel
	if len(event.Params) > 1 {
		channel = event.Params[1]
	}
	reason := channelErrorReasons[event.Command]
	zap.S().Errorw("channel_join_failed", "channel", channel, "reason", reason)
	ctx.FatalError(fmt.Errorf("cannot join %s: %s", channel, reason))
}
