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

// execute runs the observation in the background and returns at once. Nobody
// asked for this turn, so nobody should be kept waiting on it: the event
// handler returns while the link is still being read, and the observation gets
// a lifetime of its own rather than the event's. The channel returned is
// closed when the observation has finished, which is what a test waits on.
func (b *URLBehavior) execute(ctx irc.ChatContextInterface, event *girc.Event) <-chan struct{} {
	cfg := ctx.GetConfig()
	silent := cfg.Bot.URLWatcherSilent
	withConversation := core.WithConversation
	if silent {
		withConversation = core.WithDetachedConversation
	}

	// Read the event before the handler that owns it returns.
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
