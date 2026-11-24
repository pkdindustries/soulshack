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
	"pkdindustries/soulshack/internal/memory"
)

const defaultLabel = "subagent"

// defaultTimeout applies when the configured limit is <= 0; unlike elsewhere, zero is not unlimited.
const defaultTimeout = 15 * time.Minute

// Register adds the spawn and list tools to registry.
func Register(registry *tools.ToolRegistry, tracker *Tracker) {
	registry.Register(newSpawnTool(tracker))
	registry.MarkAlwaysAllowed(subagent.ToolName)

	registry.Register(newListTool(tracker))
	registry.MarkAlwaysAllowed(listToolName)
}

// newSpawnTool uses polly's tool name with soulshack's own schema; polly's
// subagent.Tool would still accept workspace, isolation and wait arguments.
func newSpawnTool(tracker *Tracker) tools.Tool {
	return &tools.Func{
		Name:        subagent.ToolName,
		LongRunning: true,
		Coordinator: true,
		Desc:        "Hand a self-contained task to a child agent that works in the background. Use it for work that would take long enough to leave the channel waiting: research, reading something large, anything with many tool calls. The child starts with no context beyond the brief you write, so state the goal, whatever facts it needs, and what to report back. It cannot talk to the channel or spawn agents of its own, and it reports only to you. This call returns as soon as the child has started; its reply arrives later as a message. Say in the channel that you have delegated, then carry on: do not claim the result before it arrives.",
		Params: schema.Params{
			"task":  schema.S("The complete brief for the agent. It starts with no other context."),
			"label": schema.S("Two to five words naming the job, shown in the channel while it runs."),
			"tools": schema.Strings("Names or globs of the tools the agent may use. Omitted: whatever it can inherit."),
			"model": schema.S("Model to run the agent on, as provider/model. Default: the configured agent model."),
		},
		Required: []string{"task", "label"},
		Run: func(ctx context.Context, args tools.Args) (string, error) {
			task := strings.TrimSpace(args.String("task"))
			if task == "" {
				return "", tools.NewToolError("task is required: the complete brief for the agent", "INVALID_ARGS")
			}
			result, err := spawn(ctx, tracker, subagent.Request{
				Task:  task,
				Label: args.String("label"),
				Tools: args.StringSlice("tools"),
				Model: strings.TrimSpace(args.String("model")),
			})
			if err != nil {
				return "", tools.NewToolError(err.Error(), "SPAWN_REFUSED")
			}
			return result.String(), nil
		},
	}
}

// spawn starts a child and returns without waiting for it.
func spawn(ctx context.Context, tracker *Tracker, req subagent.Request) (subagent.Result, error) {
	chat, err := chatFor(ctx)
	if err != nil {
		return subagent.Result{}, err
	}

	cfg := chat.GetConfig()
	label := strings.TrimSpace(req.Label)
	if label == "" {
		label = defaultLabel
	}

	id, err := tracker.admit(core.AgentInfo{
		Label:        label,
		Conversation: chat.GetConversationKey(),
		Source:       chat.GetSource(),
		Started:      time.Now(),
	}, cfg.Bot.SubagentMax, cfg.Bot.SubagentMaxPerChat)
	if err != nil {
		return subagent.Result{}, err
	}

	// Derived from the bot's context, so the child outlives this turn.
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

	// Announced regardless of showtoolactions: the turn ends with work outstanding.
	chat.ReplyAction("delegating: " + label)
	chat.GetLogger().Info("agent_started", "agent", label, "model", spec.Model)

	done := make(chan struct{})
	go func() {
		defer close(done)
		defer tracker.forget(id)
		defer cancel()
		run(background, spec)
	}()

	return subagent.Result{Started: true, Session: label, Done: done}, nil
}

// run works the child to completion and delivers its report.
func run(chat core.ChatContextInterface, spec core.SubagentSpec) {
	logger := chat.GetLogger().With("agent", spec.Label)
	started := time.Now()

	// Scratch conversation for artifacts, so large tool output stays out of requests.
	var result core.SubagentResult
	var err error
	if scratchErr := chat.GetSystem().GetMemory().WithScratch(chat, func(scratch *memory.Conversation) {
		spec.Artifacts = scratch.Artifacts()
		result, err = chat.GetSystem().GetLLM().RunSubagent(chat, spec)
	}); scratchErr != nil {
		err = scratchErr
	}
	duration := time.Since(started)

	if err != nil {
		switch {
		// Bot shutdown: stay silent.
		case errors.Is(chat.Err(), context.Canceled):
			logger.Info("agent_canceled", "duration_ms", duration.Milliseconds())
		// Timeout: the channel was promised a report, so say it gave up.
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

// deliver queues the report as a turn of its own for the bot to answer.
func deliver(chat core.ChatContextInterface, spec core.SubagentSpec, text string) {
	label := spec.Label
	text = strings.TrimSpace(text)
	if text == "" {
		text = "(the agent returned no reply)"
	}

	// Own budget, not the child's leftovers: one timeout to wait for the conversation, one to answer.
	budget := 2 * spec.Config.API.Timeout
	if budget <= 0 {
		budget = defaultTimeout
	}
	chat, cancel := chat.Background(budget)
	defer cancel()

	core.WithConversation(chat, "subagent", func(turn *core.Turn) {
		// Conversation forgotten: relay verbatim rather than answer out of context.
		if turn.Conversation.Len() == 0 {
			turn.Reply(verbatim(label, text))
			return
		}
		answered := llm.Drain(llm.CompleteWithoutDelegating(turn, fmt.Sprintf("(agent:%s) %s", label, text)), turn.Reply)
		// A failed or silent relay still owes the channel the report.
		if !answered {
			chat.GetLogger().Warn("agent_relay_incomplete", "agent", label)
			turn.Reply(verbatim(label, text))
		}
	}, func() {
		// Conversation stayed busy past the budget.
		chat.GetLogger().Warn("agent_delivery_queued_out", "agent", label)
		chat.Reply(verbatim(label, text))
	})
}

func verbatim(label, text string) string {
	return fmt.Sprintf("[%s] %s", label, text)
}
