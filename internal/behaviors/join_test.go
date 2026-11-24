package behaviors

import (
	"testing"

	"github.com/alexschlessinger/pollytool/messages"
	"github.com/lrstanley/girc"

	"pkdindustries/soulshack/internal/core"
	mocktest "pkdindustries/soulshack/internal/testing"
)

func TestJoinBehavior_SkipsGreetingWhenResumingHistory(t *testing.T) {
	sys := mocktest.NewMockSystem(t)
	model := &mocktest.MockLLM{Responses: []string{"hey everyone!"}}
	sys.LLM = model

	joinEvent := &girc.Event{
		Command: girc.JOIN,
		Source:  &girc.Source{Name: "testbot"},
		Params:  []string{"#test"},
	}
	b := &JoinBehavior{BotNick: "testbot"}

	// Fresh conversation greets normally.
	freshCtx := mocktest.NewMockContext().WithSystem(sys)
	b.Execute(freshCtx, joinEvent)
	if freshCtx.ReplyCount() != 1 || freshCtx.LastReply() != "hey everyone!" {
		t.Fatalf("fresh join replies = %q, want greeting", freshCtx.AllReplies())
	}

	// Rejoining a conversation that already has persisted history does not inject another greeting.
	model.LastRequest = nil
	resumedCtx := mocktest.NewMockContext().WithSystem(sys)
	core.WithConversation(resumedCtx, "seed", func(turn *core.Turn) {
		if turn.Conversation.Len() == 0 {
			_ = turn.Conversation.Append([]messages.ChatMessage{{Role: messages.MessageRoleUser, Content: "prior chat"}})
		}
	}, nil)

	b.Execute(resumedCtx, joinEvent)
	if resumedCtx.ReplyCount() != 0 {
		t.Fatalf("resumed join unexpectedly greeted: %q", resumedCtx.AllReplies())
	}
	if model.LastRequest != nil {
		t.Fatal("resumed join unexpectedly called the LLM")
	}
}
