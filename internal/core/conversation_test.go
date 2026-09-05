package core_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/alexschlessinger/pollytool/messages"
	"github.com/alexschlessinger/pollytool/sessions"

	"pkdindustries/soulshack/internal/core"
	mocktest "pkdindustries/soulshack/internal/testing"
)

type observedStore struct {
	sessions.SessionStore
	acquires int
}

func (s *observedStore) Acquire(ctx context.Context, key string, options sessions.AcquireOptions) (sessions.Session, error) {
	s.acquires++
	return s.SessionStore.Acquire(ctx, key, options)
}

func TestConversationQueuesBeforeAcquiringAndReleasesLease(t *testing.T) {
	sys := mocktest.NewMockSystem(t)
	store := &observedStore{SessionStore: sys.Sessions}
	sys.Sessions = store
	ctx := mocktest.NewMockContext().WithSystem(sys)
	var first sessions.Session
	core.WithConversation(ctx, "first", func(turn core.ChatContextInterface) {
		first = turn.GetSession()
		waitCtx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
		defer cancel()
		waiting := mocktest.NewMockContext().WithSystem(sys).WithContext(waitCtx)
		timedOut := false
		core.WithConversation(waiting, "queued", func(core.ChatContextInterface) { t.Fatal("queued turn ran before the first finished") }, func() { timedOut = true })
		if !timedOut || store.acquires != 1 {
			t.Fatalf("queued turn acquired a session: acquires=%d, timedOut=%v", store.acquires, timedOut)
		}
		// Completed replies can still save after their request deadline.
		if err := first.AddMessage(context.WithoutCancel(turn), messages.ChatMessage{Role: messages.MessageRoleAssistant, Content: "saved response"}); err != nil {
			t.Fatal(err)
		}
	}, nil)
	if first == nil || first.Context().Err() == nil {
		t.Fatal("lease was not closed")
	}
	core.WithConversation(ctx, "next", func(turn core.ChatContextInterface) {
		if turn.GetSession() == first {
			t.Fatal("closed lease reused")
		}
		history, err := turn.GetSession().GetHistory(turn)
		if err != nil {
			t.Fatal(err)
		}
		if history[len(history)-1].Content != "saved response" {
			t.Fatalf("saved history missing: %v", history)
		}
	}, nil)
}

func TestConversationCancelAndLeaseLoss(t *testing.T) {
	for _, loseLease := range []bool{false, true} {
		t.Run(map[bool]string{false: "request", true: "lease"}[loseLease], func(t *testing.T) {
			sys := mocktest.NewMockSystem(t)
			parent, cancel := context.WithCancelCause(context.Background())
			defer cancel(nil)
			ctx := mocktest.NewMockContext().WithSystem(sys).WithContext(parent)
			core.WithConversation(ctx, "cancel", func(turn core.ChatContextInterface) {
				cause := errors.New("request ended")
				if loseLease {
					if err := turn.GetSession().Close(); err != nil {
						t.Fatal(err)
					}
				} else {
					cancel(cause)
				}
				select {
				case <-turn.Done():
				case <-time.After(time.Second):
					t.Fatal("turn did not observe cancellation")
				}
				if !loseLease {
					if !errors.Is(context.Cause(turn), cause) {
						t.Fatalf("lost cancellation cause: %v", context.Cause(turn))
					}
					if err := turn.GetSession().AddMessage(context.WithoutCancel(turn), messages.ChatMessage{Role: messages.MessageRoleAssistant, Content: "final"}); err != nil {
						t.Fatalf("deadline prematurely released session: %v", err)
					}
				}
			}, nil)
		})
	}
}

func TestConversationDetachedCleanup(t *testing.T) {
	sys := mocktest.NewMockSystem(t)
	ctx := mocktest.NewMockContext().WithSystem(sys)
	core.WithConversation(ctx, "seed", func(turn core.ChatContextInterface) {
		if err := turn.GetSession().AddMessage(turn, messages.ChatMessage{Role: messages.MessageRoleUser, Content: "named history"}); err != nil {
			t.Fatal(err)
		}
	}, nil)
	for _, canceled := range []bool{false, true} {
		parent, cancel := context.WithCancel(context.Background())
		detached := mocktest.NewMockContext().WithSystem(sys).WithContext(parent)
		core.WithDetachedConversation(detached, "silent", func(turn core.ChatContextInterface) {
			name, err := turn.GetSession().GetName(turn)
			if err != nil || !strings.HasPrefix(name, "__detached_") {
				t.Fatalf("detached name = %q, %v", name, err)
			}
			history, err := turn.GetSession().GetHistory(turn)
			if err != nil {
				t.Fatal(err)
			}
			for _, msg := range history {
				if msg.Content == "named history" {
					t.Fatal("silent turn inherited named history")
				}
			}
			if err := turn.GetSession().AddMessage(turn, messages.ChatMessage{Role: messages.MessageRoleUser, Content: "temporary"}); err != nil {
				t.Fatal(err)
			}
			if canceled {
				cancel()
			}
		}, nil)
		cancel()
		keys, err := sys.Sessions.List(context.Background())
		if err != nil || len(keys) != 1 || keys[0] != ctx.GetLockKey() {
			t.Fatalf("detached session leaked after cancellation=%v: %v, %v", canceled, keys, err)
		}
	}
	core.WithConversation(ctx, "verify", func(turn core.ChatContextInterface) {
		history, err := turn.GetSession().GetHistory(turn)
		if err != nil || history[len(history)-1].Content != "named history" {
			t.Fatalf("named history changed: %v, %v", history, err)
		}
	}, nil)
}

func TestConversationExpiredSessionStartsFresh(t *testing.T) {
	sys := mocktest.NewMockSystem(t)
	ctx := mocktest.NewMockContext().WithSystem(sys)
	core.WithConversation(ctx, "seed", func(turn core.ChatContextInterface) {
		if err := turn.GetSession().AddMessage(turn, messages.ChatMessage{Role: messages.MessageRoleUser, Content: "expired"}); err != nil {
			t.Fatal(err)
		}
		metadata, err := turn.GetSession().GetMetadata(turn)
		if err != nil {
			t.Fatal(err)
		}
		metadata.TTL = time.Nanosecond
		if err := turn.GetSession().SetMetadata(turn, metadata); err != nil {
			t.Fatal(err)
		}
	}, nil)
	core.WithConversation(ctx, "fresh", func(turn core.ChatContextInterface) {
		history, err := turn.GetSession().GetHistory(turn)
		if err != nil {
			t.Fatal(err)
		}
		for _, msg := range history {
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
	core.WithConversation(first, "first", func(a core.ChatContextInterface) {
		deadline, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		second.WithContext(deadline)
		ran := false
		core.WithConversation(second, "second", func(b core.ChatContextInterface) {
			ran = true
			if a.GetSession() == b.GetSession() {
				t.Fatal("independent conversations share a lease")
			}
		}, nil)
		if !ran {
			t.Fatal("another conversation blocked behind the first")
		}
	}, nil)
}
