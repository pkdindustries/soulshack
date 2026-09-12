package core_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alexschlessinger/pollytool/messages"

	"pkdindustries/soulshack/internal/core"
	mocktest "pkdindustries/soulshack/internal/testing"
)

func TestConversationQueuesTurnsAndKeepsTheTranscript(t *testing.T) {
	sys := mocktest.NewMockSystem(t)
	ctx := mocktest.NewMockContext().WithSystem(sys)

	var conversation *core.Turn
	core.WithConversation(ctx, "first", func(turn *core.Turn) {
		turn.Conversation.Append([]messages.ChatMessage{{Role: messages.MessageRoleAssistant, Content: "saved response"}})
		// A second turn for the same key waits for this one, and gives up
		// rather than running when its own deadline passes first.
		waitCtx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
		defer cancel()
		waiting := mocktest.NewMockContext().WithSystem(sys).WithContext(waitCtx)
		timedOut := false
		core.WithConversation(waiting, "queued", func(*core.Turn) { t.Fatal("queued turn ran before the first finished") }, func() { timedOut = true })
		if !timedOut {
			t.Fatal("queued turn did not report a timeout")
		}
	}, nil)

	core.WithConversation(ctx, "next", func(turn *core.Turn) {
		history := turn.Conversation.Messages()
		if len(history) != 1 || history[0].Content != "saved response" {
			t.Fatalf("transcript from the previous turn is missing: %+v", history)
		}
		conversation = turn
	}, nil)
	if conversation == nil {
		t.Fatal("second turn did not run")
	}
}

func TestConversationCancellationReachesTheTurn(t *testing.T) {
	sys := mocktest.NewMockSystem(t)
	parent, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)
	ctx := mocktest.NewMockContext().WithSystem(sys).WithContext(parent)

	core.WithConversation(ctx, "cancel", func(turn *core.Turn) {
		cause := errors.New("request ended")
		cancel(cause)
		select {
		case <-turn.Done():
		case <-time.After(time.Second):
			t.Fatal("turn did not observe cancellation")
		}
		if !errors.Is(context.Cause(turn), cause) {
			t.Fatalf("lost cancellation cause: %v", context.Cause(turn))
		}
		// A finished reply can still be appended after the request ended:
		// the transcript is process memory, not a leased resource.
		turn.Conversation.Append([]messages.ChatMessage{{Role: messages.MessageRoleAssistant, Content: "final"}})
	}, nil)

	core.WithConversation(ctx, "verify", func(turn *core.Turn) {
		history := turn.Conversation.Messages()
		if len(history) != 1 || history[0].Content != "final" {
			t.Fatalf("finished reply was lost: %+v", history)
		}
	}, nil)
}

func TestDetachedConversationIsDiscarded(t *testing.T) {
	sys := mocktest.NewMockSystem(t)
	ctx := mocktest.NewMockContext().WithSystem(sys)
	core.WithConversation(ctx, "seed", func(turn *core.Turn) {
		turn.Conversation.Append([]messages.ChatMessage{{Role: messages.MessageRoleUser, Content: "named history"}})
	}, nil)

	for _, canceled := range []bool{false, true} {
		parent, cancel := context.WithCancel(context.Background())
		detached := mocktest.NewMockContext().WithSystem(sys).WithContext(parent)
		core.WithDetachedConversation(detached, "silent", func(turn *core.Turn) {
			for _, msg := range turn.Conversation.Messages() {
				if msg.Content == "named history" {
					t.Fatal("detached turn inherited the conversation's history")
				}
			}
			turn.Conversation.Append([]messages.ChatMessage{{Role: messages.MessageRoleUser, Content: "temporary"}})
			if canceled {
				cancel()
			}
		}, nil)
		cancel()

		keys := sys.Memory.Keys()
		if len(keys) != 1 || keys[0] != ctx.GetConversationKey() {
			t.Fatalf("detached turn registered a conversation (canceled=%v): %v", canceled, keys)
		}
	}

	core.WithConversation(ctx, "verify", func(turn *core.Turn) {
		history := turn.Conversation.Messages()
		if len(history) != 1 || history[0].Content != "named history" {
			t.Fatalf("detached turns changed the conversation: %+v", history)
		}
	}, nil)
}

func TestConversationIdleExpiryStartsFresh(t *testing.T) {
	sys := mocktest.NewMockSystem(t)
	ctx := mocktest.NewMockContext().WithSystem(sys)
	core.WithConversation(ctx, "seed", func(turn *core.Turn) {
		turn.Conversation.Append([]messages.ChatMessage{{Role: messages.MessageRoleUser, Content: "expired"}})
	}, nil)

	// Expire by hand rather than sleeping: the TTL clock is the memory's.
	sys.Memory.SetTTL(time.Nanosecond)
	time.Sleep(time.Millisecond)
	sys.Memory.Sweep()

	core.WithConversation(ctx, "fresh", func(turn *core.Turn) {
		for _, msg := range turn.Conversation.Messages() {
			if msg.Content == "expired" {
				t.Fatal("idle-expired transcript was reused")
			}
		}
	}, nil)
}

func TestDifferentConversationsProceedIndependently(t *testing.T) {
	sys := mocktest.NewMockSystem(t)
	first := mocktest.NewMockContext().WithSystem(sys)
	otherConfig := mocktest.DefaultTestConfig()
	otherConfig.Server.Channel = "#other"
	second := mocktest.NewMockContext().WithSystem(sys).WithConfig(otherConfig)

	core.WithConversation(first, "first", func(a *core.Turn) {
		deadline, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		second.WithContext(deadline)
		ran := false
		core.WithConversation(second, "second", func(b *core.Turn) {
			ran = true
			if a.Conversation == b.Conversation {
				t.Fatal("independent conversations share a transcript")
			}
		}, nil)
		if !ran {
			t.Fatal("another conversation blocked behind the first")
		}
	}, nil)
}
