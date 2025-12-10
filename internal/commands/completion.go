package commands

import (
	"fmt"
	"strings"

	"pkdindustries/soulshack/internal/irc"
	"pkdindustries/soulshack/internal/llm"
)

// CompletionCommand handles the default chat completion
type CompletionCommand struct{}

func (c *CompletionCommand) Name() string     { return "" }
func (c *CompletionCommand) AdminOnly() bool  { return false }

func (c *CompletionCommand) Execute(ctx irc.ChatContextInterface) {
	msg := strings.Join(ctx.GetArgs(), " ")

	// Apply URL watcher template if this was URL-triggered
	if ctx.IsURLTriggered() && ctx.GetConfig().Bot.URLWatcherTemplate != "" {
		msg = fmt.Sprintf(ctx.GetConfig().Bot.URLWatcherTemplate, msg)
	}

	outch, err := llm.Complete(ctx, fmt.Sprintf("(nick:%s) %s", ctx.GetSource(), msg))

	if err != nil {
		ctx.GetLogger().Errorw("completion_error", "error", err)
		ctx.Reply(err.Error())
		return
	}

	for res := range outch {
		ctx.Reply(res)
	}
}
