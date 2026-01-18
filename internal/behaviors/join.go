package behaviors

import (
	"github.com/lrstanley/girc"

	"pkdindustries/soulshack/internal/core"
	"pkdindustries/soulshack/internal/irc"
	"pkdindustries/soulshack/internal/llm"
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
	core.WithRequestLock(ctx, ctx.GetLockKey(), "join", func() {
		cfg := ctx.GetConfig()
		outch, err := llm.Complete(ctx, cfg.Bot.Greeting)
		if err != nil {
			ctx.GetLogger().Errorw("join_behavior_error", "error", err)
			ctx.Reply(err.Error())
			return
		}

		for res := range outch {
			ctx.Reply(res)
		}
	}, nil)
}
