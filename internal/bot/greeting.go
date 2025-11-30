package bot

import (
	"pkdindustries/soulshack/internal/irc"
	"pkdindustries/soulshack/internal/llm"
)

// Greeting sends a greeting message when the bot joins a channel
func Greeting(ctx irc.ChatContextInterface) {
	config := ctx.GetConfig()
	outch, err := llm.CompleteWithText(ctx, config.Bot.Greeting)

	if err != nil {
		ctx.Reply(err.Error())
		return
	}

	for res := range outch {
		ctx.Reply(res)
	}
}
