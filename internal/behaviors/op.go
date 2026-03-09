package behaviors

import (
	"fmt"
	"strings"

	"github.com/lrstanley/girc"

	"pkdindustries/soulshack/internal/core"
	"pkdindustries/soulshack/internal/irc"
	"pkdindustries/soulshack/internal/llm"
)

const opWatcherPrefixes = "(qaohv)~&%@+"

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

	_, ok := opActionForNick(event, b.BotNick)
	return ok
}

func (b *OpBehavior) Execute(ctx irc.ChatContextInterface, event *girc.Event) {
	core.WithRequestLock(ctx, ctx.GetLockKey(), "op", func() {
		cfg := ctx.GetConfig()
		changedBy := event.Source.Name

		action, ok := opActionForNick(event, b.BotNick)
		if !ok {
			return
		}

		prompt := fmt.Sprintf(cfg.Bot.OpWatcherTemplate, action, changedBy)
		outch, err := llm.Complete(ctx, prompt)

		if err != nil {
			ctx.GetLogger().Error("op_behavior_error", "error", err)
			ctx.Reply(err.Error())
			return
		}

		for res := range outch {
			ctx.Reply(res)
		}
	}, nil)
}

func opActionForNick(event *girc.Event, nick string) (string, bool) {
	if len(event.Params) < 3 {
		return "", false
	}

	channelModes := girc.NewCModes(girc.ModeDefaults, opWatcherPrefixes)
	modes := channelModes.Parse(event.Params[1], event.Params[2:])
	for _, mode := range modes {
		switch mode.Short() {
		case "+o":
			if modeTarget(mode) == nick {
				return "opped", true
			}
		case "-o":
			if modeTarget(mode) == nick {
				return "deopped", true
			}
		}
	}

	return "", false
}

func modeTarget(mode girc.CMode) string {
	parts := strings.SplitN(mode.String(), " ", 2)
	if len(parts) != 2 {
		return ""
	}
	return parts[1]
}
