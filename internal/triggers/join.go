package triggers

import (
	"github.com/lrstanley/girc"

	"pkdindustries/soulshack/internal/core"
	"pkdindustries/soulshack/internal/irc"
	"pkdindustries/soulshack/internal/llm"
)

// JoinTrigger sends a greeting when the bot joins a channel
type JoinTrigger struct {
	BotNick string
}

func (t *JoinTrigger) Name() string {
	return "join"
}

func (t *JoinTrigger) Events() []string {
	return []string{girc.JOIN}
}

func (t *JoinTrigger) Check(ctx irc.ChatContextInterface, event *girc.Event) bool {
	cfg := ctx.GetConfig()
	return event.Source.Name == t.BotNick && cfg.Bot.Greeting != ""
}

func (t *JoinTrigger) Execute(ctx irc.ChatContextInterface, event *girc.Event) {
	core.WithRequestLock(ctx, ctx.GetLockKey(), "join", func() {
		cfg := ctx.GetConfig()
		outch, err := llm.Complete(ctx, cfg.Bot.Greeting)
		if err != nil {
			ctx.GetLogger().Errorw("join_trigger_error", "error", err)
			ctx.Reply(err.Error())
			return
		}

		for res := range outch {
			ctx.Reply(res)
		}
	}, nil)
}
