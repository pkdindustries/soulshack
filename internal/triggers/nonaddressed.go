package triggers

import (
	"github.com/lrstanley/girc"

	"pkdindustries/soulshack/internal/commands"
	"pkdindustries/soulshack/internal/irc"
)

// NonAddressedTrigger handles all messages when addressed mode is disabled
type NonAddressedTrigger struct {
	CmdRegistry *commands.Registry
}

func (t *NonAddressedTrigger) Name() string {
	return "nonaddressed"
}

func (t *NonAddressedTrigger) Events() []string {
	return []string{girc.PRIVMSG}
}

func (t *NonAddressedTrigger) Check(ctx irc.ChatContextInterface, event *girc.Event) bool {
	cfg := ctx.GetConfig()
	return !cfg.Bot.Addressed && !ctx.IsAddressed() && !ctx.IsPrivate() && len(ctx.GetArgs()) > 0
}

func (t *NonAddressedTrigger) Execute(ctx irc.ChatContextInterface, event *girc.Event) {
	t.CmdRegistry.Dispatch(ctx)
}
