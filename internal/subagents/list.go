package subagents

import (
	"context"
	"errors"
	"time"

	"github.com/alexschlessinger/pollytool/schema"
	"github.com/alexschlessinger/pollytool/tools"

	"pkdindustries/soulshack/internal/core"
)

// listToolName is polly's name for this, which is worth keeping: its
// ChildRegistry denies the name outright, so a child never inherits a view of
// its parent's roster.
const listToolName = "list_agents"

// chatFor finds the chat a tool call is running under, and reports a call that
// has already been cancelled.
func chatFor(ctx context.Context) (core.ChatContextInterface, error) {
	chat, ok := core.ChatFromContext(ctx)
	if !ok {
		return nil, errors.New("agents are only available from a chat turn")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return chat, nil
}

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
			chat, err := chatFor(ctx)
			if err != nil {
				return "", err
			}
			report := Report(tracker, chat.GetConversationKey(), time.Now())
			chat.GetLogger().Info("agents_listed", "report", report)
			return report, nil
		},
	}
}
