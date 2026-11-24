package commands

import (
	"strings"
	"testing"
	"time"

	"pkdindustries/soulshack/internal/core"
	mocktest "pkdindustries/soulshack/internal/testing"
)

// fakeAgents avoids an import cycle with the real tracker.
type fakeAgents []core.AgentInfo

func (f fakeAgents) List() []core.AgentInfo { return f }

func TestAgentsCommand(t *testing.T) {
	for _, tt := range []struct {
		name     string
		agents   core.Agents
		wants    []string
		excludes []string
	}{
		{name: "disabled", agents: nil, wants: []string{"not enabled"}},
		{name: "none running", agents: fakeAgents{}, wants: []string{"No agents are working"}},
		{
			name: "running here",
			agents: fakeAgents{
				{Label: "bird lookup", Conversation: "#test", Source: "alice", Started: time.Now().Add(-90 * time.Second)},
				{Label: "long read", Conversation: "#test", Source: "bob", Started: time.Now().Add(-3 * time.Second)},
			},
			wants: []string{"2 working", "bird lookup", "alice", "1m30s", "long read", "bob"},
		},
		{
			// Shows the bot is busy, not with what or for whom.
			name: "running elsewhere",
			agents: fakeAgents{
				{Label: "secret errand", Conversation: "bob", Source: "bob", Started: time.Now()},
				{Label: "other channel", Conversation: "#other", Source: "carol", Started: time.Now()},
			},
			wants:    []string{"No agents are working for this conversation", "2 agents are working for other conversations"},
			excludes: []string{"secret errand", "other channel", "bob", "carol", "#other"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			sys := mocktest.NewMockSystem(t)
			sys.Agents = tt.agents
			ctx := mocktest.NewTurnContext(t, sys, "#test")

			(&AgentsCommand{}).Execute(ctx.Turn())

			if ctx.ReplyCount() != 1 {
				t.Fatalf("expected one reply, got %q", ctx.AllReplies())
			}
			for _, want := range tt.wants {
				if !strings.Contains(ctx.LastReply(), want) {
					t.Errorf("reply %q is missing %q", ctx.LastReply(), want)
				}
			}
			for _, leak := range tt.excludes {
				if strings.Contains(ctx.LastReply(), leak) {
					t.Errorf("reply %q carries %q out of the conversation it belongs to", ctx.LastReply(), leak)
				}
			}
		})
	}
}
