package subagents

import (
	"strings"
	"testing"
	"time"

	"github.com/alexschlessinger/pollytool/subagent"
	"github.com/alexschlessinger/pollytool/tools"

	"pkdindustries/soulshack/internal/core"
)

// running tracks a fake child without starting one.
func running(t *testing.T, tracker *Tracker, label, conversation, source string, age time.Duration) {
	t.Helper()
	if _, err := tracker.admit(core.AgentInfo{
		Label:        label,
		Conversation: conversation,
		Source:       source,
		Started:      time.Now().Add(-age),
	}, 0, 0); err != nil {
		t.Fatal(err)
	}
}

func TestListToolAnswersWhatIsStillWorking(t *testing.T) {
	_, ctx, _, tracker := newFixture(t)
	running(t, tracker, "bird lookup", "#test", "alice", 90*time.Second)

	result, err := newListTool(tracker).Execute(ctx, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"1 working", "bird lookup", "alice", "1m30s"} {
		if !strings.Contains(result, want) {
			t.Errorf("result %q is missing %q", result, want)
		}
	}
}

func TestListToolSaysWhenNothingIsWorking(t *testing.T) {
	_, ctx, _, tracker := newFixture(t)

	result, err := newListTool(tracker).Execute(ctx, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "No agents are working") {
		t.Fatalf("unexpected result %q", result)
	}
}

// Other conversations' children are counted, not described.
func TestListToolKeepsOtherConversationsOut(t *testing.T) {
	_, ctx, _, tracker := newFixture(t)
	running(t, tracker, "secret errand", "bob", "bob", time.Second)
	running(t, tracker, "mine", "#test", "alice", time.Second)

	result, err := newListTool(tracker).Execute(ctx, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "mine") {
		t.Fatalf("this conversation's own agent is missing from %q", result)
	}
	if !strings.Contains(result, "1 agent is working for another conversation") {
		t.Fatalf("result %q does not account for the other conversation", result)
	}
	for _, leak := range []string{"secret errand", "bob"} {
		if strings.Contains(result, leak) {
			t.Errorf("result %q carries %q out of the conversation it belongs to", result, leak)
		}
	}
}

func TestChildrenDoNotInheritTheRoster(t *testing.T) {
	registry := tools.NewToolRegistry([]tools.Tool{}, tools.WithUnsafeNoSandbox())
	defer registry.Close()
	Register(registry, NewTracker())

	child := subagent.ChildRegistry(registry, nil)
	defer child.Close()

	for _, tool := range child.All() {
		if tool.GetName() == listToolName {
			t.Fatal("a child inherited the agent roster")
		}
	}
}

func TestListToolIsOffered(t *testing.T) {
	registry := tools.NewToolRegistry([]tools.Tool{}, tools.WithUnsafeNoSandbox())
	defer registry.Close()
	Register(registry, NewTracker())

	offered := false
	for _, tool := range registry.All() {
		if tool.GetName() == listToolName {
			offered = true
			if len(tool.GetSchema().Required()) != 0 {
				t.Errorf("the roster asks the model for arguments: %v", tool.GetSchema().Required())
			}
		}
	}
	if !offered {
		t.Fatal("the roster is not offered to the model")
	}
}
