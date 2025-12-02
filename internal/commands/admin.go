package commands

import (
	"fmt"
	"slices"
	"strings"

	"pkdindustries/soulshack/internal/irc"
)

// AdminCommand handles the /admin command for managing bot administrators
type AdminCommand struct{}

func (c *AdminCommand) Name() string    { return "/admins" }
func (c *AdminCommand) AdminOnly() bool { return true }

func (c *AdminCommand) Execute(ctx irc.ChatContextInterface) {
	args := ctx.GetArgs()

	// No args or "list" = show current admins
	if len(args) < 2 || args[1] == "list" {
		c.listAdmins(ctx)
		return
	}

	subcommand := args[1]
	if len(args) < 3 {
		ctx.Reply("Usage: /admins <add|remove> <hostmask>")
		return
	}
	hostmask := strings.Join(args[2:], " ")

	switch subcommand {
	case "add":
		c.addAdmin(ctx, hostmask)
	case "remove":
		c.removeAdmin(ctx, hostmask)
	default:
		ctx.Reply(fmt.Sprintf("Unknown subcommand: %s. Usage: /admins [list|add|remove] <hostmask>", subcommand))
	}

	cfg := ctx.GetConfig() // refresh after modification
	ctx.GetLogger().With("admins", cfg.Bot.Admins).Debug("Admin list updated")
}

func (c *AdminCommand) listAdmins(ctx irc.ChatContextInterface) {
	cfg := ctx.GetConfig()
	if len(cfg.Bot.Admins) == 0 {
		ctx.Reply("No admins configured")
		return
	}
	ctx.Reply("Admins: " + strings.Join(cfg.Bot.Admins, ", "))
}

func (c *AdminCommand) addAdmin(ctx irc.ChatContextInterface, hostmask string) {
	if hostmask == "" {
		ctx.Reply("Usage: /admins add <hostmask>")
		return
	}

	cfg := ctx.GetConfig()

	// Check if already exists
	if slices.Contains(cfg.Bot.Admins, hostmask) {
		ctx.Reply(fmt.Sprintf("Already an admin: %s", hostmask))
		return
	}

	cfg.Bot.Admins = append(cfg.Bot.Admins, hostmask)
	ctx.Reply(fmt.Sprintf("Added admin: %s", hostmask))
	ctx.GetSession().Clear()
}

func (c *AdminCommand) removeAdmin(ctx irc.ChatContextInterface, hostmask string) {
	if hostmask == "" {
		ctx.Reply("Usage: /admins remove <hostmask>")
		return
	}

	cfg := ctx.GetConfig()

	// Find and remove
	idx := slices.Index(cfg.Bot.Admins, hostmask)
	if idx == -1 {
		ctx.Reply(fmt.Sprintf("Not an admin: %s", hostmask))
		return
	}

	cfg.Bot.Admins = slices.Delete(cfg.Bot.Admins, idx, idx+1)
	ctx.Reply(fmt.Sprintf("Removed admin: %s", hostmask))
	ctx.GetSession().Clear()
}
