package llm

import (
	"context"
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

// A request under maxcontext carries some things whole: the tool schemas, the
// system prompt and polly's omission marker, which together are polly's
// context floor, and the part of a turn's tool loop polly cannot shrink. What
// the budget holds beyond that is room, and two things share it: the incoming
// message (boundMessage) and the pages tools return, which polly sizes to
// whatever room the request has left when it runs them. Each has a floor below
// which it stops being useful and a ceiling above which it only costs more.
// MinContext is the budget in which the floors fit, so a turn under at least
// that much never fails for want of room.
const (
	// loopReserve holds what a turn's tool loop adds that polly cannot
	// shrink: the model's own calls and the receipts and stubs standing in
	// for results it has read.
	loopReserve = 1024
	// messageFloorTokens is what an incoming message may always take
	// whole: a couple of IRC lines, so an ordinary message is never cut.
	messageFloorTokens = 256
)

// MinContext is the smallest maxcontext a turn always fits in with this
// configuration's prompts and these tools: polly's floor for them, the loop
// reserve, and a floor-sized message and page.
func MinContext(cfg *config.Configuration, registry *tools.ToolRegistry) int {
	var offered []tools.Tool
	if registry != nil {
		offered = registry.All()
	}
	floor := contextFloor(cfg.Bot.Prompt, offered)
	if cfg.Bot.Subagents {
		floor = max(floor, contextFloor(fmt.Sprintf(childPrompt, cfg.Server.Nick), offered))
	}
	return floor + loopReserve + messageFloorTokens + llm.PageFloorTokens
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

// contextFloor is polly's context floor for a request with this system prompt
// offering these tools, polly's own readers included, as every agent here has
// an artifact store.
func contextFloor(prompt string, offered []tools.Tool) int {
	req := &llm.CompletionRequest{Tools: append(append([]tools.Tool(nil), offered...), builtinTools()...)}
	if prompt = strings.TrimSpace(prompt); prompt != "" {
		req.Messages = []messages.ChatMessage{{Role: messages.MessageRoleSystem, Content: prompt}}
	}
	return llm.ContextFloorTokens(req)
}

// schemaOnly stands in for an artifact store when all that is wanted is the
// tools an agent with one installs. Nothing reads or writes it.
type schemaOnly struct{ artifacts.Store }

// builtinTools are polly's private readers, which an agent with an artifact
// store adds to what it offers.
var builtinTools = sync.OnceValue(func() []tools.Tool {
	agent := llm.NewAgent(nil, nil, llm.AgentConfig{ArtifactStore: schemaOnly{}})
	defer agent.Close()
	return agent.ToolRegistry().All()
})

// messageShare is how much of a request under budget the incoming message may
// take, given polly's floor for the request. It gets half the room, less any
// that would leave no floor-sized page, between its own floor and a ceiling:
// the size at which polly stores tool output rather than sending it, since a
// longer message costs the same on every request without fitting any better.
// Without a budget only the ceiling applies.
func messageShare(budget, floor int) int {
	ceiling := llm.DefaultInlineToolResultTokens
	if budget <= 0 {
		return ceiling
	}
	room := budget - floor - loopReserve
	return min(ceiling, max(messageFloorTokens, min(room/2, room-llm.PageFloorTokens)))
}

// boundMessage keeps an incoming message within share tokens. A longer one,
// such as a child's report, is stored whole as an artifact and stands in the
// request as its head and tail and a receipt, which the model opens with
// read_artifact: polly authorizes an artifact any transcript message refers
// to. Without a store the message is left as it is.
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
