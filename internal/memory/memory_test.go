package memory_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/alexschlessinger/pollytool/messages"

	"pkdindustries/soulshack/internal/memory"
)

func user(content string) messages.ChatMessage {
	return messages.ChatMessage{Role: messages.MessageRoleUser, Content: content}
}

func assistant(content string) messages.ChatMessage {
	return messages.ChatMessage{Role: messages.MessageRoleAssistant, Content: content}
}

func TestConversationsAreIndependent(t *testing.T) {
	mem := memory.New(memory.Config{})
	t.Cleanup(mem.Close)

	mem.With(context.Background(), "#a", func(c *memory.Conversation) { c.Append([]messages.ChatMessage{user("in a")}) })

	mem.With(context.Background(), "#b", func(c *memory.Conversation) {
		if got := c.Messages(); len(got) != 0 {
			t.Fatalf("conversation #b saw %d messages from another key", len(got))
		}
		c.Append([]messages.ChatMessage{user("in b")})
	})

	mem.With(context.Background(), "#a", func(c *memory.Conversation) {
		got := c.Messages()
		if len(got) != 1 || got[0].Content != "in a" {
			t.Fatalf("#a history = %+v", got)
		}
	})
}

func TestTurnsOnOneConversationSerialize(t *testing.T) {
	mem := memory.New(memory.Config{})
	t.Cleanup(mem.Close)

	running := make(chan struct{})
	release := make(chan struct{})
	go func() {
		mem.With(context.Background(), "#chan", func(*memory.Conversation) {
			close(running)
			<-release
		})
	}()
	<-running

	// A second turn cannot start while the first holds the conversation, and
	// gives up when its own context ends.
	waitCtx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	ran := false
	if mem.With(waitCtx, "#chan", func(*memory.Conversation) { ran = true }) {
		t.Fatal("second turn ran while the first was still holding the conversation")
	}
	if ran {
		t.Fatal("second turn ran after giving up")
	}

	// Another key is unaffected.
	if !mem.With(context.Background(), "#other", func(*memory.Conversation) {}) {
		t.Fatal("an unrelated conversation was blocked")
	}

	close(release)
}

func TestAppendTrimsToBudget(t *testing.T) {
	var mem *memory.Memory
	// Smallest workable budget: TrimHistory keeps whole messages that fit.
	mem = memory.New(memory.Config{Budget: 12})
	t.Cleanup(mem.Close)

	mem.With(context.Background(), "#chan", func(c *memory.Conversation) {
		for i := 0; i < 40; i++ {
			c.Append([]messages.ChatMessage{user(strings.Repeat("x", 40)), assistant(strings.Repeat("y", 40))})
		}
		got := c.Messages()
		if len(got) >= 80 {
			t.Fatalf("transcript was not trimmed: %d messages retained", len(got))
		}
		if len(got) == 0 || got[len(got)-1].Content != strings.Repeat("y", 40) {
			t.Fatalf("trimming dropped the newest message: %+v", got[len(got)-1:])
		}
	})
}

func TestBudgetChangesApplyToExistingConversations(t *testing.T) {
	mem := memory.New(memory.Config{})
	t.Cleanup(mem.Close)

	mem.With(context.Background(), "#chan", func(c *memory.Conversation) {
		c.Append([]messages.ChatMessage{user(strings.Repeat("x", 400)), assistant(strings.Repeat("y", 400))})
	})
	mem.SetBudget(12)
	mem.With(context.Background(), "#chan", func(c *memory.Conversation) {
		// The newest exchange survives; everything older is trimmed away.
		c.Append([]messages.ChatMessage{user("new question")})
		got := c.Messages()
		if len(got) != 1 || got[0].Role != messages.MessageRoleUser || got[0].Content != "new question" {
			t.Fatalf("existing conversation was not trimmed to the new budget: %d messages", len(got))
		}
	})
}

func TestIdleConversationIsForgotten(t *testing.T) {
	now := time.Now()
	mem := memory.New(memory.Config{
		TTL: time.Minute,
		Now: func() time.Time { return now },
	})
	t.Cleanup(mem.Close)

	mem.With(context.Background(), "#chan", func(c *memory.Conversation) {
		c.Append([]messages.ChatMessage{user("old exchange")})
		c.SetUsage(memory.ContextUsage{EstimatedTokens: 10, Budget: 100})
	})

	// Still within the TTL: the transcript stays.
	mem.With(context.Background(), "#chan", func(c *memory.Conversation) {
		if len(c.Messages()) != 1 {
			t.Fatal("transcript expired before the TTL passed")
		}
	})

	now = now.Add(2 * time.Minute)
	mem.Sweep()
	mem.With(context.Background(), "#chan", func(c *memory.Conversation) {
		if got := c.Messages(); len(got) != 0 {
			t.Fatalf("idle transcript survived the sweeper: %+v", got)
		}
		if _, ok := c.Usage(); ok {
			t.Fatal("idle conversation kept its usage record")
		}
	})

	// A conversation that has gone idle is also restarted on its next turn,
	// without waiting for the sweeper.
	now = now.Add(2 * time.Minute)
	mem.With(context.Background(), "#chan", func(c *memory.Conversation) {
		c.Append([]messages.ChatMessage{user("fresh start")})
	})
	now = now.Add(2 * time.Minute)
	mem.With(context.Background(), "#chan", func(c *memory.Conversation) {
		if got := c.Messages(); len(got) != 0 {
			t.Fatalf("idle detection did not reset the transcript: %+v", got)
		}
	})
}

func TestBudgetZeroKeepsEverything(t *testing.T) {
	mem := memory.New(memory.Config{})
	t.Cleanup(mem.Close)

	mem.With(context.Background(), "#chan", func(c *memory.Conversation) {
		for i := 0; i < 40; i++ {
			c.Append([]messages.ChatMessage{user(strings.Repeat("x", 40)), assistant("ok")})
		}
		if got := len(c.Messages()); got != 80 {
			t.Fatalf("unlimited budget kept %d of 80 messages", got)
		}
	})
}

func TestTTLZeroNeverExpires(t *testing.T) {
	now := time.Now()
	mem := memory.New(memory.Config{TTL: 0, Now: func() time.Time { return now }})
	t.Cleanup(mem.Close)

	mem.With(context.Background(), "#chan", func(c *memory.Conversation) {
		c.Append([]messages.ChatMessage{user("keep me")})
	})
	now = now.Add(1000 * time.Hour)
	mem.Sweep()
	mem.With(context.Background(), "#chan", func(c *memory.Conversation) {
		if len(c.Messages()) != 1 {
			t.Fatal("conversation expired with TTL disabled")
		}
	})
}

func TestDetachedTurnDoesNotTouchTheConversation(t *testing.T) {
	mem := memory.New(memory.Config{})
	t.Cleanup(mem.Close)

	mem.With(context.Background(), "#chan", func(c *memory.Conversation) {
		c.Append([]messages.ChatMessage{user("channel history")})
	})

	mem.WithDetached(context.Background(), "#chan", func(c *memory.Conversation) {
		if got := c.Messages(); len(got) != 0 {
			t.Fatalf("detached turn saw channel history: %+v", got)
		}
		c.Append([]messages.ChatMessage{user("silent observation")})
	})

	mem.With(context.Background(), "#chan", func(c *memory.Conversation) {
		got := c.Messages()
		if len(got) != 1 || got[0].Content != "channel history" {
			t.Fatalf("detached turn polluted the conversation: %+v", got)
		}
	})
	if keys := mem.Keys(); len(keys) != 1 || keys[0] != "#chan" {
		t.Fatalf("detached turn registered a conversation: %v", keys)
	}
}

func TestUsageRoundTrip(t *testing.T) {
	mem := memory.New(memory.Config{})
	t.Cleanup(mem.Close)

	mem.With(context.Background(), "#chan", func(c *memory.Conversation) {
		if _, ok := c.Usage(); ok {
			t.Fatal("a new conversation reported usage")
		}
		c.SetUsage(memory.ContextUsage{EstimatedTokens: 800, Budget: 1000, OmittedExchanges: 3})
	})

	mem.With(context.Background(), "#chan", func(c *memory.Conversation) {
		got, ok := c.Usage()
		if !ok || got != (memory.ContextUsage{EstimatedTokens: 800, Budget: 1000, OmittedExchanges: 3}) {
			t.Fatalf("usage = %+v, found=%v", got, ok)
		}
		c.Clear()
		if _, ok := c.Usage(); ok {
			t.Fatal("clear kept the usage record")
		}
		if len(c.Messages()) != 0 {
			t.Fatal("clear kept the transcript")
		}
	})
}

func TestCancelledContextDoesNotCreateATurn(t *testing.T) {
	mem := memory.New(memory.Config{})
	t.Cleanup(mem.Close)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if mem.With(ctx, "#chan", func(*memory.Conversation) { t.Fatal("cancelled turn ran") }) {
		t.Fatal("cancelled turn reported success")
	}
}
