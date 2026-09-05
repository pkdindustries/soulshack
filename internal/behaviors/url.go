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
	silent := ctx.GetConfig().Bot.URLWatcherSilent
	withConversation := core.WithConversation
	if silent {
		withConversation = core.WithDetachedConversation
	}
	withConversation(ctx, "url", func(ctx irc.ChatContextInterface) {
		prompt := fmt.Sprintf("(nick:%s) %s", ctx.GetSource(), event.Last())
		outch, err := llm.Complete(ctx, prompt)
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
