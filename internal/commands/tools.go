package commands

import (
	"fmt"
	"path"
	"strings"

	"pkdindustries/soulshack/internal/irc"

	"github.com/alexschlessinger/pollytool/tools"
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

	subcommand := args[1]
	rest := ""
	if len(args) > 2 {
		rest = strings.Join(args[2:], " ")
	}

	// Handle non-admin subcommands first
	if subcommand == "list" {
		c.listNamespace(ctx, rest)
		return
	}

	// Other subcommands require admin privileges
	if !ctx.IsAdmin() {
		ctx.Reply("You don't have permission to perform this action.")
		return
	}

	switch subcommand {
	case "add":
		c.addTool(ctx, rest)
	case "remove":
		c.removeTool(ctx, rest)
	default:
		ctx.Reply("Usage: /tools [list|add|remove] <args>")
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

	message := formatToolList(toolNames)
	ctx.Reply(truncateMessage(message, ctx.GetConfig().Session.ChunkMax))
}

func (c *ToolsCommand) listNamespace(ctx irc.ChatContextInterface, namespace string) {
	if namespace == "" {
		c.listTools(ctx)
		return
	}

	registry := ctx.GetSystem().GetToolRegistry()
	allTools := registry.All()

	prefix := namespace + "__"

	var toolNames []string
	for _, tool := range allTools {
		name := tool.GetName()
		if strings.HasPrefix(name, prefix) {
			_, bareName := parseToolName(name)
			toolNames = append(toolNames, namespace+"__"+bareName)
		}
	}

	if len(toolNames) == 0 {
		ctx.Reply(fmt.Sprintf("No tools in namespace: %s", namespace))
		return
	}

	message := strings.Join(toolNames, ", ")
	ctx.Reply(truncateMessage(message, ctx.GetConfig().Session.ChunkMax))
}

// parseToolName extracts namespace and bare name from a tool name
// e.g., "git__status" -> ("git", "status")
// e.g., "irc__op" -> ("irc", "op")
func parseToolName(name string) (namespace, bareName string) {
	if idx := strings.Index(name, "__"); idx != -1 {
		return name[:idx], name[idx+2:]
	}
	return "other", name
}

func (c *ToolsCommand) addTool(ctx irc.ChatContextInterface, toolPath string) {
	if toolPath == "" {
		ctx.Reply("Usage: /tools add <path>")
		return
	}

	registry := ctx.GetSystem().GetToolRegistry()
	result, err := registry.LoadToolAuto(toolPath)
	if err != nil {
		ctx.Reply(fmt.Sprintf("Failed: %v", err))
		return
	}

	ctx.Reply(formatLoadResult(result))
}

// formatLoadResult creates a compact message for all loaded tools
func formatLoadResult(result tools.LoadResult) string {
	if len(result.Servers) == 0 {
		return "No tools loaded"
	}

	groups := make(map[string]int)
	var order []string
	for _, server := range result.Servers {
		groups[server.Name] = len(server.ToolNames)
		order = append(order, server.Name)
	}
	return fmt.Sprintf("Added: %s", formatGroupedSummary(groups, order))
}

func (c *ToolsCommand) removeTool(ctx irc.ChatContextInterface, pattern string) {
	if pattern == "" {
		ctx.Reply("Usage: /tools remove <name or pattern>")
		return
	}

	registry := ctx.GetSystem().GetToolRegistry()

	// Check if this is a namespace removal (plain name or name__*)
	isNamespaceRemoval := false
	namespace := pattern
	if !strings.Contains(pattern, "*") && !strings.Contains(pattern, "__") {
		isNamespaceRemoval = true
	} else if strings.HasSuffix(pattern, "__*") {
		isNamespaceRemoval = true
		namespace = strings.TrimSuffix(pattern, "__*")
	}

	// If no wildcards and no __, treat as namespace prefix
	if !strings.Contains(pattern, "*") && !strings.Contains(pattern, "__") {
		pattern = pattern + "__*"
	}

	// Use wildcard matching
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
			if isNamespaceRemoval {
				ctx.Reply(fmt.Sprintf("Removed: %s", namespace))
			} else {
				ctx.Reply(fmt.Sprintf("Removed: %s", formatToolList(removed)))
			}
		} else {
			ctx.Reply(fmt.Sprintf("No tools matched: %s", pattern))
		}
	} else {
		// Exact match
		if _, exists := registry.Get(pattern); !exists {
			ctx.Reply(fmt.Sprintf("Not found: %s", pattern))
		} else {
			registry.Remove(pattern)
			_, bareName := parseToolName(pattern)
			ctx.Reply(fmt.Sprintf("Removed: %s", bareName))
		}
	}
}

// formatToolList formats a list of tool names in grouped format
func formatToolList(toolNames []string) string {
	groups := make(map[string]int)
	var order []string

	for _, name := range toolNames {
		namespace, _ := parseToolName(name)
		if _, exists := groups[namespace]; !exists {
			order = append(order, namespace)
		}
		groups[namespace]++
	}

	return formatGroupedSummary(groups, order)
}

// formatGroupedSummary formats namespace counts as "ns1 (N tools), ns2 (M tools)"
func formatGroupedSummary(groups map[string]int, order []string) string {
	var parts []string
	for _, ns := range order {
		count := groups[ns]
		if count == 1 {
			parts = append(parts, fmt.Sprintf("%s (1 tool)", ns))
		} else {
			parts = append(parts, fmt.Sprintf("%s (%d tools)", ns, count))
		}
	}
	return strings.Join(parts, ", ")
}

// truncateMessage truncates a message to maxLen with ellipsis
func truncateMessage(message string, maxLen int) string {
	if maxLen <= 0 {
		maxLen = 350
	}
	if len(message) > maxLen {
		return message[:maxLen-3] + "..."
	}
	return message
}
