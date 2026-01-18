package behaviors

import (
	"fmt"

	"github.com/lrstanley/girc"

	"pkdindustries/soulshack/internal/core"
	"pkdindustries/soulshack/internal/irc"
	"pkdindustries/soulshack/internal/llm"
)

// OpBehavior responds when the bot receives +o or -o (operator status change)
type OpBehavior struct {
	BotNick string
}

func (b *OpBehavior) Name() string {
	return "op"
}

func (b *OpBehavior) Events() []string {
	return []string{girc.MODE}
}

func (b *OpBehavior) Check(ctx irc.ChatContextInterface, event *girc.Event) bool {
	cfg := ctx.GetConfig()
	if !cfg.Bot.OpWatcher {
		return false
	}

	// MODE format: [channel, modes, target...]
	if len(event.Params) < 3 {
		return false
	}

	targets := event.Params[2:]

	// Check if bot is in targets
	for _, target := range targets {
		if target == b.BotNick {
			return true
		}
	}
	return false
}

func (b *OpBehavior) Execute(ctx irc.ChatContextInterface, event *girc.Event) {
	core.WithRequestLock(ctx, ctx.GetLockKey(), "op", func() {
		cfg := ctx.GetConfig()
		changedBy := event.Source.Name
		channel := event.Params[0]

		action := "deopped"
		if ctx.IsOp(channel, b.BotNick) {
			action = "opped"
		}

		// Template takes: nick, action (e.g., "%s %s you")
		prompt := fmt.Sprintf(cfg.Bot.OpWatcherTemplate, changedBy, action)
		outch, err := llm.Complete(ctx, prompt)

		if err != nil {
			ctx.GetLogger().Errorw("op_behavior_error", "error", err)
			ctx.Reply(err.Error())
			return
		}

		for res := range outch {
			ctx.Reply(res)
		}
	}, nil)
}
