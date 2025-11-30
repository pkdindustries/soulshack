package commands

import (
	"pkdindustries/soulshack/internal/irc"
)

// VersionCommand handles the /version command
type VersionCommand struct {
	Version string
}

func (c *VersionCommand) Name() string     { return "/version" }
func (c *VersionCommand) AdminOnly() bool  { return false }

func (c *VersionCommand) Execute(ctx irc.ChatContextInterface) {
	nick := ctx.GetConfig().Server.Nick
	ctx.Reply(nick + " " + c.Version)
}
