package llm

import (
	"testing"
	"time"

	"github.com/alexschlessinger/pollytool/messages"
	"pkdindustries/soulshack/internal/memory"
	mocktest "pkdindustries/soulshack/internal/testing"
)

func TestSilentTurnSuppressesCompletionAnnouncements(t *testing.T) {
	sys := mocktest.NewMockSystem(t)
	ctx := mocktest.NewMockContext().WithSystem(sys).WithConversation(sys.Conversation(t, "test"))
	turn := ctx.Turn()
	turn.Silent = true
	cfg := turn.GetConfig()
	cfg.Bot.ShowThinkingAction, cfg.Bot.ShowToolActions = true, true
	handler := newCallbackHandler(turn, turn.NewChunkWriter(make(chan string, 1)), cfg)
	handler.startTime = time.Now().Add(-time.Minute)
	handler.onReasoning("working")
	handler.onToolStart([]messages.ChatMessageToolCall{{Name: "lookup"}})
	checkContextUsage(turn, memory.ContextUsage{Compacted: true})
	checkContextUsage(turn, memory.ContextUsage{EstimatedTokens: 950, Budget: 1000})
	turn.Reply("discarded answer")
	if len(ctx.AllActions()) != 0 || ctx.ReplyCount() != 0 {
		t.Fatalf("silent turn spoke: replies=%q actions=%q", ctx.AllReplies(), ctx.AllActions())
	}
}
