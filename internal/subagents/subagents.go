package subagents

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/alexschlessinger/pollytool/schema"
	"github.com/alexschlessinger/pollytool/subagent"
	"github.com/alexschlessinger/pollytool/tools"

	"pkdindustries/soulshack/internal/core"
	"pkdindustries/soulshack/internal/llm"
)

// slots is what polly's own concurrency bound is set to. It is deliberately
// out of reach: that bound makes a caller wait for a slot, and a tool call
// that waits is a turn that has stopped answering the channel. The tracker
// bounds children instead, by refusing rather than blocking.
const slots = 1024

// defaultLabel names a child whose brief did not.
const defaultLabel = "subagent"

// defaultTimeout bounds a child whose configured limit is missing or zero.
// Zero means unlimited elsewhere in the configuration, but a child agent with
// no deadline is one that can spend on a provider until the bot is restarted,
// so here it means the default instead.
const defaultTimeout = 15 * time.Minute

// Register offers the model the spawning tool and, since delegated work
// finishes out of sight, a way to look at what is still running.
func Register(registry *tools.ToolRegistry, tracker *Tracker) {
	registry.Register(&spawnTool{Tool: subagent.NewTool(runner(tracker), subagent.WithMaxConcurrent(slots))})
	registry.MarkAlwaysAllowed(subagent.ToolName)

	registry.Register(newListTool(tracker))
	registry.MarkAlwaysAllowed(listToolName)
}

// spawnTool is polly's spawning tool with a schema of soulshack's own. The
// behavior it keeps is the part worth keeping: argument parsing, the
// exemption from the per-tool timeout, and the result text a parent reads.
// What it replaces is the description, which otherwise offers a model
// workspaces, isolation modes and a choice about waiting, none of which an
// IRC bot has.
type spawnTool struct {
	*subagent.Tool
}

func (t *spawnTool) GetSchema() *schema.ToolSchema {
	return schema.Tool(subagent.ToolName,
		"Hand a self-contained task to a child agent that works in the background. Use it for work that would take long enough to leave the channel waiting: research, reading something large, anything with many tool calls. The child starts with no context beyond the brief you write, so state the goal, whatever facts it needs, and what to report back. It cannot talk to the channel or spawn agents of its own, and it reports only to you. This call returns as soon as the child has started; its reply arrives later as a message. Say in the channel that you have delegated, then carry on: do not claim the result before it arrives.",
		schema.Params{
			"task":  schema.S("The complete brief for the agent. It starts with no other context."),
			"label": schema.S("Two to five words naming the job, shown in the channel while it runs."),
			"tools": schema.Strings("Names or globs of the tools the agent may use. Omitted: whatever it can inherit."),
			"model": schema.S("Model to run the agent on, as provider/model. Default: the configured agent model."),
		},
		"task", "label")
}

// runner starts a child and returns. Everything after the start happens on
// the child's own goroutine, under a context that outlives this turn.
func runner(tracker *Tracker) subagent.Runner {
	return func(ctx context.Context, req subagent.Request) (subagent.Result, error) {
		chat, ok := core.ChatFromContext(ctx)
		if !ok {
			return subagent.Result{}, errors.New("agents can only be spawned from a chat turn")
		}
		if err := ctx.Err(); err != nil {
			return subagent.Result{}, err
		}

		cfg := chat.GetConfig()
		label := strings.TrimSpace(req.Label)
		if label == "" {
			label = defaultLabel
		}

		release, err := tracker.admit(core.AgentInfo{
			Label:        label,
			Conversation: chat.GetConversationKey(),
			Source:       chat.GetSource(),
			Started:      time.Now(),
		}, cfg.Bot.SubagentMax, cfg.Bot.SubagentMaxPerChat)
		if err != nil {
			return subagent.Result{}, err
		}

		// The child's own lifetime. It is the bot's context underneath, not
		// this turn's, so the child survives the end of the turn and still
		// ends when the bot does.
		timeout := cfg.Bot.SubagentTimeout
		if timeout <= 0 {
			timeout = defaultTimeout
		}
		background, cancel := chat.Background(timeout)

		spec := core.SubagentSpec{
			Task:     req.Task,
			Label:    label,
			Model:    req.Model,
			Tools:    req.Tools,
			Registry: chat.GetSystem().GetToolRegistry(),
			Config:   cfg,
		}

		// Always said, whatever showtoolactions is set to. That setting hides
		// the running commentary on a turn that is about to answer anyway;
		// this is the opposite, a turn ending with the work still outstanding.
		// The channel is owed the same account of it as of its outcome, which
		// is reported unconditionally further down.
		chat.ReplyAction("delegating: " + label)
		chat.GetLogger().Info("agent_started", "agent", label, "model", spec.Model)

		// Done tells polly the child is still holding its slot: the tool
		// call is long over by the time the child settles.
		done := make(chan struct{})
		go func() {
			defer close(done)
			defer release()
			defer cancel()
			run(background, spec)
		}()

		return subagent.Result{Started: true, Session: label, Done: done}, nil
	}
}

// run works the child to completion and reports what it said. It is the whole
// of a child's life after the spawning turn has ended.
func run(chat core.ChatContextInterface, spec core.SubagentSpec) {
	logger := chat.GetLogger().With("agent", spec.Label)
	started := time.Now()

	result, err := chat.GetSystem().GetLLM().RunSubagent(chat, spec)
	duration := time.Since(started)

	if err != nil {
		switch {
		// A bot shutting down cancels every child. That is not a failure
		// anyone in the channel needs to hear about, and there is nobody
		// left to hear it.
		case errors.Is(chat.Err(), context.Canceled):
			logger.Info("agent_canceled", "duration_ms", duration.Milliseconds())
		// Running out of time is the opposite: the channel was told this
		// was coming, so it has to be told that it is not. Distinguishing
		// the two matters because both end this same context.
		case errors.Is(chat.Err(), context.DeadlineExceeded):
			logger.Error("agent_timeout", "duration_ms", duration.Milliseconds())
			chat.ReplyAction(fmt.Sprintf("%s gave up after %s", spec.Label, duration.Round(time.Second)))
		default:
			logger.Error("agent_failed", "duration_ms", duration.Milliseconds(), "error", err)
			chat.ReplyAction(fmt.Sprintf("%s failed: %v", spec.Label, err))
		}
		return
	}

	logger.Info("agent_complete",
		"duration_ms", duration.Milliseconds(),
		"input_tokens", result.InputTokens,
		"output_tokens", result.OutputTokens,
	)
	deliver(chat, spec, result.Text)
}

// deliver puts the child's report in front of the bot as a turn of its own,
// queued behind whatever else the conversation is doing, and lets the bot
// answer it in its own voice.
func deliver(chat core.ChatContextInterface, spec core.SubagentSpec, text string) {
	label := spec.Label
	text = strings.TrimSpace(text)
	if text == "" {
		text = "(the agent returned no reply)"
	}

	// Delivering the report needs a budget of its own. Sharing the child's
	// would mean that a child which spent almost all of its time producing a
	// report would have almost none left to deliver it, and the report would
	// be dropped for having arrived too near the deadline. One timeout's
	// worth of waiting for the conversation, one of answering in it.
	budget := 2 * spec.Config.API.Timeout
	if budget <= 0 {
		budget = defaultTimeout
	}
	chat, cancel := chat.Background(budget)
	defer cancel()

	core.WithConversation(chat, "subagent", func(turn *core.Turn) {
		// The conversation this was asked in has since been forgotten, so
		// there is nothing left for the bot to answer the report against:
		// pass it on as it stands rather than reacting to it out of context.
		if turn.Conversation.Len() == 0 {
			turn.Reply(verbatim(label, text))
			return
		}
		answered := false
		for chunk := range llm.CompleteRelay(turn, fmt.Sprintf("(agent:%s) %s", label, text)) {
			answered = true
			turn.Reply(chunk)
		}
		// The bot had nothing to say, which at this point means the
		// completion failed rather than that the report was not worth
		// answering. Either way the channel is owed the report.
		if !answered {
			chat.GetLogger().Warn("agent_relay_silent", "agent", label)
			turn.Reply(verbatim(label, text))
		}
	}, func() {
		// The conversation stayed busy for the rest of the child's lifetime.
		// The report is still worth having.
		chat.GetLogger().Warn("agent_delivery_queued_out", "agent", label)
		chat.Reply(verbatim(label, text))
	})
}

// verbatim is the child's own words, for when the bot cannot answer them.
func verbatim(label, text string) string {
	return fmt.Sprintf("[%s] %s", label, text)
}
