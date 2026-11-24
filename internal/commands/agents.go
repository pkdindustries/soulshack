package commands

import (
	"time"

	"pkdindustries/soulshack/internal/core"
	"pkdindustries/soulshack/internal/subagents"
)

// AgentsCommand reports the child agents working for this conversation.
type AgentsCommand struct{}

func (c *AgentsCommand) Name() string    { return "/agents" }
func (c *AgentsCommand) AdminOnly() bool { return false }

func (c *AgentsCommand) Execute(turn *core.Turn) {
	agents := turn.GetSystem().GetAgents()
	if agents == nil {
		turn.Reply("Agents are not enabled.")
		return
	}

	turn.Reply(subagents.Report(agents, turn.GetConversationKey(), time.Now()))
}
