// Package memory stores conversations as pollytool sessions: transcripts, artifacts and idle expiry.
// Transcripts are never trimmed; the budget only bounds what a request sends.
package memory

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/alexschlessinger/pollytool/artifacts"
	"github.com/alexschlessinger/pollytool/messages"
	"github.com/alexschlessinger/pollytool/sessions"
)

// cleanupInterval bounds how long abandoned sessions linger; turns also expire on open.
const cleanupInterval = time.Minute

// scratchTTL reaps scratch sessions orphaned mid-turn; leased ones never expire.
const scratchTTL = time.Hour

// scratchPrefix can't collide with a key: sessionName escapes every '%'.
const scratchPrefix = "%scratch-"

// ContextUsage describes a conversation's last completed request.
type ContextUsage struct {
	// EstimatedTokens prefers the provider's count when available.
	EstimatedTokens int
	Budget          int
	// Compacted reports that requests carry a summary in place of older exchanges.
	Compacted bool
}

// Config configures a Memory.
type Config struct {
	// Budget is maxcontext in tokens per request; zero is unlimited.
	Budget int
	// TTL is the idle expiry; zero never expires.
	TTL time.Duration
	// Path is the SQLite database; empty keeps conversations in memory.
	Path string
	// Now overrides the expiry clock, for tests.
	Now func() time.Time
}

// Memory holds one conversation per key, one turn at a time.
type Memory struct {
	store *sessions.SQLiteStore

	mu    sync.Mutex
	convs map[string]*Conversation
	now   func() time.Time

	budget atomic.Int64
	ttlNS  atomic.Int64

	// expiryMu orders SetTTL, turn acquisition and sweeps. Never held for a turn.
	expiryMu sync.RWMutex
	cancel   context.CancelFunc
	done     chan struct{}
}

// New opens the session store.
func New(cfg Config) (*Memory, error) {
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	storeCfg := sessions.StoreConfig{Mode: sessions.ModeMemory, CleanupInterval: cleanupInterval, AutoSessionTTL: scratchTTL}
	if cfg.Path != "" {
		storeCfg.Mode, storeCfg.Path = sessions.ModeDisk, cfg.Path
	}
	store, err := sessions.OpenStore(storeCfg)
	if err != nil {
		return nil, fmt.Errorf("open conversation store: %w", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	m := &Memory{
		store:  store,
		convs:  make(map[string]*Conversation),
		now:    now,
		cancel: cancel,
		done:   make(chan struct{}),
	}
	m.SetBudget(cfg.Budget)
	m.SetTTL(cfg.TTL)
	go m.cleanupLoop(ctx)
	return m, nil
}

// Close closes the session store.
func (m *Memory) Close() {
	m.cancel()
	<-m.done
	_ = m.store.Close()
}

// Budget returns the per-request token budget.
func (m *Memory) Budget() int { return int(m.budget.Load()) }

// SetBudget changes the token budget for all conversations.
func (m *Memory) SetBudget(tokens int) { m.budget.Store(int64(tokens)) }

// TTL returns the idle expiry.
func (m *Memory) TTL() time.Duration { return time.Duration(m.ttlNS.Load()) }

// SetTTL changes the idle expiry for all conversations, stored ones included.
func (m *Memory) SetTTL(d time.Duration) {
	m.expiryMu.Lock()
	defer m.expiryMu.Unlock()
	m.ttlNS.Store(int64(d))
}

// Keys returns the keys this process has run turns on.
func (m *Memory) Keys() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	keys := make([]string, 0, len(m.convs))
	for key := range m.convs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// With runs fn on key's conversation, one turn at a time.
// If ctx ends while queued, fn doesn't run and ctx's error is returned.
func (m *Memory) With(ctx context.Context, key string, fn func(*Conversation)) error {
	conversation := m.acquireConversation(key)
	defer m.releaseConversation(conversation)
	if err := conversation.enter(ctx); err != nil {
		return err
	}
	defer conversation.leave()

	if err := conversation.open(ctx, key); err != nil {
		return err
	}
	defer conversation.close()
	fn(conversation)
	return nil
}

// WithDetached runs fn on a discarded scratch conversation, queued on key's turns.
func (m *Memory) WithDetached(ctx context.Context, key string, fn func(*Conversation)) error {
	queue := m.acquireConversation(key)
	defer m.releaseConversation(queue)
	if err := queue.enter(ctx); err != nil {
		return err
	}
	defer queue.leave()
	return m.WithScratch(ctx, fn)
}

// WithScratch runs fn on an unqueued throwaway conversation, e.g. for a child agent.
func (m *Memory) WithScratch(ctx context.Context, fn func(*Conversation)) error {
	name := scratchPrefix + rand.Text()
	session, err := m.store.Acquire(ctx, name, sessions.AcquireOptions{Auto: true})
	if err != nil {
		return fmt.Errorf("open scratch conversation: %w", err)
	}
	scratch := &Conversation{session: session}
	defer func() {
		scratch.close()
		// Not fn's ctx, which may have ended.
		_ = m.store.Delete(context.WithoutCancel(ctx), name)
	}()
	fn(scratch)
	return nil
}

func (m *Memory) acquireConversation(key string) *Conversation {
	m.mu.Lock()
	defer m.mu.Unlock()
	conversation, ok := m.convs[key]
	if !ok {
		conversation = &Conversation{owner: m, turn: make(chan struct{}, 1)}
		m.convs[key] = conversation
	}
	conversation.refs++
	return conversation
}

func (m *Memory) releaseConversation(conversation *Conversation) {
	m.mu.Lock()
	defer m.mu.Unlock()
	conversation.refs--
}

func (m *Memory) pruneIdle(storedName string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for key, conversation := range m.convs {
		if conversation.refs == 0 && sessionName(key) == storedName {
			delete(m.convs, key)
		}
	}
}

// sessionName percent-escapes nick characters like '|' and '\', and '%' to stay one-to-one.
func sessionName(key string) string {
	var b strings.Builder
	for i := 0; i < len(key); i++ {
		c := key[i]
		edge := i == 0 || i == len(key)-1
		if c < 32 || c == 127 || strings.IndexByte(`/\:*?"<>|%`, c) >= 0 || edge && (c == '.' || c == ' ') {
			fmt.Fprintf(&b, "%%%02X", c)
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

// Conversation is one key's transcript and turn queue. Methods are goroutine-safe.
type Conversation struct {
	owner *Memory
	turn  chan struct{}
	refs  int // guarded by owner.mu

	mu       sync.Mutex
	session  sessions.Session
	history  []messages.ChatMessage
	usage    ContextUsage
	hasUsage bool
}

// open leases the session, expires it if idle, and loads the transcript.
func (c *Conversation) open(ctx context.Context, key string) error {
	m := c.owner
	m.expiryMu.RLock()
	defer m.expiryMu.RUnlock()
	session, err := m.store.Acquire(ctx, sessionName(key), sessions.AcquireOptions{})
	if err != nil {
		return fmt.Errorf("open conversation %s: %w", key, err)
	}
	history, err := prepare(ctx, session, m.TTL(), m.now())
	if err != nil {
		return errors.Join(fmt.Errorf("open conversation %s: %w", key, err), session.Close())
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.session, c.history = session, history
	if len(history) == 0 {
		c.usage, c.hasUsage = ContextUsage{}, false
	} else if !c.hasUsage && hasCompactionSummary(history) {
		c.usage.Compacted = true
	}
	return nil
}

func hasCompactionSummary(msgs []messages.ChatMessage) bool {
	for _, msg := range msgs {
		if comp, ok := msg.Compaction(); ok && comp.Summary != "" {
			return true
		}
	}
	return false
}

// prepare applies the current TTL. Named sessions keep a zero storage TTL
// so the store never expires them by a stale one.
func prepare(ctx context.Context, session sessions.Session, ttl time.Duration, now time.Time) ([]messages.ChatMessage, error) {
	metadata, err := session.GetMetadata(ctx)
	if err != nil {
		return nil, err
	}
	if ttl > 0 && now.Sub(metadata.LastUsed) > ttl {
		if err := session.Clear(ctx); err != nil {
			return nil, err
		}
	}
	return session.GetHistory(ctx)
}

func (c *Conversation) close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.session != nil {
		_ = c.session.Close()
	}
	c.session, c.history = nil, nil
}

// Messages returns a copy of the transcript.
func (c *Conversation) Messages() []messages.ChatMessage {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.history)
}

// Len returns the transcript length without copying it.
func (c *Conversation) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.history)
}

// Append adds messages to the transcript.
func (c *Conversation) Append(msgs []messages.ChatMessage) error {
	if len(msgs) == 0 {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.session != nil {
		if err := c.session.AddMessages(c.session.Context(), msgs); err != nil {
			return err
		}
	}
	c.history = append(c.history, msgs...)
	return nil
}

// Clear deletes the transcript and artifacts.
func (c *Conversation) Clear() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.session != nil {
		if err := c.session.Clear(c.session.Context()); err != nil {
			return err
		}
	}
	c.history = nil
	c.usage, c.hasUsage = ContextUsage{}, false
	return nil
}

// Artifacts returns the conversation's artifact store, or nil outside a turn.
func (c *Conversation) Artifacts() artifacts.Store {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.session == nil {
		return nil
	}
	return c.session.ArtifactStore()
}

// Compacted reports whether the conversation carries a compaction summary.
func (c *Conversation) Compacted() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.usage.Compacted || hasCompactionSummary(c.history)
}

// Usage returns the last completed turn's accounting.
func (c *Conversation) Usage() (ContextUsage, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.usage, c.hasUsage
}

// SetUsage records a completed turn's accounting.
func (c *Conversation) SetUsage(usage ContextUsage) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.usage, c.hasUsage = usage, true
}

func (c *Conversation) enter(ctx context.Context) error {
	if err := context.Cause(ctx); err != nil {
		return err
	}
	select {
	case c.turn <- struct{}{}:
		return nil
	case <-ctx.Done():
		return context.Cause(ctx)
	}
}

func (c *Conversation) leave() {
	select {
	case <-c.turn:
	default:
	}
}
