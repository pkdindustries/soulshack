package commands

import (
	"pkdindustries/soulshack/internal/core"
)

// ClearCommand resets the current conversation's transcript and artifacts.
type ClearCommand struct{}

func (c *ClearCommand) Name() string            { return "/clear" }
func (c *ClearCommand) AdminOnly() bool         { return true }
func (c *ClearCommand) NeedsConversation() bool { return true }

func (c *ClearCommand) Execute(turn *core.Turn) {
	if err := turn.Conversation.Clear(); err != nil {
		turn.GetLogger().Error("conversation_clear_failed", "error", err)
		turn.Reply("Failed to clear conversation.")
		return
	}
	turn.Reply("Conversation cleared.")
}
