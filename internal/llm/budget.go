package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/alexschlessinger/pollytool/artifacts"
	"github.com/alexschlessinger/pollytool/llm"
	"github.com/alexschlessinger/pollytool/messages"
	"github.com/alexschlessinger/pollytool/tools"

	"pkdindustries/soulshack/internal/config"
)

// messageFloorTokens is what an incoming message may always take whole: a couple of IRC lines.
const messageFloorTokens = 256

// pageFloorTokens is polly's smallest tool page: two previews.
const pageFloorTokens = 1_000

// inlineCeilingTokens matches polly's threshold for storing tool output as an artifact.
const inlineCeilingTokens = 10_000

// MinContext is the smallest maxcontext that fits polly's floor, one message and one tool page.
func MinContext(cfg *config.Configuration, registry *tools.ToolRegistry) int {
	var offered []tools.Tool
	if registry != nil {
		offered = registry.All()
	}
	floor := contextFloor(cfg.Bot.Prompt, offered)
	if cfg.Bot.Subagents {
		floor = max(floor, contextFloor(fmt.Sprintf(childPrompt, cfg.Server.Nick), offered))
	}
	return floor + messageFloorTokens + pageFloorTokens
}

// CheckContextBudget refuses a maxcontext below MinContext. Zero means unlimited.
func CheckContextBudget(cfg *config.Configuration, registry *tools.ToolRegistry) error {
	budget := cfg.Session.MaxContext
	if budget <= 0 {
		return nil
	}
	if least := MinContext(cfg, registry); budget < least {
		return fmt.Errorf("maxcontext %d is below %d, the least a turn always fits in with this prompt and these tools", budget, least)
	}
	return nil
}

// contextFloor mirrors polly's private estimate of prompt plus tool schemas, builtins included.
func contextFloor(prompt string, offered []tools.Tool) int {
	floor := 0
	if prompt = strings.TrimSpace(prompt); prompt != "" {
		floor = messages.EstimateMessageTokens(messages.ChatMessage{Role: messages.MessageRoleSystem, Content: prompt})
	}
	for _, tool := range append(append([]tools.Tool(nil), offered...), builtinTools()...) {
		floor += toolSchemaTokens(tool)
	}
	return floor
}

func toolSchemaTokens(tool tools.Tool) int {
	schema := tool.GetSchema()
	if schema == nil {
		return 0
	}
	tokens := 8
	if raw, err := json.Marshal(schema.Raw); err == nil {
		tokens += messages.EstimatedJSONTokens(string(raw))
	}
	return tokens
}

// schemaOnly is a never-used store that makes an agent install its builtins.
type schemaOnly struct{ artifacts.Store }

// builtinTools are the readers polly adds when an agent has an artifact store.
var builtinTools = sync.OnceValue(func() []tools.Tool {
	agent := llm.NewAgent(nil, nil, llm.AgentConfig{ArtifactStore: schemaOnly{}})
	defer agent.Close()
	return agent.ToolRegistry().All()
})

// messageShare: half the room above floor, leaving a page, clamped to [messageFloorTokens, inlineCeilingTokens].
func messageShare(budget, floor int) int {
	if budget <= 0 {
		return inlineCeilingTokens
	}
	room := budget - floor
	return min(inlineCeilingTokens, max(messageFloorTokens, min(room/2, room-pageFloorTokens)))
}

// boundMessage replaces a message over share with head, tail and an artifact receipt.
func boundMessage(ctx context.Context, store artifacts.Store, msg messages.ChatMessage, share int) (messages.ChatMessage, error) {
	if store == nil || messages.EstimateMessageTokens(msg) <= share {
		return msg, nil
	}
	ref, err := store.Put(ctx, artifacts.Blob{Kind: artifacts.KindText, MIMEType: "text/plain", Name: "message", Data: []byte(msg.Content)})
	if err != nil {
		return msg, fmt.Errorf("store long message: %w", err)
	}
	receipt := fmt.Sprintf("[message stored as artifact %s; %d bytes; %d lines. Use read_artifact to read it in full.]", ref.ID, ref.Bytes, ref.Lines)
	const gap = "\n[...]\n"
	// Polly's 4 bytes/token, less envelope slack.
	room := max(0, share*4-len(receipt)-len(gap)-64)
	head := prefix(msg.Content, room*2/3)
	tail := suffix(msg.Content, room-len(head))

	bounded := msg
	bounded.Content = head + gap + tail + "\n" + receipt
	bounded.Parts = append(append([]messages.ContentPart(nil), msg.Parts...), messages.ContentPart{Type: "artifact", Artifact: &ref})
	return bounded, nil
}

// prefix is at most the first n bytes of s, cut at a rune boundary.
func prefix(s string, n int) string {
	if n >= len(s) {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}

// suffix is at most the last n bytes of s, cut at a rune boundary.
func suffix(s string, n int) string {
	if n >= len(s) {
		return s
	}
	i := len(s) - n
	for i < len(s) && !utf8.RuneStart(s[i]) {
		i++
	}
	return s[i:]
}
