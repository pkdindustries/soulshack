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

func launch(t *testing.T, tracker *Tracker, ctx core.ChatContextInterface, req subagent.Request) subagent.Result {
	t.Helper()
	result, err := spawn(ctx, tracker, req)
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	if !result.Started {
		t.Fatal("spawn did not report the child as started")
	}
	return result
}

func settled(t *testing.T, result subagent.Result) {
	t.Helper()
	select {
	case <-result.Done:
	case <-time.After(5 * time.Second):
		t.Fatal("child never settled")
	}
}

// seed leaves the history a spawning turn would have.
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

func TestDelegationIsAnnouncedWhateverTheToolActionSettingIs(t *testing.T) {
	for _, show := range []bool{true, false} {
		sys, ctx, model, tracker := newFixture(t)
		mocktest.SetConfig(t, sys, func(c *config.Configuration) { c.Bot.ShowToolActions = show })
		model.Subagent = func(context.Context, core.SubagentSpec) (core.SubagentResult, error) {
			return core.SubagentResult{Text: "done"}, nil
		}

		settled(t, launch(t, tracker, ctx, subagent.Request{Task: "find out", Label: "bird lookup"}))

		actions := ctx.AllActions()
		if len(actions) != 1 || actions[0] != "delegating: bird lookup" {
			t.Fatalf("showtoolactions=%v: expected the delegation announced, got %q", show, actions)
		}
	}
}

func TestSpawnReturnsWhileTheChildIsStillWorking(t *testing.T) {
	_, ctx, model, tracker := newFixture(t)

	working := make(chan struct{})
	release := make(chan struct{})
	model.Subagent = func(context.Context, core.SubagentSpec) (core.SubagentResult, error) {
		close(working)
		<-release
		return core.SubagentResult{Text: "found it"}, nil
	}

	result := launch(t, tracker, ctx, subagent.Request{Task: "find out", Label: "the lookup"})
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

func TestReportIsDeliveredAsTheBotsOwnTurn(t *testing.T) {
	_, ctx, model, tracker := newFixture(t)
	seed(t, ctx)
	model.Subagent = func(context.Context, core.SubagentSpec) (core.SubagentResult, error) {
		return core.SubagentResult{Text: "the thing is a kind of bird"}, nil
	}

	settled(t, launch(t, tracker, ctx, subagent.Request{Task: "find out", Label: "bird lookup"}))

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

// Otherwise a report could keep spawning children with no one in the channel asking.
func TestDeliveryTurnCannotSpawnAgain(t *testing.T) {
	sys, ctx, model, tracker := newFixture(t)
	Register(sys.ToolRegistry, tracker)
	seed(t, ctx)
	model.Subagent = func(context.Context, core.SubagentSpec) (core.SubagentResult, error) {
		return core.SubagentResult{Text: "done"}, nil
	}

	settled(t, launch(t, tracker, ctx, subagent.Request{Task: "find out", Label: "lookup"}))

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

func TestForgottenConversationGetsTheReportVerbatim(t *testing.T) {
	_, ctx, model, tracker := newFixture(t)
	model.Subagent = func(context.Context, core.SubagentSpec) (core.SubagentResult, error) {
		return core.SubagentResult{Text: "it was a bird all along"}, nil
	}

	settled(t, launch(t, tracker, ctx, subagent.Request{Task: "find out", Label: "bird lookup"}))

	if replies := ctx.AllReplies(); len(replies) != 1 || replies[0] != "[bird lookup] it was a bird all along" {
		t.Fatalf("expected the child's own words, got %q", replies)
	}
	if model.LastRequest != nil {
		t.Fatal("a forgotten conversation still ran a completion")
	}
}

func TestFailedChildReportsAndReleasesItsSlot(t *testing.T) {
	_, ctx, model, tracker := newFixture(t)
	model.Subagent = func(context.Context, core.SubagentSpec) (core.SubagentResult, error) {
		return core.SubagentResult{}, context.DeadlineExceeded
	}

	settled(t, launch(t, tracker, ctx, subagent.Request{Task: "find out", Label: "doomed lookup"}))

	if actions := ctx.AllActions(); len(actions) != 2 || !strings.Contains(actions[1], "doomed lookup failed") {
		t.Fatalf("expected the delegation and then the failure, got %q", actions)
	}
	if running := tracker.List(); len(running) != 0 {
		t.Fatalf("a failed child is still tracked: %+v", running)
	}
}

func TestCancelledChildIsQuiet(t *testing.T) {
	_, ctx, model, tracker := newFixture(t)
	bot, shutdown := context.WithCancel(context.Background())
	ctx.Parent = bot
	model.Subagent = func(ctx context.Context, _ core.SubagentSpec) (core.SubagentResult, error) {
		<-ctx.Done()
		return core.SubagentResult{}, ctx.Err()
	}

	result := launch(t, tracker, ctx, subagent.Request{Task: "find out", Label: "interrupted"})
	shutdown()
	settled(t, result)

	if replies, actions := ctx.AllReplies(), ctx.AllActions(); len(replies) != 0 || len(actions) != 1 {
		t.Fatalf("a cancelled child spoke past its delegation: replies %q, actions %q", replies, actions)
	}
}

// Timeout and shutdown end the same context but must be told apart.
func TestTimedOutChildSaysSo(t *testing.T) {
	sys, ctx, model, tracker := newFixture(t)
	mocktest.SetConfig(t, sys, func(c *config.Configuration) {
		c.Bot.SubagentTimeout = 50 * time.Millisecond
	})
	model.Subagent = func(ctx context.Context, _ core.SubagentSpec) (core.SubagentResult, error) {
		<-ctx.Done()
		return core.SubagentResult{}, ctx.Err()
	}

	settled(t, launch(t, tracker, ctx, subagent.Request{Task: "find out", Label: "slow lookup"}))

	if actions := ctx.AllActions(); len(actions) != 2 || !strings.Contains(actions[1], "slow lookup gave up") {
		t.Fatalf("expected the channel to hear that the agent ran out of time, got %q", actions)
	}
}

// Delivery must not run on the child's leftover budget.
func TestReportSurvivesAChildThatUsedItsWholeBudget(t *testing.T) {
	sys, ctx, model, tracker := newFixture(t)
	seed(t, ctx)
	mocktest.SetConfig(t, sys, func(c *config.Configuration) {
		c.Bot.SubagentTimeout = 60 * time.Millisecond
	})
	model.Subagent = func(ctx context.Context, _ core.SubagentSpec) (core.SubagentResult, error) {
		time.Sleep(50 * time.Millisecond)
		return core.SubagentResult{Text: "just made it"}, nil
	}
	// Longer than the child had left.
	model.Delay = 50 * time.Millisecond

	settled(t, launch(t, tracker, ctx, subagent.Request{Task: "find out", Label: "slow lookup"}))

	if replies := ctx.AllReplies(); len(replies) != 1 || replies[0] != "right, here is what it found" {
		t.Fatalf("the report was dropped for arriving near the child's deadline: %q", replies)
	}
}

func TestSilentRelayFallsBackToTheChildsWords(t *testing.T) {
	_, ctx, model, tracker := newFixture(t)
	seed(t, ctx)
	model.Responses = nil // the completion produced nothing
	model.Subagent = func(context.Context, core.SubagentSpec) (core.SubagentResult, error) {
		return core.SubagentResult{Text: "it was a bird"}, nil
	}

	settled(t, launch(t, tracker, ctx, subagent.Request{Task: "find out", Label: "bird lookup"}))

	if replies := ctx.AllReplies(); len(replies) != 1 || replies[0] != "[bird lookup] it was a bird" {
		t.Fatalf("expected the child's own words after a silent relay, got %q", replies)
	}
}

func TestCapsRefuseRatherThanWait(t *testing.T) {
	_, ctx, model, tracker := newFixture(t)
	release := make(chan struct{})
	model.Subagent = func(context.Context, core.SubagentSpec) (core.SubagentResult, error) {
		<-release
		return core.SubagentResult{Text: "done"}, nil
	}

	first := launch(t, tracker, ctx, subagent.Request{Task: "one", Label: "one"})
	second := launch(t, tracker, ctx, subagent.Request{Task: "two", Label: "two"})

	if _, err := spawn(ctx, tracker, subagent.Request{Task: "three", Label: "three"}); err == nil {
		t.Fatal("the per-conversation cap admitted a third child")
	}

	close(release)
	settled(t, first)
	settled(t, second)

	if _, err := spawn(ctx, tracker, subagent.Request{Task: "three", Label: "three"}); err != nil {
		t.Fatalf("a freed slot was not reusable: %v", err)
	}
}

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
	for _, name := range []string{"task", "label", "tools"} {
		if _, ok := params[name]; !ok {
			t.Errorf("the schema is missing %s", name)
		}
	}
	for _, name := range []string{"model", "background", "source", "read_only", "review", "session", "task_id"} {
		if _, ok := params[name]; ok {
			t.Errorf("the schema still offers %s, which this host does not allow", name)
		}
	}
}
