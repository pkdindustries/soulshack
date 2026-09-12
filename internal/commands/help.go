package commands

import (
	"strings"

	"pkdindustries/soulshack/internal/core"
)

// HelpCommand handles the /help command
type HelpCommand struct {
	registry *Registry
}

// NewHelpCommand creates a help command that can list registered commands
func NewHelpCommand(registry *Registry) *HelpCommand {
	return &HelpCommand{registry: registry}
}

func (c *HelpCommand) Name() string    { return "/help" }
func (c *HelpCommand) AdminOnly() bool { return false }

func (c *HelpCommand) Execute(turn *core.Turn) {
	cmds := c.registry.All()
	var names []string
	isAdmin := turn.IsAdmin()

	for _, cmd := range cmds {
		if cmd.AdminOnly() && !isAdmin {
			continue
		}
		if name := cmd.Name(); name != "" {
			names = append(names, name)
		}
	}

	turn.Reply("Supported commands: " + strings.Join(names, ", "))
}
