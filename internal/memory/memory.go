// Package memory owns every conversation the bot has: the transcript, how much
// of it is kept, and when an idle conversation is forgotten.
//
// pollytool's agent does not hold transcript state across turns ("callers
// provide messages and persist the generated messages themselves"), so this
// package is the whole of soulshack's conversation storage. Nothing here is
// durable: a conversation lives as long as the process and its idle expiry
// allow.
package memory

import (
	"context"
	"slices"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/alexschlessinger/pollytool/messages"
	"github.com/alexschlessinger/pollytool/sessions"
)

// sweepInterval is how often idle conversations drop their transcripts. Expiry
// also happens on the next turn, so this only bounds how long the transcript of
// a conversation that has gone quiet stays in memory.
const sweepInterval = time.Minute

// ContextUsage describes the last provider request a conversation completed.
type ContextUsage struct {
	EstimatedTokens  int
	Budget           int
	OmittedExchanges int
}

// Config configures a Memory.
type Config struct {
	// Budget is the number of tokens kept in a conversation and sent from it
	// (maxcontext). Zero keeps everything.
	Budget int
	// TTL forgets a conversation after this much inactivity. Zero never
	// forgets one.
	TTL time.Duration
	// Now overrides the clock, for tests.
	Now func() time.Time
}

// Memory holds one conversation per key, each usable by one turn at a time.
type Memory struct {
	mu    sync.Mutex
	convs map[string]*Conversation
	now   func() time.Time

	budget atomic.Int64
	ttlNS  atomic.Int64

	closeOnce sync.Once
	done      chan struct{}
	wg        sync.WaitGroup
}

// New returns a Memory and starts its idle sweeper.
func New(cfg Config) *Memory {
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	m := &Memory{
		convs: make(map[string]*Conversation),
		now:   now,
		done:  make(chan struct{}),
	}
	m.SetBudget(cfg.Budget)
	m.SetTTL(cfg.TTL)

	m.wg.Add(1)
	go m.sweepLoop()
	return m
}

// Close stops the sweeper. Conversations are dropped with the process.
func (m *Memory) Close() {
	m.closeOnce.Do(func() { close(m.done) })
	m.wg.Wait()
}

// Budget returns the token budget conversations are trimmed to.
func (m *Memory) Budget() int { return int(m.budget.Load()) }

// SetBudget changes the token budget for every conversation, including ones
// that already exist.
func (m *Memory) SetBudget(tokens int) { m.budget.Store(int64(tokens)) }

// TTL returns how long a conversation survives inactivity.
func (m *Memory) TTL() time.Duration { return time.Duration(m.ttlNS.Load()) }

// SetTTL changes the idle expiry for every conversation.
func (m *Memory) SetTTL(d time.Duration) { m.ttlNS.Store(int64(d)) }

// Keys returns the conversations currently held, for inspection and tests.
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

// With runs fn with key's conversation, one turn at a time. It reports false
// without running fn when ctx ends first, so callers can tell the user that the
// previous turn is still running.
func (m *Memory) With(ctx context.Context, key string, fn func(*Conversation)) bool {
	conversation := m.conversation(key)
	return m.run(ctx, conversation, conversation, fn)
}

// WithDetached runs fn against a scratch conversation that is discarded
// afterwards. It waits on key's turn queue, so a detached turn cannot
// interleave with the conversation's own turns.
func (m *Memory) WithDetached(ctx context.Context, key string, fn func(*Conversation)) bool {
	return m.run(ctx, m.conversation(key), newConversation(m), fn)
}

// Sweep forgets conversations that have been idle longer than the TTL. It never
// interrupts a running turn.
func (m *Memory) Sweep() {
	ttl := m.TTL()
	if ttl <= 0 {
		return
	}
	m.mu.Lock()
	convs := make([]*Conversation, 0, len(m.convs))
	for _, conversation := range m.convs {
		convs = append(convs, conversation)
	}
	m.mu.Unlock()

	now := m.now()
	for _, conversation := range convs {
		conversation.forgetIfIdle(ttl, now)
	}
}

func (m *Memory) sweepLoop() {
	defer m.wg.Done()
	ticker := time.NewTicker(sweepInterval)
	defer ticker.Stop()
	for {
		select {
		case <-m.done:
			return
		case <-ticker.C:
			m.Sweep()
		}
	}
}

// conversation returns key's conversation, creating an empty one if needed.
func (m *Memory) conversation(key string) *Conversation {
	m.mu.Lock()
	defer m.mu.Unlock()
	conversation, ok := m.convs[key]
	if !ok {
		conversation = newConversation(m)
		conversation.lastUsed = m.now()
		m.convs[key] = conversation
	}
	return conversation
}

func newConversation(owner *Memory) *Conversation {
	return &Conversation{owner: owner, turn: make(chan struct{}, 1)}
}

// run holds the queue's turn slot while fn is running. queue carries the
// transcript and the turn slot; handle is what fn sees, which is a scratch
// conversation for detached turns.
func (m *Memory) run(ctx context.Context, queue, handle *Conversation, fn func(*Conversation)) bool {
	if !queue.enter(ctx) {
		return false
	}
	defer queue.leave()

	// The turn slot is held, so the transcript is ours to restart.
	queue.resetIfIdle(m.TTL(), m.now())
	queue.touch(m.now())
	fn(handle)
	queue.touch(m.now())
	return true
}

// Conversation is one key's transcript and turn queue. Its methods are safe to
// call from any goroutine; the turn slot only decides who goes first.
type Conversation struct {
	owner *Memory
	turn  chan struct{}

	mu       sync.Mutex
	history  []messages.ChatMessage
	usage    ContextUsage
	hasUsage bool
	lastUsed time.Time
}

// Messages returns a copy of the transcript.
func (c *Conversation) Messages() []messages.ChatMessage {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.history)
}

// Len returns how many messages the transcript holds, for the callers that
// only want to know whether there is anything in it. Messages copies the whole
// transcript, which is a lot of work to answer that.
func (c *Conversation) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.history)
}

// Append adds messages and trims the transcript to the token budget, keeping
// the newest exchanges.
func (c *Conversation) Append(msgs []messages.ChatMessage) {
	if len(msgs) == 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.history = append(c.history, msgs...)
	if c.owner != nil {
		if budget := c.owner.Budget(); budget > 0 {
			c.history = sessions.TrimHistory(c.history, budget)
		}
	}
}

// Clear forgets the transcript, keeping the conversation usable.
func (c *Conversation) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.history = nil
	c.usage, c.hasUsage = ContextUsage{}, false
}

// Usage returns the request accounting of the last completed turn.
func (c *Conversation) Usage() (ContextUsage, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.usage, c.hasUsage
}

// SetUsage records the request accounting of a completed turn.
func (c *Conversation) SetUsage(usage ContextUsage) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.usage, c.hasUsage = usage, true
}

func (c *Conversation) enter(ctx context.Context) bool {
	if ctx.Err() != nil {
		return false
	}
	select {
	case c.turn <- struct{}{}:
		return true
	case <-ctx.Done():
		return false
	}
}

func (c *Conversation) leave() {
	select {
	case <-c.turn:
	default:
	}
}

func (c *Conversation) touch(now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastUsed = now
}

// forgetIfIdle drops the transcript of a conversation that has gone quiet,
// unless a turn is running.
func (c *Conversation) forgetIfIdle(ttl time.Duration, now time.Time) {
	select {
	case c.turn <- struct{}{}:
		defer c.leave()
		c.resetIfIdle(ttl, now)
	default:
		// A turn is running; its completion refreshes the clock anyway.
	}
}

// resetIfIdle drops the transcript when the conversation has been idle past
// ttl. The caller holds the turn slot.
func (c *Conversation) resetIfIdle(ttl time.Duration, now time.Time) {
	if ttl <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if now.Sub(c.lastUsed) <= ttl {
		return
	}
	c.history = nil
	c.usage, c.hasUsage = ContextUsage{}, false
}
