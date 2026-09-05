package commands

import (
	"fmt"
	"strings"
	"time"

	"pkdindustries/soulshack/internal/core"
	"pkdindustries/soulshack/internal/irc"

	"github.com/alexschlessinger/pollytool/messages"
)

// StatsCommand handles the /stats command for showing session statistics
type StatsCommand struct{}

func (c *StatsCommand) Name() string    { return "/stats" }
func (c *StatsCommand) AdminOnly() bool { return false }

func (c *StatsCommand) Execute(ctx irc.ChatContextInterface) {
	session := ctx.GetSession()
	history, err := session.GetHistory(ctx)
	if err != nil {
		ctx.GetLogger().Error("stats_failed", "error", err)
		ctx.Reply("Failed to read session stats")
		return
	}
	metadata, err := session.GetMetadata(ctx)
	if err != nil {
		ctx.GetLogger().Error("stats_failed", "error", err)
		ctx.Reply("Failed to read session stats")
		return
	}

	// Calculate token breakdown
	totalInputTokens := 0
	totalOutputTokens := 0

	// Track participants (IRC-specific)
	participants := make(map[string]bool)

	for _, msg := range history {
		// Token counting
		input := msg.GetInputTokens()
		output := msg.GetOutputTokens()

		if input > 0 || output > 0 {
			totalInputTokens += input
			totalOutputTokens += output
		}

		// Track participants from user messages
		if msg.Role == messages.MessageRoleUser {
			// Extract nick from content if present
			// Format: "(nick:username) message"
			if strings.HasPrefix(msg.Content, "(nick:") {
				end := strings.Index(msg.Content, ")")
				if end > 6 {
					parts := strings.SplitN(msg.Content[6:end], ":", 2)
					if len(parts) > 0 {
						nick := parts[0]
						participants[nick] = true
					}
				}
			}
		}
	}

	// Get message counts and tool calls using new interface methods
	messageCounts, err := session.GetMessageCounts(ctx)
	if err != nil {
		ctx.GetLogger().Error("stats_failed", "error", err)
		ctx.Reply("Failed to read session stats")
		return
	}

	// The active lease keeps a session alive; expiry is an idle retention rule.
	ttlStr := "disabled"
	if metadata.TTL > 0 {
		ttlStr = fmt.Sprintf("after %s idle", formatDuration(metadata.TTL))
	}
	contextStr := "no completed request"
	if usage, ok := core.LastContextUsage(history); ok {
		contextStr = fmt.Sprintf("~%d tokens (no configured limit)", usage.EstimatedTokens)
		if usage.Budget > 0 {
			contextStr = fmt.Sprintf("~%d/%d tokens", usage.EstimatedTokens, usage.Budget)
		}
		if usage.OmittedExchanges > 0 {
			contextStr += fmt.Sprintf(" (%d older exchanges omitted)", usage.OmittedExchanges)
		}
	}

	// Build response in simple format
	response := fmt.Sprintf(
		"total token input: %d, "+
			"total token output: %d, "+
			"last completed input: %s, "+
			"stored messages: %d (user: %d, assistant: %d, tool: %d), "+
			"participants: %d, "+
			"idle expiry: %s",
		totalInputTokens,
		totalOutputTokens,
		contextStr,
		len(history),
		messageCounts[string(messages.MessageRoleUser)],
		messageCounts[string(messages.MessageRoleAssistant)],
		messageCounts[string(messages.MessageRoleTool)],
		len(participants),
		ttlStr,
	)

	ctx.Reply(response)
}

// formatDuration formats a duration into a human-readable string
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	} else if d < time.Hour {
		return fmt.Sprintf("%.0fm", d.Minutes())
	} else {
		hours := int(d.Hours())
		minutes := int(d.Minutes()) % 60
		if minutes > 0 {
			return fmt.Sprintf("%dh %dm", hours, minutes)
		}
		return fmt.Sprintf("%dh", hours)
	}
}
