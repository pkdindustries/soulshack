package behaviors

import (
	"fmt"
	"regexp"

	"github.com/lrstanley/girc"

	"pkdindustries/soulshack/internal/core"
	"pkdindustries/soulshack/internal/irc"
	"pkdindustries/soulshack/internal/llm"
)

var urlPattern = regexp.MustCompile(`^https?://[^\s]+`)

// URLBehavior responds to messages containing URLs
type URLBehavior struct{}

func (b *URLBehavior) Name() string {
	return "url"
}

func (b *URLBehavior) Events() []string {
	return []string{girc.PRIVMSG}
}

func (b *URLBehavior) Check(ctx irc.ChatContextInterface, event *girc.Event) bool {
	cfg := ctx.GetConfig()
	if !cfg.Bot.URLWatcher {
		return false
	}
	if ctx.IsAddressed() {
		return false
	}
	if urlPattern.MatchString(event.Last()) {
		ctx.GetLogger().Info("url_detected")
		return true
	}
	return false
}

func (b *URLBehavior) Execute(ctx irc.ChatContextInterface, event *girc.Event) {
	core.WithRequestLock(ctx, ctx.GetLockKey(), "url", func() {
		cfg := ctx.GetConfig()
		prompt := fmt.Sprintf("(nick:%s) %s", ctx.GetSource(), event.Last())

		silent := cfg.Bot.URLWatcherSilent
		execCtx := irc.ChatContextInterface(ctx)
		if silent {
			dctx, cleanup, err := newDetachedContext(ctx)
			if err != nil {
				ctx.GetLogger().Error("url_behavior_error", "error", err)
				return
			}
			defer cleanup()
			execCtx = dctx
		}

		outch, err := llm.Complete(execCtx, prompt)
		if err != nil {
			ctx.GetLogger().Error("url_behavior_error", "error", err)
			if !silent {
				ctx.Reply(err.Error())
			}
			return
		}

		for res := range outch {
			if !silent {
				ctx.Reply(res)
			}
		}
	}, nil)
}
