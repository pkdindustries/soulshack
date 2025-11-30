package bot

import (
	"regexp"

	"pkdindustries/soulshack/internal/irc"
)

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
