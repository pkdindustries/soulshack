package memory_test

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alexschlessinger/pollytool/artifacts"
	"github.com/alexschlessinger/pollytool/messages"

	"pkdindustries/soulshack/internal/memory"
)

func user(content string) messages.ChatMessage {
	return messages.ChatMessage{Role: messages.MessageRoleUser, Content: content}
}

func assistant(content string) messages.ChatMessage {
	return messages.ChatMessage{Role: messages.MessageRoleAssistant, Content: content}
}

func newMemory(t *testing.T, cfg memory.Config) *memory.Memory {
	t.Helper()
	mem, err := memory.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mem.Close)
	return mem
}

// with runs one turn and fails the test if it could not start.
func with(t *testing.T, mem *memory.Memory, key string, fn func(*memory.Conversation)) {
	t.Helper()
	if err := mem.With(context.Background(), key, fn); err != nil {
		t.Fatalf("turn on %s did not run: %v", key, err)
	}
}

func appendOrFail(t *testing.T, c *memory.Conversation, msgs ...messages.ChatMessage) {
	t.Helper()
	if err := c.Append(msgs); err != nil {
		t.Fatal(err)
	}
}

func TestConversationsAreIndependent(t *testing.T) {
	mem := newMemory(t, memory.Config{})

	with(t, mem, "#a", func(c *memory.Conversation) { appendOrFail(t, c, user("in a")) })

	with(t, mem, "#b", func(c *memory.Conversation) {
		if got := c.Messages(); len(got) != 0 {
			t.Fatalf("conversation #b saw %d messages from another key", len(got))
		}
		appendOrFail(t, c, user("in b"))
	})

	with(t, mem, "#a", func(c *memory.Conversation) {
		got := c.Messages()
		if len(got) != 1 || got[0].Content != "in a" {
			t.Fatalf("#a history = %+v", got)
		}
	})
}

// Nicks may contain characters a session name cannot. Each key still gets a
// conversation of its own, including one spelled like another's escape.
func TestKeysWithReservedCharactersStayDistinct(t *testing.T) {
	mem := newMemory(t, memory.Config{})
	keys := []string{"alice|", "alice%7C", `bob\`, "alice_", ".dot."}

	for _, key := range keys {
		with(t, mem, key, func(c *memory.Conversation) { appendOrFail(t, c, user(key)) })
	}
	for _, key := range keys {
		with(t, mem, key, func(c *memory.Conversation) {
			got := c.Messages()
			if len(got) != 1 || got[0].Content != key {
				t.Fatalf("%q history = %+v", key, got)
			}
		})
	}
}

func TestTurnsOnOneConversationSerialize(t *testing.T) {
	mem := newMemory(t, memory.Config{})

	running := make(chan struct{})
	release := make(chan struct{})
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		_ = mem.With(context.Background(), "#chan", func(*memory.Conversation) {
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
	if err := mem.With(waitCtx, "#chan", func(*memory.Conversation) { ran = true }); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("second turn while the first was still holding the conversation: %v", err)
	}
	if ran {
		t.Fatal("second turn ran after giving up")
	}

	// Another key is unaffected.
	with(t, mem, "#other", func(*memory.Conversation) {})

	close(release)
	<-finished
}

// The budget bounds what a request sends, which polly enforces; the stored
// transcript is kept whole so the model can read back what was left out.
func TestBudgetDoesNotTrimTheTranscript(t *testing.T) {
	mem := newMemory(t, memory.Config{Budget: 12})

	with(t, mem, "#chan", func(c *memory.Conversation) {
		for range 40 {
			appendOrFail(t, c, user(strings.Repeat("x", 40)), assistant("ok"))
		}
	})
	with(t, mem, "#chan", func(c *memory.Conversation) {
		if got := c.Len(); got != 80 {
			t.Fatalf("transcript kept %d of 80 messages", got)
		}
	})
}

func TestConversationsSurviveARestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.db")

	first, err := memory.New(memory.Config{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	with(t, first, "#chan", func(c *memory.Conversation) {
		appendOrFail(t, c, user("before the restart"), assistant("noted"))
	})
	first.Close()

	second := newMemory(t, memory.Config{Path: path})
	with(t, second, "#chan", func(c *memory.Conversation) {
		got := c.Messages()
		if len(got) != 2 || got[0].Content != "before the restart" || got[1].Content != "noted" {
			t.Fatalf("stored transcript after restart = %+v", got)
		}
	})
}

func TestIdleConversationIsForgotten(t *testing.T) {
	now := time.Now()
	mem := newMemory(t, memory.Config{
		TTL: time.Minute,
		Now: func() time.Time { return now },
	})

	with(t, mem, "#chan", func(c *memory.Conversation) {
		appendOrFail(t, c, user("old exchange"))
		c.SetUsage(memory.ContextUsage{EstimatedTokens: 10, Budget: 100})
	})

	// Still within the TTL: the transcript stays.
	with(t, mem, "#chan", func(c *memory.Conversation) {
		if c.Len() != 1 {
			t.Fatal("transcript expired before the TTL passed")
		}
	})

	now = now.Add(2 * time.Minute)
	with(t, mem, "#chan", func(c *memory.Conversation) {
		if got := c.Messages(); len(got) != 0 {
			t.Fatalf("idle transcript survived: %+v", got)
		}
		if _, ok := c.Usage(); ok {
			t.Fatal("idle conversation kept its usage record")
		}
	})
}

// A TTL shortened while a conversation exists applies to it on its next turn,
// even though the store judged it by the TTL it was given before.
func TestShortenedTTLAppliesToStoredConversations(t *testing.T) {
	mem := newMemory(t, memory.Config{TTL: time.Hour})

	with(t, mem, "#chan", func(c *memory.Conversation) { appendOrFail(t, c, user("expired")) })
	mem.SetTTL(time.Nanosecond)
	time.Sleep(time.Millisecond)
	with(t, mem, "#chan", func(c *memory.Conversation) {
		if c.Len() != 0 {
			t.Fatal("conversation outlived the shortened TTL")
		}
	})
}

func TestTTLZeroNeverExpires(t *testing.T) {
	now := time.Now()
	mem := newMemory(t, memory.Config{TTL: 0, Now: func() time.Time { return now }})

	with(t, mem, "#chan", func(c *memory.Conversation) { appendOrFail(t, c, user("keep me")) })
	now = now.Add(1000 * time.Hour)
	with(t, mem, "#chan", func(c *memory.Conversation) {
		if c.Len() != 1 {
			t.Fatal("conversation expired with TTL disabled")
		}
	})
}

func TestDetachedTurnDoesNotTouchTheConversation(t *testing.T) {
	mem := newMemory(t, memory.Config{})

	with(t, mem, "#chan", func(c *memory.Conversation) { appendOrFail(t, c, user("channel history")) })

	var ref artifacts.Ref
	if err := mem.WithDetached(context.Background(), "#chan", func(c *memory.Conversation) {
		if got := c.Messages(); len(got) != 0 {
			t.Fatalf("detached turn saw channel history: %+v", got)
		}
		// A store of its own, so its tools can keep large output out of
		// its requests.
		var err error
		if ref, err = c.Artifacts().Put(context.Background(), artifacts.Blob{Kind: artifacts.KindText, Data: []byte("fetched page")}); err != nil {
			t.Fatal(err)
		}
		appendOrFail(t, c, user("silent observation"))
	}); err != nil {
		t.Fatal(err)
	}

	with(t, mem, "#chan", func(c *memory.Conversation) {
		got := c.Messages()
		if len(got) != 1 || got[0].Content != "channel history" {
			t.Fatalf("detached turn polluted the conversation: %+v", got)
		}
		if r, err := c.Artifacts().Open(context.Background(), ref.ID); err == nil {
			r.Close()
			t.Fatal("detached turn's artifact reached the conversation")
		}
	})
	if keys := mem.Keys(); len(keys) != 1 || keys[0] != "#chan" {
		t.Fatalf("detached turn registered a conversation: %v", keys)
	}
}

func TestClearForgetsTranscriptArtifactsAndUsage(t *testing.T) {
	mem := newMemory(t, memory.Config{})

	var ref artifacts.Ref
	with(t, mem, "#chan", func(c *memory.Conversation) {
		if _, ok := c.Usage(); ok {
			t.Fatal("a new conversation reported usage")
		}
		var err error
		ref, err = c.Artifacts().Put(context.Background(), artifacts.Blob{Kind: artifacts.KindText, Data: []byte("big tool output")})
		if err != nil {
			t.Fatal(err)
		}
		appendOrFail(t, c, user("question"))
		c.SetUsage(memory.ContextUsage{EstimatedTokens: 800, Budget: 1000, OmittedExchanges: 3})
	})

	with(t, mem, "#chan", func(c *memory.Conversation) {
		got, ok := c.Usage()
		if !ok || got != (memory.ContextUsage{EstimatedTokens: 800, Budget: 1000, OmittedExchanges: 3}) {
			t.Fatalf("usage = %+v, found=%v", got, ok)
		}
		if r, err := c.Artifacts().Open(context.Background(), ref.ID); err != nil {
			t.Fatalf("artifact did not outlive its turn: %v", err)
		} else {
			r.Close()
		}
		if err := c.Clear(); err != nil {
			t.Fatal(err)
		}
		if _, ok := c.Usage(); ok {
			t.Fatal("clear kept the usage record")
		}
		if c.Len() != 0 {
			t.Fatal("clear kept the transcript")
		}
	})

	with(t, mem, "#chan", func(c *memory.Conversation) {
		if c.Len() != 0 {
			t.Fatal("clear did not reach the stored transcript")
		}
		if r, err := c.Artifacts().Open(context.Background(), ref.ID); err == nil {
			r.Close()
			t.Fatal("clear kept the artifacts")
		}
	})
}

func TestCancelledContextDoesNotCreateATurn(t *testing.T) {
	mem := newMemory(t, memory.Config{})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := mem.With(ctx, "#chan", func(*memory.Conversation) { t.Fatal("cancelled turn ran") }); err == nil {
		t.Fatal("cancelled turn reported success")
	}
}
