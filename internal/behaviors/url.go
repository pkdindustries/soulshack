package behaviors

import (
	"fmt"
	"regexp"

	"github.com/lrstanley/girc"

	"pkdindustries/soulshack/internal/core"
	"pkdindustries/soulshack/internal/irc"
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
	b.execute(ctx, event)
}

// execute runs the observation in the background, outliving the event; the
// returned channel closes when it finishes.
func (b *URLBehavior) execute(ctx irc.ChatContextInterface, event *girc.Event) <-chan struct{} {
	cfg := ctx.GetConfig()
	silent := cfg.Bot.URLWatcherSilent
	withConversation := core.WithConversation
	if silent {
		withConversation = core.WithDetachedConversation
	}

	// The handler owns event; read it now.
	prompt := fmt.Sprintf("(nick:%s) %s", ctx.GetSource(), event.Last())

	background, cancel := ctx.Background(cfg.API.Timeout)
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer cancel()
		withConversation(background, "url", func(turn *core.Turn) {
			observe(turn, prompt, silent)
		}, nil)
	}()
	return done
}
