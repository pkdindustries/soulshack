package llm

import (
	"context"
	"strings"
	"testing"

	polly "github.com/alexschlessinger/pollytool/llm"
	"github.com/alexschlessinger/pollytool/messages"
	"github.com/alexschlessinger/pollytool/subagent"
	"github.com/alexschlessinger/pollytool/tools"

	"pkdindustries/soulshack/internal/config"
	"pkdindustries/soulshack/internal/core"
	"pkdindustries/soulshack/internal/irc"
	mocktest "pkdindustries/soulshack/internal/testing"
)

// parentRegistry is the IRC tools plus one ordinary tool a child may keep.
func parentRegistry(t *testing.T) *tools.ToolRegistry {
	t.Helper()
	registry := tools.NewToolRegistry([]tools.Tool{}, tools.WithUnsafeNoSandbox())
	t.Cleanup(func() { registry.Close() })
	irc.RegisterIRCTools(registry)
	registry.Register(&tools.Func{Name: "search", Desc: "search the web"})
	registry.Register(&subagent.Tool{})
	return registry
}

func offered(registry *tools.ToolRegistry) map[string]bool {
	names := make(map[string]bool)
	for _, tool := range registry.All() {
		names[tool.GetName()] = true
	}
	return names
}

// Reading the channel stays allowed.
func TestChildCannotActOnTheChannel(t *testing.T) {
	child, release, err := childRegistry(parentRegistry(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer release()

	// Listed explicitly so shortening the deny list fails the test.
	names := offered(child)
	for _, denied := range []string{
		"irc__op", "irc__kick", "irc__ban", "irc__mode_set",
		"irc__invite", "irc__topic", "irc__action", subagent.ToolName,
	} {
		if names[denied] {
			t.Errorf("a child is offered %s", denied)
		}
	}
	for _, allowed := range []string{"irc__names", "irc__whois", "irc__mode_query", "search"} {
		if !names[allowed] {
			t.Errorf("a child is not offered %s", allowed)
		}
	}
}

func TestBriefCannotReopenDeniedTools(t *testing.T) {
	child, release, err := childRegistry(parentRegistry(t), []string{"irc__*", "search"})
	if err != nil {
		t.Fatal(err)
	}
	defer release()

	names := offered(child)
	if names["irc__kick"] || names["irc__op"] {
		t.Fatalf("a brief naming irc__* reopened the acting tools: %v", names)
	}
	if !names["irc__names"] || !names["search"] {
		t.Fatalf("a brief naming its tools did not get them: %v", names)
	}
}

func TestChildSeesOnlyItsBrief(t *testing.T) {
	sys := mocktest.NewMockSystem(t)
	mocktest.SetConfig(t, sys, func(c *config.Configuration) {
		c.Bot.Prompt = "you are a helpful chatbot in #test"
		c.Bot.SubagentModel = "ollama/small"
	})

	var seen *polly.CompletionRequest
	model := &PollyLLM{client: completionFunc(func(_ context.Context, req *polly.CompletionRequest) messages.ChatMessage {
		seen = req
		return messages.ChatMessage{Role: messages.MessageRoleAssistant, Content: "  the answer  ", StopReason: messages.StopReasonEndTurn}
	})}

	result, err := model.RunSubagent(context.Background(), core.SubagentSpec{
		Task:     "find out about the thing",
		Label:    "lookup",
		Registry: parentRegistry(t),
		Config:   sys.GetConfig(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Text != "the answer" {
		t.Fatalf("expected the child's reply, trimmed, got %q", result.Text)
	}

	if len(seen.Messages) != 2 {
		t.Fatalf("expected a prompt and a brief, got %+v", seen.Messages)
	}
	if seen.Messages[0].Role != messages.MessageRoleSystem || !strings.Contains(seen.Messages[0].Content, "subagent of testbot") {
		t.Fatalf("a child was not told what it is: %q", seen.Messages[0].Content)
	}
	if strings.Contains(seen.Messages[0].Content, "helpful chatbot") {
		t.Fatal("a child was given the bot's own prompt")
	}
	if seen.Messages[1].Role != messages.MessageRoleUser || seen.Messages[1].Content != "find out about the thing" {
		t.Fatalf("unexpected brief: %+v", seen.Messages[1])
	}
	if seen.Model != "ollama/small" {
		t.Fatalf("expected the configured agent model, got %q", seen.Model)
	}
}

// Brief's model, then configured subagent model, then the bot's.
func TestChildModelResolution(t *testing.T) {
	cfg := mocktest.DefaultTestConfig()
	cfg.Model.Model = "anthropic/big"
	spec := core.SubagentSpec{Config: cfg}

	cfg.Bot.SubagentModel = ""
	if model := childModel(spec); model != "" {
		t.Fatalf("expected the bot's own model to stand, got %q", model)
	}
	cfg.Bot.SubagentModel = "ollama/small"
	if model := childModel(spec); model != "ollama/small" {
		t.Fatalf("expected the configured agent model, got %q", model)
	}
	spec.Model = "openai/asked-for"
	if model := childModel(spec); model != "openai/asked-for" {
		t.Fatalf("expected the brief's model, got %q", model)
	}
}

func TestWithheldToolsCannotBeRunAnyway(t *testing.T) {
	registry := parentRegistry(t)

	offered := make([]tools.Tool, 0, len(registry.All()))
	for _, tool := range registry.All() {
		if tool.GetName() != subagent.ToolName {
			offered = append(offered, tool)
		}
	}

	narrowed, release := offeredRegistry(registry, offered)
	defer release()

	if _, ok := narrowed.Get(subagent.ToolName); ok {
		t.Fatal("an agent can still run the tool its turn withheld")
	}
	if _, ok := narrowed.Get("irc__kick"); !ok {
		t.Fatal("narrowing took away a tool the turn did offer")
	}
	if _, ok := registry.Get(subagent.ToolName); !ok {
		t.Fatal("narrowing one turn took the tool away from the bot")
	}
}

func TestUnnarrowedTurnUsesTheRegistryItself(t *testing.T) {
	registry := parentRegistry(t)
	same, release := offeredRegistry(registry, registry.All())
	defer release()
	if same != registry {
		t.Fatal("a turn offering every tool still derived a registry")
	}
}
