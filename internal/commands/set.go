package commands

import (
	"fmt"
	"strings"

	"pkdindustries/soulshack/internal/irc"
)

// SetCommand handles the /set command for configuration changes
type SetCommand struct{}

func (c *SetCommand) Name() string    { return "/set" }
func (c *SetCommand) AdminOnly() bool { return true }

func (c *SetCommand) Execute(ctx irc.ChatContextInterface) {
	keys := getConfigKeys()
	if len(ctx.GetArgs()) < 3 {
		ctx.Reply(fmt.Sprintf("Usage: /set <key> <value>. Available keys: %s", strings.Join(keys, ", ")))
		return
	}

	param, v := ctx.GetArgs()[1], ctx.GetArgs()[2:]
	value := strings.Join(v, " ")
	cfg := ctx.GetConfig()

	ctx.GetLogger().Debugw("config_change_requested", "param", param, "value", value)

	// Handle standard config fields
	field, ok := configFields[param]
	if !ok {
		ctx.Reply(fmt.Sprintf("Unknown key. Available keys: %s", strings.Join(keys, ", ")))
		return
	}

	if err := field.setter(cfg, value); err != nil {
		ctx.Reply(err.Error())
		return
	}

	// If an API key or URL was set, update the LLM client
	if strings.Contains(param, "key") || strings.Contains(param, "url") || strings.Contains(param, "model") {
		if err := ctx.GetSystem().UpdateLLM(*cfg.API); err != nil {
			ctx.GetLogger().Errorw("llm_update_failed", "error", err)
			ctx.Reply("Configuration saved, but failed to update LLM client")
		}
	}

	ctx.Reply(fmt.Sprintf("%s set to: %s", param, field.getter(cfg)))
	ctx.GetSession().Clear()
}
