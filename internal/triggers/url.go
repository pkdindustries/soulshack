package triggers

import (
	"fmt"
	"regexp"

	"github.com/lrstanley/girc"

	"pkdindustries/soulshack/internal/core"
	"pkdindustries/soulshack/internal/irc"
	"pkdindustries/soulshack/internal/llm"
)

var urlPattern = regexp.MustCompile(`^https?://[^\s]+`)

// URLTrigger responds to messages containing URLs
type URLTrigger struct{}

func (t *URLTrigger) Name() string {
	return "url"
}

func (t *URLTrigger) Events() []string {
	return []string{girc.PRIVMSG}
}

func (t *URLTrigger) Check(ctx irc.ChatContextInterface, event *girc.Event) bool {
	cfg := ctx.GetConfig()
	if !cfg.Bot.URLWatcher {
		return false
	}
	if ctx.IsAddressed() {
		return false
	}
	if urlPattern.MatchString(event.Last()) {
		ctx.GetLogger().Infow("url_detected")
		return true
	}
	return false
}

func (t *URLTrigger) Execute(ctx irc.ChatContextInterface, event *girc.Event) {
	core.WithRequestLock(ctx, ctx.GetLockKey(), "url", func() {
		cfg := ctx.GetConfig()
		msg := event.Last()

		if cfg.Bot.URLWatcherTemplate != "" {
			msg = fmt.Sprintf(cfg.Bot.URLWatcherTemplate, msg)
		}

		outch, err := llm.Complete(ctx, fmt.Sprintf("(nick:%s) %s", ctx.GetSource(), msg))
		if err != nil {
			ctx.GetLogger().Errorw("url_trigger_error", "error", err)
			ctx.Reply(err.Error())
			return
		}

		for res := range outch {
			ctx.Reply(res)
		}
	}, nil)
}
