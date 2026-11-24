package memory

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/alexschlessinger/pollytool/messages"
	"github.com/alexschlessinger/pollytool/sessions"
)

func testMemory(t *testing.T, cfg Config) *Memory {
	t.Helper()
	m, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(m.Close)
	return m
}

func storeHistory(t *testing.T, m *Memory, key string) {
	t.Helper()
	if err := m.With(context.Background(), key, func(c *Conversation) {
		if err := c.Append([]messages.ChatMessage{{Role: messages.MessageRoleUser, Content: "keep me"}}); err != nil {
			t.Fatal(err)
		}
	}); err != nil {
		t.Fatal(err)
	}
}

func requireExists(t *testing.T, m *Memory, key string, want bool) {
	t.Helper()
	exists, err := m.store.Exists(context.Background(), sessionName(key))
	if err != nil {
		t.Fatal(err)
	}
	if exists != want {
		t.Fatalf("session %q exists = %t, want %t", key, exists, want)
	}
}

func TestCleanupUsesCurrentTTL(t *testing.T) {
	for _, restart := range []bool{false, true} {
		for _, ttl := range []time.Duration{0, time.Hour} {
			t.Run(fmt.Sprintf("restart=%t/ttl=%s", restart, ttl), func(t *testing.T) {
				// Future clock fakes idleness without racing the background worker.
				cfg := Config{TTL: time.Minute, Now: func() time.Time { return time.Now().Add(10 * time.Minute) }}
				if restart {
					cfg.Path = filepath.Join(t.TempDir(), "sessions.db")
				}
				m := testMemory(t, cfg)
				storeHistory(t, m, "#chan")
				metadata, err := m.store.GetMetadata(context.Background(), "#chan")
				if err != nil {
					t.Fatal(err)
				}
				if metadata.TTL != 0 {
					t.Fatalf("named conversation has a stale storage policy: %s", metadata.TTL)
				}
				if restart {
					m.Close()
					cfg.TTL = ttl
					m = testMemory(t, cfg)
					if len(m.Keys()) != 0 {
						t.Fatal("restarted memory unexpectedly loaded conversation keys")
					}
				} else {
					m.SetTTL(ttl)
				}
				if err := m.expire(context.Background()); err != nil {
					t.Fatal(err)
				}
				requireExists(t, m, "#chan", true)
				// A setting change and a sweep are not activity.
				after, err := m.store.GetMetadata(context.Background(), "#chan")
				if err != nil || !after.LastUsed.Equal(metadata.LastUsed) {
					t.Fatalf("idle timestamp changed: metadata=%+v, err=%v", after, err)
				}
				m.SetTTL(time.Second)
				if err := m.expire(context.Background()); err != nil {
					t.Fatal(err)
				}
				requireExists(t, m, "#chan", false)
			})
		}
	}
}

func TestCleanupKeepsActiveConversations(t *testing.T) {
	m := testMemory(t, Config{TTL: time.Second, Now: func() time.Time { return time.Now().Add(time.Hour) }})
	if err := m.With(context.Background(), "#chan", func(c *Conversation) {
		if err := c.Append([]messages.ChatMessage{{Role: messages.MessageRoleUser, Content: "active"}}); err != nil {
			t.Fatal(err)
		}
		m.SetTTL(time.Nanosecond) // Must not wait on the turn itself.
		if err := m.expire(context.Background()); err != nil {
			t.Fatal(err)
		}
		requireExists(t, m, "#chan", true)
		if c.Len() != 1 {
			t.Fatal("cleanup cleared an active conversation")
		}
	}); err != nil {
		t.Fatal(err)
	}
	if err := m.expire(context.Background()); err != nil {
		t.Fatal(err)
	}
	requireExists(t, m, "#chan", false)
}

func TestCleanupLoopAndScratchExpiry(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		m := testMemory(t, Config{TTL: time.Second})
		storeHistory(t, m, "#chan")
		orphanName := scratchPrefix + "orphan"
		orphan, err := m.store.Acquire(context.Background(), orphanName, sessions.AcquireOptions{Auto: true})
		if err != nil {
			t.Fatal(err)
		}
		if err := orphan.AddMessage(context.Background(), messages.ChatMessage{Role: messages.MessageRoleUser, Content: "abandoned"}); err != nil {
			t.Fatal(err)
		}
		if err := orphan.Close(); err != nil {
			t.Fatal(err)
		}
		// Named sessions' zero storage TTL must not disable cleanup.
		time.Sleep(cleanupInterval + time.Second)
		synctest.Wait()
		requireExists(t, m, "#chan", false)
		metadata, err := m.store.GetMetadata(context.Background(), orphanName)
		if err != nil || metadata.TTL != scratchTTL {
			t.Fatalf("scratch was governed by conversation TTL: metadata=%+v, err=%v", metadata, err)
		}
		m.SetTTL(0)
		storeHistory(t, m, "#forever")
		// TTL 0 must not disable scratch cleanup.
		if err := m.WithScratch(context.Background(), func(active *Conversation) {
			time.Sleep(scratchTTL + cleanupInterval)
			synctest.Wait()
			if _, err := m.store.GetMetadata(context.Background(), orphanName); err == nil {
				t.Fatal("abandoned scratch session outlived its TTL")
			}
			if err := active.Append([]messages.ChatMessage{{Role: messages.MessageRoleUser, Content: "still active"}}); err != nil {
				t.Fatalf("active scratch expired: %v", err)
			}
		}); err != nil {
			t.Fatal(err)
		}
		requireExists(t, m, "#forever", true)
		names, err := m.store.List(context.Background())
		if err != nil || len(names) != 1 || names[0] != "#forever" {
			t.Fatalf("scratch was not discarded after turn: names=%v, err=%v", names, err)
		}
		var wg sync.WaitGroup
		for range 3 {
			wg.Go(m.Close)
		}
		wg.Wait()
		select {
		case <-m.done:
		default:
			t.Fatal("Close returned before cleanup stopped")
		}
	})
}
