package commands

import (
	"fmt"
	"regexp"
	"strings"

	"pkdindustries/soulshack/internal/irc"
	"pkdindustries/soulshack/internal/llm"
)

func CompletionResponse(ctx irc.ChatContextInterface) {
	msg := strings.Join(ctx.GetArgs(), " ")

	outch, err := llm.CompleteWithText(ctx, fmt.Sprintf("(nick:%s) %s", ctx.GetSource(), msg))

	if err != nil {
		ctx.GetLogger().Errorw("Completion response error", "error", err)
		ctx.Reply(err.Error())
		return
	}

	for res := range outch {
		ctx.Reply(res)
	}
}

var urlPattern = regexp.MustCompile(`^https?://[^\s]+`)

// CheckURLTrigger checks if the message contains a URL and should trigger a response
func CheckURLTrigger(ctx irc.ChatContextInterface, message string) bool {
	if !ctx.GetConfig().Bot.URLWatcher {
		return false
	}
	if ctx.IsAddressed() {
		return false
	}
	if urlPattern.MatchString(message) {
		ctx.GetLogger().Info("URL detected, triggering response")
		return true
	}
	return false
}
