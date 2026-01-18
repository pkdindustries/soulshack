package triggers

import (
	"fmt"

	"github.com/lrstanley/girc"

	"pkdindustries/soulshack/internal/core"
	"pkdindustries/soulshack/internal/irc"
	"pkdindustries/soulshack/internal/llm"
)

// OpTrigger responds when the bot receives +o (operator status)
type OpTrigger struct {
	BotNick string
}

func (t *OpTrigger) Name() string {
	return "op"
}

func (t *OpTrigger) Events() []string {
	return []string{girc.MODE}
}

func (t *OpTrigger) Check(ctx irc.ChatContextInterface, event *girc.Event) bool {
	cfg := ctx.GetConfig()
	if !cfg.Bot.OpWatcher {
		return false
	}

	// MODE format: [channel, modes, target...]
	if len(event.Params) < 3 {
		return false
	}

	channel := event.Params[0]
	targets := event.Params[2:]

	// Check if bot is in targets and is now op (girc updates state before our handler)
	for _, target := range targets {
		if target == t.BotNick {
			return ctx.IsOp(channel, t.BotNick)
		}
	}
	return false
}

func (t *OpTrigger) Execute(ctx irc.ChatContextInterface, event *girc.Event) {
	core.WithRequestLock(ctx, ctx.GetLockKey(), "op", func() {
		cfg := ctx.GetConfig()
		oppedBy := event.Source.Name

		prompt := fmt.Sprintf(cfg.Bot.OpWatcherTemplate, oppedBy)
		outch, err := llm.Complete(ctx, prompt)

		if err != nil {
			ctx.GetLogger().Errorw("op_trigger_error", "error", err)
			ctx.Reply(err.Error())
			return
		}

		for res := range outch {
			ctx.Reply(res)
		}
	}, nil)
}
