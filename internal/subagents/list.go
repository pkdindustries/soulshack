package subagents

import (
	"context"
	"errors"
	"time"

	"github.com/alexschlessinger/pollytool/schema"
	"github.com/alexschlessinger/pollytool/tools"

	"pkdindustries/soulshack/internal/core"
)

// listToolName matches polly's, which ChildRegistry denies to children.
const listToolName = "list_agents"

// chatFor also errors if ctx is already cancelled.
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

// newListTool lets the model check whether delegated work is still running; the transcript can't say.
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
