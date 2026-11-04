package commands

import (
	"fmt"
	"strings"

	"pkdindustries/soulshack/internal/core"
)

// GetCommand handles the /get command for reading configuration
type GetCommand struct{}

func (c *GetCommand) Name() string    { return "/get" }
func (c *GetCommand) AdminOnly() bool { return false }

func (c *GetCommand) Execute(turn *core.Turn) {
	keys := getConfigKeys()
	if len(turn.GetArgs()) < 2 {
		turn.Reply(fmt.Sprintf("Usage: /get <key>. Available keys: %s", strings.Join(keys, ", ")))
		return
	}

	param := turn.GetArgs()[1]
	cfg := turn.GetConfig()

	// Handle special cases first
	switch param {
	case "admins":
		if len(cfg.Bot.Admins) == 0 {
			turn.Reply("empty admin list, all nicks are permitted to use admin commands")
			return
		}
		turn.Reply(fmt.Sprintf("%s: %s", param, strings.Join(cfg.Bot.Admins, ", ")))
		return

	}

	// Handle standard config fields
	field, ok := configFields[param]
	if !ok {
		turn.Reply(fmt.Sprintf("Unknown key %s. Available keys: %s", param, strings.Join(keys, ", ")))
		return
	}

	turn.Reply(fmt.Sprintf("%s: %s", param, field.getter(cfg)))
}
