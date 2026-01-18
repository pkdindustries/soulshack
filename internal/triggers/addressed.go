package triggers

import (
	"github.com/lrstanley/girc"

	"pkdindustries/soulshack/internal/commands"
	"pkdindustries/soulshack/internal/core"
	"pkdindustries/soulshack/internal/irc"
)

// AddressedTrigger handles messages addressed to the bot
type AddressedTrigger struct {
	CmdRegistry *commands.Registry
}

func (t *AddressedTrigger) Name() string {
	return "addressed"
}

func (t *AddressedTrigger) Events() []string {
	return []string{girc.PRIVMSG}
}

func (t *AddressedTrigger) Check(ctx irc.ChatContextInterface, event *girc.Event) bool {
	return (ctx.IsAddressed() || ctx.IsPrivate()) && len(ctx.GetArgs()) > 0
}

func (t *AddressedTrigger) Execute(ctx irc.ChatContextInterface, event *girc.Event) {
	core.WithRequestLock(ctx, ctx.GetLockKey(), "addressed", func() {
		t.CmdRegistry.Dispatch(ctx)
	}, func() {
		ctx.Reply("Request timed out waiting for previous operation to complete")
	})
}
