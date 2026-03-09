package behaviors

import (
	"github.com/lrstanley/girc"

	"pkdindustries/soulshack/internal/commands"
	"pkdindustries/soulshack/internal/core"
	"pkdindustries/soulshack/internal/irc"
)

// NonAddressedBehavior handles all messages when addressed mode is disabled
type NonAddressedBehavior struct {
	CmdRegistry *commands.Registry
}

func (b *NonAddressedBehavior) Name() string {
	return "nonaddressed"
}

func (b *NonAddressedBehavior) Events() []string {
	return []string{girc.PRIVMSG}
}

func (b *NonAddressedBehavior) Check(ctx irc.ChatContextInterface, event *girc.Event) bool {
	cfg := ctx.GetConfig()
	return !cfg.Bot.Addressed && !ctx.IsAddressed() && !ctx.IsPrivate() && len(ctx.GetArgs()) > 0
}

func (b *NonAddressedBehavior) Execute(ctx irc.ChatContextInterface, event *girc.Event) {
	core.WithRequestLock(ctx, ctx.GetLockKey(), "nonaddressed", func() {
		b.CmdRegistry.Dispatch(ctx)
	}, func() {
		ctx.Reply("Request timed out waiting for previous operation to complete")
	})
}
