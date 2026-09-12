package commands

import (
	"fmt"
	"strings"
	"time"

	"pkdindustries/soulshack/internal/core"

	"github.com/alexschlessinger/pollytool/messages"
)

// StatsCommand handles the /stats command for showing conversation statistics
type StatsCommand struct{}

func (c *StatsCommand) Name() string    { return "/stats" }
func (c *StatsCommand) AdminOnly() bool { return false }

func (c *StatsCommand) Execute(turn *core.Turn) {
	history := turn.Conversation.Messages()

	// Calculate token breakdown
	totalInputTokens := 0
	totalOutputTokens := 0

	// Track participants (IRC-specific)
	participants := make(map[string]bool)
	counts := make(map[string]int)

	for _, msg := range history {
		counts[string(msg.Role)]++

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

	ttlStr := "disabled"
	if ttl := turn.GetSystem().GetMemory().TTL(); ttl > 0 {
		ttlStr = fmt.Sprintf("after %s idle", formatDuration(ttl))
	}
	contextStr := "no completed request"
	if usage, ok := turn.Conversation.Usage(); ok {
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
		counts[string(messages.MessageRoleUser)],
		counts[string(messages.MessageRoleAssistant)],
		counts[string(messages.MessageRoleTool)],
		len(participants),
		ttlStr,
	)

	turn.Reply(response)
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
