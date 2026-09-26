package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/alexschlessinger/pollytool/artifacts"
	"github.com/alexschlessinger/pollytool/llm"
	"github.com/alexschlessinger/pollytool/messages"
	"github.com/alexschlessinger/pollytool/tools"

	"pkdindustries/soulshack/internal/config"
)

// A request under maxcontext carries some things whole: the tool schemas, the
// system prompt, polly's omission marker, and the part of the turn's tool loop
// polly cannot shrink. Two things vary and are bounded to a share of the
// budget each: the incoming message (boundMessage) and the newest page a tool
// returned, which polly keeps verbatim until the model has read it and bounds
// to the same share. MinContext is the budget at which all of it fits, so a
// turn under at least that much never fails for want of room.
const (
	// budgetShareDivisor gives the incoming message a quarter of the budget,
	// the share polly gives a tool's page.
	budgetShareDivisor = 4
	// loopReserve holds what a turn's tool loop adds that polly cannot
	// shrink: the model's own calls and the receipts standing in for the
	// results it stored.
	loopReserve = 1024
	// markerReserve holds polly's omission marker.
	markerReserve = 100
)

// MinContext is the smallest maxcontext a turn always fits in with this
// configuration's prompts and these tools: the parts carried whole take half
// the budget at most, leaving a quarter each for the message and for a page.
func MinContext(cfg *config.Configuration, registry *tools.ToolRegistry) int {
	prompt := promptTokens(cfg.Bot.Prompt)
	if cfg.Bot.Subagents {
		prompt = max(prompt, promptTokens(fmt.Sprintf(childPrompt, cfg.Server.Nick)))
	}
	return 2 * (toolSchemaTokens(registry) + prompt + markerReserve + loopReserve)
}

// CheckContextBudget refuses a maxcontext below MinContext. Unlimited (zero)
// always fits.
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

func promptTokens(prompt string) int {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return 0
	}
	return messages.EstimateMessageTokens(messages.ChatMessage{Role: messages.MessageRoleSystem, Content: prompt})
}

// schemaOnly stands in for an artifact store when all that is wanted is the
// tools an agent would offer: an agent with a store installs polly's artifact
// readers. Nothing reads or writes it.
type schemaOnly struct{ artifacts.Store }

// toolSchemaTokens estimates what the tool schemas an agent over registry
// offers cost a request, polly's built-ins included, the way polly estimates
// them when it takes them out of the budget.
func toolSchemaTokens(registry *tools.ToolRegistry) int {
	agent := llm.NewAgent(nil, registry, llm.AgentConfig{ArtifactStore: schemaOnly{}})
	defer agent.Close()
	total := 0
	for _, tool := range agent.ToolRegistry().All() {
		schema := tool.GetSchema()
		if schema == nil {
			continue
		}
		total += 8
		if raw, err := json.Marshal(schema.Raw); err == nil {
			total += messages.EstimatedJSONTokens(string(raw))
		}
	}
	return total
}

// boundMessage keeps an incoming message within its share of the budget. A
// longer one, such as a child's report, is stored whole as an artifact and
// stands in the request as its head and tail and a receipt, which the model
// opens with read_artifact: polly authorizes an artifact any transcript
// message refers to. Without a budget or a store the message is left as it is.
func boundMessage(ctx context.Context, store artifacts.Store, msg messages.ChatMessage, budget int) (messages.ChatMessage, error) {
	share := budget / budgetShareDivisor
	if budget <= 0 || store == nil || messages.EstimateMessageTokens(msg) <= share {
		return msg, nil
	}
	ref, err := store.Put(ctx, artifacts.Blob{Kind: artifacts.KindText, MIMEType: "text/plain", Name: "message", Data: []byte(msg.Content)})
	if err != nil {
		return msg, fmt.Errorf("store long message: %w", err)
	}
	receipt := fmt.Sprintf("[message stored as artifact %s; %d bytes; %d lines. Use read_artifact to read it in full.]", ref.ID, ref.Bytes, ref.Lines)
	const gap = "\n[...]\n"
	// Four bytes a token, as polly estimates text, less room for the message
	// envelope.
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
