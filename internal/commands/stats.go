package commands

import (
	"fmt"

	"pkdindustries/soulshack/internal/irc"

	"github.com/alexschlessinger/pollytool/sessions"
)

// StatsCommand handles the /stats command for showing session statistics
type StatsCommand struct{}

func (c *StatsCommand) Name() string    { return "/stats" }
func (c *StatsCommand) AdminOnly() bool { return false }

func (c *StatsCommand) Execute(ctx irc.ChatContextInterface) {
	session := ctx.GetSession()
	history := session.GetHistory()
	metadata := session.GetMetadata()

	// Get capacity percentage using the new interface method
	percentage := session.GetCapacityPercentage()

	// Still calculate breakdown for detailed stats
	totalInputTokens := 0
	totalOutputTokens := 0
	totalEstimated := 0

	for _, msg := range history {
		input := msg.GetInputTokens()
		output := msg.GetOutputTokens()

		if input > 0 || output > 0 {
			totalInputTokens += input
			totalOutputTokens += output
		} else {
			// Using estimation fallback
			estimated := sessions.EstimateTokens(msg)
			totalEstimated += estimated
		}
	}

	// Format capacity string
	capacityStr := "unlimited"
	if metadata.MaxHistoryTokens > 0 {
		capacityStr = fmt.Sprintf("%.1f%% of %d", percentage, metadata.MaxHistoryTokens)
	}

	// Build detailed response
	response := fmt.Sprintf(
		"token input: %d, "+
			"token output: %d, "+
			"context capacity: %s",
		totalInputTokens,
		totalOutputTokens,
		capacityStr,
	)

	ctx.Reply(response)
}
