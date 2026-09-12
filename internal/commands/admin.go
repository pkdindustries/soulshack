package commands

import (
	"fmt"
	"slices"
	"strings"

	"pkdindustries/soulshack/internal/config"
	"pkdindustries/soulshack/internal/core"
	"pkdindustries/soulshack/internal/irc"
)

// AdminCommand handles the /admin command for managing bot administrators
type AdminCommand struct{}

func (c *AdminCommand) Name() string    { return "/admins" }
func (c *AdminCommand) AdminOnly() bool { return true }

func (c *AdminCommand) Execute(turn *core.Turn) {
	args := turn.GetArgs()

	// No args or "list" = show current admins
	if len(args) < 2 || args[1] == "list" {
		c.listAdmins(turn)
		return
	}

	subcommand := args[1]
	if len(args) < 3 {
		turn.Reply("Usage: /admins <add|remove> <hostmask>")
		return
	}
	hostmask := strings.Join(args[2:], " ")

	switch subcommand {
	case "add":
		c.addAdmin(turn, hostmask)
	case "remove":
		c.removeAdmin(turn, hostmask)
	default:
		turn.Reply(fmt.Sprintf("Unknown subcommand: %s. Usage: /admins [list|add|remove] <hostmask>", subcommand))
	}

	cfg := turn.GetConfig() // refresh after modification
	turn.GetLogger().Debug("admin_list_updated", "admins", cfg.Bot.Admins)
}

func (c *AdminCommand) listAdmins(turn *core.Turn) {
	cfg := turn.GetConfig()
	if len(cfg.Bot.Admins) == 0 {
		turn.Reply("No admins configured")
		return
	}
	turn.Reply("Admins: " + strings.Join(cfg.Bot.Admins, ", "))
}

func (c *AdminCommand) addAdmin(turn *core.Turn, hostmask string) {
	if hostmask == "" {
		turn.Reply("Usage: /admins add <hostmask>")
		return
	}

	if err := irc.ValidateHostmask(hostmask); err != nil {
		turn.Reply(fmt.Sprintf("Invalid hostmask: %s", err))
		return
	}

	added := false
	if err := turn.GetSystem().UpdateConfig(func(live *config.Configuration) error {
		if slices.Contains(live.Bot.Admins, hostmask) {
			return nil
		}
		live.Bot.Admins = append(live.Bot.Admins, hostmask)
		added = true
		return nil
	}); err != nil {
		turn.GetLogger().Error("admin_update_failed", "error", err)
		turn.Reply("Failed to change admins")
		return
	}
	if !added {
		turn.Reply(fmt.Sprintf("Already an admin: %s", hostmask))
		return
	}
	turn.Reply(fmt.Sprintf("Added admin: %s", hostmask))
}

func (c *AdminCommand) removeAdmin(turn *core.Turn, hostmask string) {
	if hostmask == "" {
		turn.Reply("Usage: /admins remove <hostmask>")
		return
	}

	removed := false
	if err := turn.GetSystem().UpdateConfig(func(live *config.Configuration) error {
		idx := slices.Index(live.Bot.Admins, hostmask)
		if idx == -1 {
			return nil
		}
		live.Bot.Admins = slices.Delete(live.Bot.Admins, idx, idx+1)
		removed = true
		return nil
	}); err != nil {
		turn.GetLogger().Error("admin_update_failed", "error", err)
		turn.Reply("Failed to change admins")
		return
	}
	if !removed {
		turn.Reply(fmt.Sprintf("Not an admin: %s", hostmask))
		return
	}
	turn.Reply(fmt.Sprintf("Removed admin: %s", hostmask))
}
