package behaviors

import (
	"github.com/lrstanley/girc"

	"pkdindustries/soulshack/internal/core"
	"pkdindustries/soulshack/internal/irc"
)

// JoinBehavior sends a greeting when the bot joins a channel
type JoinBehavior struct {
	BotNick string
}

func (b *JoinBehavior) Name() string {
	return "join"
}

func (b *JoinBehavior) Events() []string {
	return []string{girc.JOIN}
}

func (b *JoinBehavior) Check(ctx irc.ChatContextInterface, event *girc.Event) bool {
	cfg := ctx.GetConfig()
	return event.Source.Name == b.BotNick && cfg.Bot.Greeting != ""
}

func (b *JoinBehavior) Execute(ctx irc.ChatContextInterface, event *girc.Event) {
	core.WithConversation(ctx, "join", func(turn *core.Turn) {
		complete(turn, turn.GetConfig().Bot.Greeting, false)
	}, nil)
}
