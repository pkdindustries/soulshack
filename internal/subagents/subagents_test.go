package subagents

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/alexschlessinger/pollytool/messages"
	"github.com/alexschlessinger/pollytool/subagent"
	"github.com/alexschlessinger/pollytool/tools"

	"pkdindustries/soulshack/internal/config"
	"pkdindustries/soulshack/internal/core"
	mocktest "pkdindustries/soulshack/internal/testing"
)

// spawn starts one child through the runner and hands back the result, so a
// test can wait on the child rather than sleeping until it looks finished.
func spawn(t *testing.T, tracker *Tracker, ctx core.ChatContextInterface, req subagent.Request) subagent.Result {
	t.Helper()
	result, err := runner(tracker)(ctx, req)
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	if !result.Started {
		t.Fatal("spawn did not report the child as started")
	}
	return result
}

// settled waits for a child to finish, with a bound so a broken one fails the
// test instead of hanging it.
func settled(t *testing.T, result subagent.Result) {
	t.Helper()
	select {
	case <-result.Done:
	case <-time.After(5 * time.Second):
		t.Fatal("child never settled")
	}
}

// seed gives a conversation the history a spawning turn would have left,
// since a report is delivered into an ongoing conversation.
func seed(t *testing.T, ctx core.ChatContextInterface) {
	t.Helper()
	core.WithConversation(ctx, "seed", func(turn *core.Turn) {
		turn.Conversation.Append(messages.User("(nick:alice) go find out about the thing"))
	}, nil)
}

func newFixture(t *testing.T) (*mocktest.MockSystem, *mocktest.MockChatContext, *mocktest.MockLLM, *Tracker) {
	t.Helper()
	sys := mocktest.NewMockSystem(t)
	model := &mocktest.MockLLM{Responses: []string{"right, here is what it found"}}
	sys.LLM = model
	tracker := NewTracker()
	sys.Agents = tracker
	ctx := mocktest.NewMockContext().WithSystem(sys).WithSource("alice")
	return sys, ctx, model, tracker
}

// Delegation ends a turn with the work still outstanding, so the channel is
// told either way. showtoolactions hides the running commentary on a turn that
// is about to answer; this is not that, and it says what was delegated rather
// than that a tool was called. With the setting on, the commentary announces
// the call as well, from the layer that runs it.
func TestDelegationIsAnnouncedWhateverTheToolActionSettingIs(t *testing.T) {
	for _, show := range []bool{true, false} {
		sys, ctx, model, tracker := newFixture(t)
		mocktest.SetConfig(t, sys, func(c *config.Configuration) { c.Bot.ShowToolActions = show })
		model.Subagent = func(context.Context, core.SubagentSpec) (core.SubagentResult, error) {
			return core.SubagentResult{Text: "done"}, nil
		}

		settled(t, spawn(t, tracker, ctx, subagent.Request{Task: "find out", Label: "bird lookup"}))

		actions := ctx.AllActions()
		if len(actions) != 1 || actions[0] != "delegating: bird lookup" {
			t.Fatalf("showtoolactions=%v: expected the delegation announced, got %q", show, actions)
		}
	}
}

// The point of the feature: the turn that spawns a child must end while the
// child is still working, or the channel waits for it.
func TestSpawnReturnsWhileTheChildIsStillWorking(t *testing.T) {
	_, ctx, model, tracker := newFixture(t)

	working := make(chan struct{})
	release := make(chan struct{})
	model.Subagent = func(context.Context, core.SubagentSpec) (core.SubagentResult, error) {
		close(working)
		<-release
		return core.SubagentResult{Text: "found it"}, nil
	}

	result := spawn(t, tracker, ctx, subagent.Request{Task: "find out", Label: "the lookup"})
	<-working

	select {
	case <-result.Done:
		t.Fatal("spawn reported the child settled while it was still working")
	default:
	}
	if running := tracker.List(); len(running) != 1 || running[0].Label != "the lookup" {
		t.Fatalf("expected one running child named for the brief, got %+v", running)
	}

	close(release)
	settled(t, result)

	if running := tracker.List(); len(running) != 0 {
		t.Fatalf("a settled child is still tracked: %+v", running)
	}
}

// The child's report reaches the channel as a turn of its own, in the bot's
// voice rather than the child's.
func TestReportIsDeliveredAsTheBotsOwnTurn(t *testing.T) {
	_, ctx, model, tracker := newFixture(t)
	seed(t, ctx)
	model.Subagent = func(context.Context, core.SubagentSpec) (core.SubagentResult, error) {
		return core.SubagentResult{Text: "the thing is a kind of bird"}, nil
	}

	settled(t, spawn(t, tracker, ctx, subagent.Request{Task: "find out", Label: "bird lookup"}))

	if replies := ctx.AllReplies(); len(replies) != 1 || replies[0] != "right, here is what it found" {
		t.Fatalf("expected the bot's answer in the channel, got %q", replies)
	}

	request := model.LastRequest
	if request == nil {
		t.Fatal("the report did not run a completion")
	}
	last := request.Messages[len(request.Messages)-1]
	if last.Role != messages.MessageRoleUser || last.Content != "(agent:bird lookup) the thing is a kind of bird" {
		t.Fatalf("unexpected report message: %+v", last)
	}
	if !strings.Contains(request.Messages[len(request.Messages)-2].Content, "go find out about the thing") {
		t.Fatalf("the report was not delivered into the conversation it came from: %+v", request.Messages)
	}
}

// A report the bot answers is input the bot generated for itself. If that turn
// could spawn, nothing in the channel would have to be said for it to keep
// spawning.
func TestDeliveryTurnCannotSpawnAgain(t *testing.T) {
	sys, ctx, model, tracker := newFixture(t)
	Register(sys.ToolRegistry, tracker)
	seed(t, ctx)
	model.Subagent = func(context.Context, core.SubagentSpec) (core.SubagentResult, error) {
		return core.SubagentResult{Text: "done"}, nil
	}

	settled(t, spawn(t, tracker, ctx, subagent.Request{Task: "find out", Label: "lookup"}))

	if !offers(sys.ToolRegistry.All(), subagent.ToolName) {
		t.Fatal("the bot's own tools no longer include the spawning tool")
	}
	if offers(model.LastRequest.Tools, subagent.ToolName) {
		t.Fatal("the delivery turn was offered the spawning tool")
	}
}

func offers(offered []tools.Tool, name string) bool {
	for _, tool := range offered {
		if tool.GetName() == name {
			return true
		}
	}
	return false
}

// A child can outlive the conversation that asked for it. Answering a report
// against a transcript that has since been forgotten would be answering a
// question nobody asked, so the report is passed on as it stands.
func TestForgottenConversationGetsTheReportVerbatim(t *testing.T) {
	_, ctx, model, tracker := newFixture(t)
	model.Subagent = func(context.Context, core.SubagentSpec) (core.SubagentResult, error) {
		return core.SubagentResult{Text: "it was a bird all along"}, nil
	}

	settled(t, spawn(t, tracker, ctx, subagent.Request{Task: "find out", Label: "bird lookup"}))

	if replies := ctx.AllReplies(); len(replies) != 1 || replies[0] != "[bird lookup] it was a bird all along" {
		t.Fatalf("expected the child's own words, got %q", replies)
	}
	if model.LastRequest != nil {
		t.Fatal("a forgotten conversation still ran a completion")
	}
}

// A child that fails says so in the channel, and stops occupying a slot.
func TestFailedChildReportsAndReleasesItsSlot(t *testing.T) {
	_, ctx, model, tracker := newFixture(t)
	model.Subagent = func(context.Context, core.SubagentSpec) (core.SubagentResult, error) {
		return core.SubagentResult{}, context.DeadlineExceeded
	}

	settled(t, spawn(t, tracker, ctx, subagent.Request{Task: "find out", Label: "doomed lookup"}))

	if actions := ctx.AllActions(); len(actions) != 2 || !strings.Contains(actions[1], "doomed lookup failed") {
		t.Fatalf("expected the delegation and then the failure, got %q", actions)
	}
	if running := tracker.List(); len(running) != 0 {
		t.Fatalf("a failed child is still tracked: %+v", running)
	}
}

// Shutting the bot down cancels every child. Nobody is left in the channel to
// read a complaint about it.
func TestCancelledChildIsQuiet(t *testing.T) {
	_, ctx, model, tracker := newFixture(t)
	bot, shutdown := context.WithCancel(context.Background())
	ctx.Parent = bot
	model.Subagent = func(ctx context.Context, _ core.SubagentSpec) (core.SubagentResult, error) {
		<-ctx.Done()
		return core.SubagentResult{}, ctx.Err()
	}

	result := spawn(t, tracker, ctx, subagent.Request{Task: "find out", Label: "interrupted"})
	shutdown()
	settled(t, result)

	// The delegation was announced when it started; nothing follows it.
	if replies, actions := ctx.AllReplies(), ctx.AllActions(); len(replies) != 0 || len(actions) != 1 {
		t.Fatalf("a cancelled child spoke past its delegation: replies %q, actions %q", replies, actions)
	}
}

// Running out of time is not shutting down, though both end the same context.
// The channel was told the work was coming, so it has to be told it is not.
func TestTimedOutChildSaysSo(t *testing.T) {
	sys, ctx, model, tracker := newFixture(t)
	mocktest.SetConfig(t, sys, func(c *config.Configuration) {
		c.Bot.SubagentTimeout = 50 * time.Millisecond
	})
	model.Subagent = func(ctx context.Context, _ core.SubagentSpec) (core.SubagentResult, error) {
		<-ctx.Done()
		return core.SubagentResult{}, ctx.Err()
	}

	settled(t, spawn(t, tracker, ctx, subagent.Request{Task: "find out", Label: "slow lookup"}))

	if actions := ctx.AllActions(); len(actions) != 2 || !strings.Contains(actions[1], "slow lookup gave up") {
		t.Fatalf("expected the channel to hear that the agent ran out of time, got %q", actions)
	}
}

// A child that spends nearly all of its budget still has a report worth
// delivering. Delivery on the child's leftovers would drop it.
func TestReportSurvivesAChildThatUsedItsWholeBudget(t *testing.T) {
	sys, ctx, model, tracker := newFixture(t)
	seed(t, ctx)
	mocktest.SetConfig(t, sys, func(c *config.Configuration) {
		c.Bot.SubagentTimeout = 60 * time.Millisecond
	})
	model.Subagent = func(ctx context.Context, _ core.SubagentSpec) (core.SubagentResult, error) {
		// Finishes with almost nothing left of its own deadline.
		time.Sleep(50 * time.Millisecond)
		return core.SubagentResult{Text: "just made it"}, nil
	}
	// Answering the report takes longer than the child had left, so a
	// delivery on the child's leftovers is one that never gets to speak.
	model.Delay = 50 * time.Millisecond

	settled(t, spawn(t, tracker, ctx, subagent.Request{Task: "find out", Label: "slow lookup"}))

	if replies := ctx.AllReplies(); len(replies) != 1 || replies[0] != "right, here is what it found" {
		t.Fatalf("the report was dropped for arriving near the child's deadline: %q", replies)
	}
}

// If the bot cannot answer the report, the channel is still owed the report.
func TestSilentRelayFallsBackToTheChildsWords(t *testing.T) {
	_, ctx, model, tracker := newFixture(t)
	seed(t, ctx)
	model.Responses = nil // the completion produced nothing
	model.Subagent = func(context.Context, core.SubagentSpec) (core.SubagentResult, error) {
		return core.SubagentResult{Text: "it was a bird"}, nil
	}

	settled(t, spawn(t, tracker, ctx, subagent.Request{Task: "find out", Label: "bird lookup"}))

	if replies := ctx.AllReplies(); len(replies) != 1 || replies[0] != "[bird lookup] it was a bird" {
		t.Fatalf("expected the child's own words after a silent relay, got %q", replies)
	}
}

// The caps bound what can be in flight. They refuse rather than wait: a
// spawning call that blocks is a turn that has stopped answering the channel.
func TestCapsRefuseRatherThanWait(t *testing.T) {
	_, ctx, model, tracker := newFixture(t)
	release := make(chan struct{})
	model.Subagent = func(context.Context, core.SubagentSpec) (core.SubagentResult, error) {
		<-release
		return core.SubagentResult{Text: "done"}, nil
	}

	first := spawn(t, tracker, ctx, subagent.Request{Task: "one", Label: "one"})
	second := spawn(t, tracker, ctx, subagent.Request{Task: "two", Label: "two"})

	if _, err := runner(tracker)(ctx, subagent.Request{Task: "three", Label: "three"}); err == nil {
		t.Fatal("the per-conversation cap admitted a third child")
	}

	close(release)
	settled(t, first)
	settled(t, second)

	if _, err := runner(tracker)(ctx, subagent.Request{Task: "three", Label: "three"}); err != nil {
		t.Fatalf("a freed slot was not reusable: %v", err)
	}
}

// The tool the model is offered describes what this host actually does, not
// polly's workspaces and waiting.
func TestOfferedSchemaIsTheBotsOwn(t *testing.T) {
	sys, _, _, tracker := newFixture(t)
	Register(sys.ToolRegistry, tracker)

	var spawner tools.Tool
	for _, tool := range sys.ToolRegistry.All() {
		if tool.GetName() == subagent.ToolName {
			spawner = tool
		}
	}
	if spawner == nil {
		t.Fatal("the spawning tool is not offered to the model")
	}

	params := spawner.GetSchema().Properties()
	for _, name := range []string{"task", "label", "tools", "model"} {
		if _, ok := params[name]; !ok {
			t.Errorf("the schema is missing %s", name)
		}
	}
	for _, name := range []string{"background", "source", "read_only", "review", "session", "task_id"} {
		if _, ok := params[name]; ok {
			t.Errorf("the schema still offers %s, which this host does not have", name)
		}
	}
}
