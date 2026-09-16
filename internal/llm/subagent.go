package llm

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/alexschlessinger/pollytool/llm"
	"github.com/alexschlessinger/pollytool/messages"
	"github.com/alexschlessinger/pollytool/subagent"
	"github.com/alexschlessinger/pollytool/tools"

	"pkdindustries/soulshack/internal/config"
	"pkdindustries/soulshack/internal/core"
)

// childMaxIterations bounds a child's tool loop. It is shorter than the
// parent's: a child has one brief to answer, and a child that has not
// finished by then is looping rather than working.
const childMaxIterations = 8

// ircWriteTools are the IRC tools a child never gets. A child acts on a brief
// the parent model wrote from channel traffic, under whatever authority the
// original event carried, possibly long after that event. The read-only IRC
// tools are left in place: a child may look at who is in the channel, it may
// not do anything to them.
var ircWriteTools = []string{
	"irc__op",
	"irc__kick",
	"irc__ban",
	"irc__mode_set",
	"irc__invite",
	"irc__topic",
	"irc__action",
}

// childPrompt tells a child what it is. It replaces the bot's own prompt
// rather than extending it: a child is not talking to the channel, it is
// answering the coordinator that spawned it.
const childPrompt = `You are a subagent of %s, an IRC bot. You were given one task and you have no other context; the conversation you were spawned from is not visible to you.

Do the task and report the result. Your reply goes to the coordinating agent, not to the channel: write plain prose, no IRC formatting, no greeting, no sign-off. Lead with the answer. Say plainly what you could not determine rather than guessing.`

// RunSubagent runs one child agent to completion over a narrowed view of the
// parent's tools. It blocks for as long as the child runs, which is why its
// caller gives it a context of its own rather than a turn's.
func (p *PollyLLM) RunSubagent(ctx context.Context, spec core.SubagentSpec) (core.SubagentResult, error) {
	cfg := spec.Config
	if cfg == nil {
		return core.SubagentResult{}, errors.New("subagent has no configuration")
	}
	if spec.Registry == nil {
		return core.SubagentResult{}, errors.New("subagent has no tool registry")
	}

	registry, release, err := childRegistry(spec.Registry, spec.Tools)
	if err != nil {
		return core.SubagentResult{}, err
	}
	defer release()

	agent := createAgent(p.client, registry, llm.AgentConfig{
		MaxIterations: childMaxIterations,
		ToolTimeout:   cfg.API.Timeout,
	})
	defer agent.Close()

	req := NewCompletionRequest(cfg, []messages.ChatMessage{
		{Role: messages.MessageRoleSystem, Content: fmt.Sprintf(childPrompt, cfg.Server.Nick)},
		{Role: messages.MessageRoleUser, Content: spec.Task},
	}, cfg.Session.MaxContext, registry.All())
	if model := childModel(spec, cfg); model != "" {
		req.Model = model
	}
	req.BaseURL = baseURLForModel(req.Model, cfg.API)

	resp, err := agent.Run(ctx, req, nil)
	if err != nil {
		return core.SubagentResult{}, err
	}
	if resp == nil || resp.Message == nil {
		return core.SubagentResult{}, errors.New("agent returned no reply")
	}

	result := core.SubagentResult{Text: strings.TrimSpace(resp.Message.GetContent())}
	result.InputTokens, result.OutputTokens = childTokens(resp.AllMessages)
	return result, nil
}

// childModel resolves which model a child runs on: what the brief asked for,
// then the configured subagent model, then, by leaving it unset, the bot's own.
func childModel(spec core.SubagentSpec, cfg *config.Configuration) string {
	if spec.Model != "" {
		return spec.Model
	}
	return cfg.Bot.SubagentModel
}

// childRegistry narrows the parent's tools to what a child may use. Polly
// drops nested spawning and the coordination tools that carry the parent's
// identity; soulshack drops the IRC tools that act on the channel. The
// returned release closes both views, innermost first, and leaves the
// parent's registry and its MCP connections open.
func childRegistry(parent *tools.ToolRegistry, allow []string) (*tools.ToolRegistry, func(), error) {
	inherited := subagent.ChildRegistry(parent, allow)
	child := inherited.Derive(tools.DenyTools(ircWriteTools...))
	release := func() {
		child.Close()
		inherited.Close()
	}
	if err := subagent.CheckChildTools(allow, child); err != nil {
		release()
		return nil, nil, err
	}
	return child, release, nil
}

// childTokens sums a child's usage the way polly reports a turn: providers
// count input per call, cumulatively, so the largest call stands for the run,
// while output is summed across calls.
func childTokens(all []messages.ChatMessage) (in, out int) {
	for _, m := range all {
		if input := m.GetInputTokens(); input > in {
			in = input
		}
		out += m.GetOutputTokens()
	}
	return in, out
}
