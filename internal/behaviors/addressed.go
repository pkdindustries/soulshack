package behaviors

import (
	"github.com/lrstanley/girc"

	"pkdindustries/soulshack/internal/commands"
	"pkdindustries/soulshack/internal/core"
	"pkdindustries/soulshack/internal/irc"
)

// AddressedBehavior handles messages addressed to the bot
type AddressedBehavior struct {
	CmdRegistry *commands.Registry
}

func (b *AddressedBehavior) Name() string {
	return "addressed"
}

func (b *AddressedBehavior) Events() []string {
	return []string{girc.PRIVMSG}
}

func (b *AddressedBehavior) Check(ctx irc.ChatContextInterface, event *girc.Event) bool {
	return (ctx.IsAddressed() || ctx.IsPrivate()) && len(ctx.GetArgs()) > 0
}

func (b *AddressedBehavior) Execute(ctx irc.ChatContextInterface, event *girc.Event) {
	core.WithRequestLock(ctx, ctx.GetLockKey(), "addressed", func() {
		b.CmdRegistry.Dispatch(ctx)
	}, func() {
		ctx.Reply("Request timed out waiting for previous operation to complete")
	})
}
