package commands

import (
	"fmt"
	"path"
	"strings"

	"pkdindustries/soulshack/internal/irc"
)

// ToolsCommand handles the /tools command for managing tools
type ToolsCommand struct{}

func (c *ToolsCommand) Name() string    { return "/tools" }
func (c *ToolsCommand) AdminOnly() bool { return false } // We handle permissions internally

func (c *ToolsCommand) Execute(ctx irc.ChatContextInterface) {
	args := ctx.GetArgs()

	// If no arguments, list tools (equivalent to old /get tools)
	if len(args) < 2 {
		c.listTools(ctx)
		return
	}

	// Subcommands require admin privileges
	if !ctx.IsAdmin() {
		ctx.Reply("You don't have permission to perform this action.")
		return
	}

	subcommand := args[1]
	rest := ""
	if len(args) > 2 {
		rest = strings.Join(args[2:], " ")
	}

	switch subcommand {
	case "add":
		c.addTool(ctx, rest)
	case "remove":
		c.removeTool(ctx, rest)
	default:
		ctx.Reply("Usage: /tools [add|remove] <args>")
	}
}

func (c *ToolsCommand) listTools(ctx irc.ChatContextInterface) {
	registry := ctx.GetSystem().GetToolRegistry()
	allTools := registry.All()

	if len(allTools) == 0 {
		ctx.Reply("No tools loaded")
		return
	}

	var toolNames []string
	for _, tool := range allTools {
		toolNames = append(toolNames, tool.GetName())
	}

	message := "Tools: " + strings.Join(toolNames, ", ")
	maxLen := ctx.GetConfig().Session.ChunkMax
	if maxLen <= 0 {
		maxLen = 350
	}
	if len(message) > maxLen {
		message = message[:maxLen-3] + "..."
	}
	ctx.Reply(message)
}

func (c *ToolsCommand) addTool(ctx irc.ChatContextInterface, toolPath string) {
	if toolPath == "" {
		ctx.Reply("Usage: /tools add <path>")
		return
	}

	registry := ctx.GetSystem().GetToolRegistry()
	_, err := registry.LoadToolAuto(toolPath)
	if err != nil {
		ctx.Reply(fmt.Sprintf("Failed: %v", err))
	} else {
		ctx.Reply(fmt.Sprintf("Added tool: %s", toolPath))
	}
}

func (c *ToolsCommand) removeTool(ctx irc.ChatContextInterface, pattern string) {
	if pattern == "" {
		ctx.Reply("Usage: /tools remove <name or pattern>")
		return
	}

	registry := ctx.GetSystem().GetToolRegistry()

	if strings.Contains(pattern, "*") {
		var removed []string
		for _, tool := range registry.All() {
			name := tool.GetName()
			matched, _ := path.Match(pattern, name)
			if matched {
				registry.Remove(name)
				removed = append(removed, name)
			}
		}

		if len(removed) > 0 {
			ctx.Reply(fmt.Sprintf("Removed %d tools: %s", len(removed), strings.Join(removed, ", ")))
		} else {
			ctx.Reply(fmt.Sprintf("No tools matched pattern: %s", pattern))
		}
	} else {
		if _, exists := registry.Get(pattern); !exists {
			ctx.Reply(fmt.Sprintf("Not found: %s", pattern))
		} else {
			registry.Remove(pattern)
			ctx.Reply(fmt.Sprintf("Removed: %s", pattern))
		}
	}
}
