package commands

import (
	"fmt"
	"strings"

	"pkdindustries/soulshack/internal/config"
	"pkdindustries/soulshack/internal/core"
	"pkdindustries/soulshack/internal/llm"
)

// SetCommand handles the /set command for configuration changes
type SetCommand struct{}

func (c *SetCommand) Name() string    { return "/set" }
func (c *SetCommand) AdminOnly() bool { return true }

func (c *SetCommand) Execute(turn *core.Turn) {
	keys := getConfigKeys()
	if len(turn.GetArgs()) < 3 {
		turn.Reply(fmt.Sprintf("Usage: /set <key> <value>. Available keys: %s", strings.Join(keys, ", ")))
		return
	}

	param, v := turn.GetArgs()[1], turn.GetArgs()[2:]
	value := strings.Join(v, " ")
	turn.GetLogger().Debug("config_change_requested", "param", param, "value", value)

	// Handle standard config fields
	field, ok := configFields[param]
	if !ok {
		turn.Reply(fmt.Sprintf("Unknown key. Available keys: %s", strings.Join(keys, ", ")))
		return
	}

	// The change lands on the live configuration, and anything that holds a
	// copy of the setting follows in the same step.
	system := turn.GetSystem()
	if err := system.UpdateConfig(func(live *config.Configuration) error {
		// The change is tried on a copy first: a setting that leaves
		// maxcontext too small for the prompt and tools, whichever of them
		// changed, is refused before it lands.
		candidate := live.Clone()
		if err := field.setter(candidate, value); err != nil {
			return err
		}
		if err := llm.CheckContextBudget(candidate, system.GetToolRegistry()); err != nil {
			return err
		}
		*live = *candidate
		if field.apply != nil {
			field.apply(live, system)
		}
		return nil
	}); err != nil {
		turn.Reply(fmt.Sprintf("invalid value for %s: %s", param, err))
		return
	}
	cfg := turn.GetConfig()

	// If an API key or URL was set, update the LLM client
	if strings.Contains(param, "key") || strings.Contains(param, "url") || strings.Contains(param, "model") {
		if err := system.UpdateLLM(*cfg.API); err != nil {
			turn.GetLogger().Error("llm_update_failed", "error", err)
			turn.Reply("Configuration saved, but failed to update LLM client")
		}
	}

	turn.Reply(fmt.Sprintf("%s set to: %s", param, field.getter(cfg)))

	// Any configuration change starts this conversation over: its transcript
	// may describe a bot that no longer exists.
	if err := turn.Conversation.Clear(); err != nil {
		turn.GetLogger().Error("conversation_clear_failed", "error", err)
	}
}
