package behaviors

import (
	"fmt"
	"strings"

	"github.com/lrstanley/girc"

	"pkdindustries/soulshack/internal/core"
	"pkdindustries/soulshack/internal/irc"
)

// OpBehavior responds when the bot receives +o or -o (operator status change)
type OpBehavior struct{}

func (b *OpBehavior) Name() string {
	return "op"
}

func (b *OpBehavior) Events() []string {
	return []string{girc.MODE}
}

func (b *OpBehavior) Check(ctx irc.ChatContextInterface, event *girc.Event) bool {
	if !ctx.GetConfig().Bot.OpWatcher || len(event.Params) < 3 || !girc.IsValidChannel(event.Params[0]) {
		return false
	}

	// Parameter modes and membership prefixes vary by server. Their argument
	// rules determine which nickname belongs to each +o or -o in a MODE line.
	channelModes := girc.NewCModes(
		serverOption(ctx, "CHANMODES", girc.ModeDefaults),
		serverOption(ctx, "PREFIX", girc.DefaultPrefixes),
	)
	caseMapping := serverOption(ctx, "CASEMAPPING", "rfc1459")
	nick := irc.FoldNick(ctx.GetBotNick(), caseMapping)
	for _, mode := range channelModes.Parse(event.Params[1], event.Params[2:]) {
		if mode.Short() != "+o" && mode.Short() != "-o" {
			continue
		}
		_, target, ok := strings.Cut(mode.String(), " ")
		if ok && irc.FoldNick(target, caseMapping) == nick {
			return true
		}
	}
	return false
}

func (b *OpBehavior) Execute(ctx irc.ChatContextInterface, event *girc.Event) {
	core.WithConversation(ctx, "op", func(turn *core.Turn) {
		complete(turn, fmt.Sprintf("(nick:%s) %s %s", turn.GetSource(), event.Command, strings.Join(event.Params, " ")), false)
	}, nil)
}

func serverOption(ctx irc.ChatContextInterface, key, fallback string) string {
	if value, ok := ctx.GetServerOption(key); ok {
		return value
	}
	return fallback
}
