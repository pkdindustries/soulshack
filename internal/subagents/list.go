package subagents

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/alexschlessinger/pollytool/schema"
	"github.com/alexschlessinger/pollytool/tools"

	"pkdindustries/soulshack/internal/core"
)

// listToolName is polly's name for this, which is worth keeping: its
// ChildRegistry denies the name outright, so a child never inherits a view of
// its parent's roster.
const listToolName = "list_agents"

// newListTool lets the model answer the question the channel actually asks,
// which is never "/agents" but "are you still doing that thing". The bot knows
// it delegated, but a turn minutes later has only the transcript to go on, and
// the transcript says a child started, not whether it is still going.
func newListTool(tracker *Tracker) tools.Tool {
	return &tools.Func{
		Name:   listToolName,
		Desc:   "List the agents still working on tasks you delegated in this conversation, with how long each has been going. Call it when asked what you are working on, whether something is still running, or how long it has been; a delegated task is finished only when its report has arrived, so do not guess from the conversation. Agents working for other conversations are counted but not described.",
		Params: schema.Params{},
		Run: func(ctx context.Context, args tools.Args) (string, error) {
			chat, ok := core.ChatFromContext(ctx)
			if !ok {
				return "", errors.New("no chat context available")
			}
			if err := ctx.Err(); err != nil {
				return "", err
			}

			here, elsewhere := Running(tracker, chat.GetConversationKey())
			chat.GetLogger().Info("agents_listed", "here", len(here), "elsewhere", elsewhere)
			return Report(here, elsewhere, time.Now()), nil
		},
	}
}

// Report renders what is running for whoever asked, model or person.
func Report(here []core.AgentInfo, elsewhere int, now time.Time) string {
	others := ""
	switch {
	case elsewhere == 1:
		others = " 1 agent is working for another conversation."
	case elsewhere > 1:
		others = fmt.Sprintf(" %d agents are working for other conversations.", elsewhere)
	}

	if len(here) == 0 {
		return strings.TrimSpace("No agents are working for this conversation." + others)
	}

	described := make([]string, 0, len(here))
	for _, info := range here {
		described = append(described, Describe(info, now))
	}
	return strings.TrimSpace(fmt.Sprintf("%d working: %s.%s", len(here), strings.Join(described, "; "), others))
}
