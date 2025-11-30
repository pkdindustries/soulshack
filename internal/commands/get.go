package commands

import (
	"fmt"
	"strings"

	"pkdindustries/soulshack/internal/irc"
)

// GetCommand handles the /get command for reading configuration
type GetCommand struct{}

func (c *GetCommand) Name() string    { return "/get" }
func (c *GetCommand) AdminOnly() bool { return false }

func (c *GetCommand) Execute(ctx irc.ChatContextInterface) {
	keys := getConfigKeys()
	if len(ctx.GetArgs()) < 2 {
		ctx.Reply(fmt.Sprintf("Usage: /get <key>. Available keys: %s", strings.Join(keys, ", ")))
		return
	}

	param := ctx.GetArgs()[1]
	cfg := ctx.GetConfig()

	// Handle special cases first
	switch param {
	case "admins":
		if len(cfg.Bot.Admins) == 0 {
			ctx.Reply("empty admin list, all nicks are permitted to use admin commands")
			return
		}
		ctx.Reply(fmt.Sprintf("%s: %s", param, strings.Join(cfg.Bot.Admins, ", ")))
		return

	}

	// Handle standard config fields
	field, ok := configFields[param]
	if !ok {
		ctx.Reply(fmt.Sprintf("Unknown key %s. Available keys: %s", param, strings.Join(keys, ", ")))
		return
	}

	ctx.Reply(fmt.Sprintf("%s: %s", param, field.getter(cfg)))
}
