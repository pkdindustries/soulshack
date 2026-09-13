package commands

import (
	"pkdindustries/soulshack/internal/core"
)

// VersionCommand handles the /version command
type VersionCommand struct {
	Version string
}

func (c *VersionCommand) Name() string    { return "/version" }
func (c *VersionCommand) AdminOnly() bool { return false }

func (c *VersionCommand) Execute(turn *core.Turn) {
	turn.Reply(c.Version)
}
