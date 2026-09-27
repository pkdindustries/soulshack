// Package memory owns every conversation the bot has: the transcript, the
// artifacts its tools produced, and when an idle conversation is forgotten.
//
// Each conversation is a pollytool session. pollytool's agent does not hold
// transcript state across turns ("callers provide messages and persist the
// generated messages themselves"), so a turn loads the session's transcript
// when it starts and appends what it generated. The session store is a SQLite
// database on disk when Config.Path names one, so conversations survive a
// restart, or an in-memory one that goes with the process.
//
// A stored transcript is never trimmed. The token budget (maxcontext) bounds
// what a request sends, not what is kept: polly projects each request down to
// the budget, omitting the oldest exchanges, and the agent's read_transcript
// can reach back past them.
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

// cleanupInterval is how often the store deletes conversations idle past their
// TTL. A conversation is also expired when its next turn starts, so this only
// bounds how long an abandoned one stays stored.
const cleanupInterval = time.Minute

// scratchTTL expires a scratch session its turn did not delete, as when the
// process stops mid-turn. A scratch session is leased while it is in use, and
// the store never expires a leased session, so this only has to outlast the
// time between its last use and the sweep.
const scratchTTL = time.Hour

// scratchPrefix starts every scratch session's name. sessionName escapes
// every '%' in a key as %XX with two hex digits, so no key can produce it.
const scratchPrefix = "%scratch-"

// ContextUsage describes the last provider request a conversation completed.
type ContextUsage struct {
	EstimatedTokens  int
	Budget           int
	OmittedExchanges int
}

// Config configures a Memory.
type Config struct {
	// Budget is the number of tokens a request may send from a conversation
	// (maxcontext). Zero sends everything.
	Budget int
	// TTL forgets a conversation after this much inactivity. Zero never
	// forgets one.
	TTL time.Duration
	// Path is the SQLite database conversations are stored in. Empty keeps
	// them in memory, and they end with the process.
	Path string
	// Now overrides the clock idle expiry is judged by, for tests.
	Now func() time.Time
}

// Memory holds one conversation per key, each usable by one turn at a time.
type Memory struct {
	store *sessions.SQLiteStore

	mu    sync.Mutex
	convs map[string]*Conversation
	now   func() time.Time

	budget atomic.Int64
	ttlNS  atomic.Int64
}

// New opens the session store and returns a Memory over it.
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
	m := &Memory{
		store: store,
		convs: make(map[string]*Conversation),
		now:   now,
	}
	m.SetBudget(cfg.Budget)
	m.SetTTL(cfg.TTL)
	return m, nil
}

// Close closes the session store. Stored conversations stay on disk.
func (m *Memory) Close() {
	_ = m.store.Close()
}

// Budget returns the token budget a request may send from a conversation.
func (m *Memory) Budget() int { return int(m.budget.Load()) }

// SetBudget changes the token budget for every conversation, including ones
// that already exist.
func (m *Memory) SetBudget(tokens int) { m.budget.Store(int64(tokens)) }

// TTL returns how long a conversation survives inactivity.
func (m *Memory) TTL() time.Duration { return time.Duration(m.ttlNS.Load()) }

// SetTTL changes the idle expiry for every conversation. A stored conversation
// takes the new TTL on its next turn.
func (m *Memory) SetTTL(d time.Duration) { m.ttlNS.Store(int64(d)) }

// Keys returns the conversations this process has run turns on, for
// inspection and tests.
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

// With runs fn with key's conversation, one turn at a time. When ctx ends
// before the previous turn does, fn does not run and the error is ctx's, so
// callers can tell the user that the previous turn is still running. Any
// other error means the stored conversation could not be opened.
func (m *Memory) With(ctx context.Context, key string, fn func(*Conversation)) error {
	conversation := m.conversation(key)
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

// WithDetached runs fn against a scratch conversation that is discarded
// afterwards, artifacts and all. It waits on key's turn queue, so a detached
// turn cannot interleave with the conversation's own turns.
func (m *Memory) WithDetached(ctx context.Context, key string, fn func(*Conversation)) error {
	queue := m.conversation(key)
	if err := queue.enter(ctx); err != nil {
		return err
	}
	defer queue.leave()
	return m.WithScratch(ctx, fn)
}

// WithScratch runs fn against a conversation of its own that is deleted
// afterwards, for an agent that answers to no conversation, such as a child.
// It has a transcript and an artifact store like any other, so its agent can
// keep large tool output out of its requests.
func (m *Memory) WithScratch(ctx context.Context, fn func(*Conversation)) error {
	name := scratchPrefix + rand.Text()
	session, err := m.store.Acquire(ctx, name, sessions.AcquireOptions{Auto: true})
	if err != nil {
		return fmt.Errorf("open scratch conversation: %w", err)
	}
	scratch := &Conversation{session: session}
	defer func() {
		scratch.close()
		// The store's own context: fn's may have ended, and the session
		// should go regardless.
		_ = m.store.Delete(context.WithoutCancel(ctx), name)
	}()
	fn(scratch)
	return nil
}

// conversation returns key's conversation, creating it if needed.
func (m *Memory) conversation(key string) *Conversation {
	m.mu.Lock()
	defer m.mu.Unlock()
	conversation, ok := m.convs[key]
	if !ok {
		conversation = &Conversation{owner: m, turn: make(chan struct{}, 1)}
		m.convs[key] = conversation
	}
	return conversation
}

// sessionName is key as a session name. IRC nicks may contain characters a
// session name may not, such as '|' and '\', so those are percent-escaped, as
// is '%' itself to keep the mapping one-to-one.
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

// Conversation is one key's transcript and turn queue. While a turn holds it,
// it holds that key's session and a copy of its transcript; a scratch
// conversation holds a session of its own and no queue. Its methods are safe
// to call from any goroutine; the turn slot only decides who goes first.
type Conversation struct {
	owner *Memory
	turn  chan struct{}

	mu       sync.Mutex
	session  sessions.Session
	history  []messages.ChatMessage
	usage    ContextUsage
	hasUsage bool
}

// open leases key's session for the turn holding the slot, starts it over if
// it has been idle past the TTL, and loads its transcript.
func (c *Conversation) open(ctx context.Context, key string) error {
	m := c.owner
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
	// A transcript that starts over, by expiry or otherwise, has no last
	// request to describe.
	if len(history) == 0 {
		c.usage, c.hasUsage = ContextUsage{}, false
	}
	return nil
}

// prepare expires a leased session idle past ttl and gives it the current TTL,
// then returns its transcript. The store expires sessions on the TTL each one
// was last given, so a TTL shortened since is judged here, by the live one.
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
	if metadata.TTL != ttl {
		metadata.TTL = ttl
		if err := session.SetMetadata(ctx, metadata); err != nil {
			return nil, err
		}
	}
	return session.GetHistory(ctx)
}

// close releases the session at the end of the turn.
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

// Len returns how many messages the transcript holds, for the callers that
// only want to know whether there is anything in it. Messages copies the whole
// transcript, which is a lot of work to answer that.
func (c *Conversation) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.history)
}

// Append stores messages at the end of the transcript.
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

// Clear forgets the transcript and its artifacts, keeping the conversation
// usable.
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

// Artifacts returns the store for the large tool output this conversation's
// transcript refers to, or nil outside a turn.
func (c *Conversation) Artifacts() artifacts.Store {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.session == nil {
		return nil
	}
	return c.session.ArtifactStore()
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
