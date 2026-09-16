package testing

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/lrstanley/girc"

	"pkdindustries/soulshack/internal/config"
	"pkdindustries/soulshack/internal/core"
	"pkdindustries/soulshack/internal/memory"
)

// MockChatContext implements core.ChatContextInterface for testing
type MockChatContext struct {
	context.Context

	// Configurable return values
	Addressed bool
	Admin     bool
	Private   bool
	Command   string
	Source    string
	Args      []string

	// Recorded replies (for assertions). Background work replies from its
	// own goroutine, so mu guards them and the accessors below read them
	// safely; a test that reads the slices directly must be sure nothing
	// else is still replying.
	mu      sync.Mutex
	Replies []string
	Actions []string

	// Injected dependencies
	conversation *memory.Conversation
	cfg          *config.Store
	sys          core.System
	logger       *slog.Logger
	client       *girc.Client

	// Parent stands in for the bot's context, which background work derives
	// from. Cancelling it is how a test shuts the bot down.
	Parent context.Context

	// Mock data for lookups
	Users         map[string]*core.UserInfo
	Channels      map[string]*core.ChannelInfo
	ChannelUsers  map[string][]core.ChannelUser
	BotNick       string
	ServerOptions map[string]string
}

// Verify MockChatContext implements core.ChatContextInterface
var _ core.ChatContextInterface = (*MockChatContext)(nil)

// NewMockContext creates a new MockChatContext with sensible defaults
func NewMockContext() *MockChatContext {
	return &MockChatContext{
		Context:      context.Background(),
		Addressed:    true,
		Admin:        false,
		Private:      false,
		Source:       "testuser",
		Args:         []string{},
		Replies:      []string{},
		Actions:      []string{},
		logger:       slog.New(discardHandler{}),
		client:       NewMockIRCClient(),
		Users:        make(map[string]*core.UserInfo),
		Channels:     make(map[string]*core.ChannelInfo),
		ChannelUsers: make(map[string][]core.ChannelUser),
		BotNick:      "soulshack",
	}
}

// discardHandler is a slog.Handler that discards all log records
type discardHandler struct{}

func (discardHandler) Enabled(context.Context, slog.Level) bool  { return false }
func (discardHandler) Handle(context.Context, slog.Record) error { return nil }
func (d discardHandler) WithAttrs([]slog.Attr) slog.Handler      { return d }
func (d discardHandler) WithGroup(string) slog.Handler           { return d }

// Builder methods for fluent test setup

// WithContext sets a custom context (for timeout/cancellation testing)
func (m *MockChatContext) WithContext(ctx context.Context) *MockChatContext {
	m.Context = ctx
	return m
}

// WithAdmin sets the admin flag
func (m *MockChatContext) WithAdmin(admin bool) *MockChatContext {
	m.Admin = admin
	return m
}

// WithAddressed sets whether the bot was addressed
func (m *MockChatContext) WithAddressed(addressed bool) *MockChatContext {
	m.Addressed = addressed
	return m
}

// WithPrivate sets whether this is a private message
func (m *MockChatContext) WithPrivate(private bool) *MockChatContext {
	m.Private = private
	return m
}

// WithArgs sets the parsed arguments
func (m *MockChatContext) WithArgs(args ...string) *MockChatContext {
	m.Args = args
	if len(args) > 0 {
		m.Command = strings.ToLower(args[0])
	}
	return m
}

// WithSource sets the source nick
func (m *MockChatContext) WithSource(source string) *MockChatContext {
	m.Source = source
	return m
}

// WithConfig pins the configuration this context reads, for tests that need
// settings the system does not have (a different channel, say).
func (m *MockChatContext) WithConfig(cfg *config.Configuration) *MockChatContext {
	m.cfg = config.NewStore(cfg)
	return m
}

// WithSystem sets the system
func (m *MockChatContext) WithSystem(sys core.System) *MockChatContext {
	m.sys = sys
	return m
}

// WithConversation sets the conversation this context's turn works on
func (m *MockChatContext) WithConversation(conversation *memory.Conversation) *MockChatContext {
	m.conversation = conversation
	return m
}

// WithURLWatcher sets the URLWatcher config flag
func (m *MockChatContext) WithURLWatcher(enabled bool) *MockChatContext {
	m.updateConfig(func(c *config.Configuration) error {
		c.Bot.URLWatcher = enabled
		return nil
	})
	return m
}

// Event methods

func (m *MockChatContext) IsAddressed() bool {
	return m.Addressed
}

func (m *MockChatContext) IsAdmin() bool {
	return m.Admin
}

func (m *MockChatContext) IsPrivate() bool {
	return m.Private
}

func (m *MockChatContext) GetCommand() string {
	return m.Command
}

func (m *MockChatContext) GetSource() string {
	return m.Source
}

func (m *MockChatContext) GetArgs() []string {
	return m.Args
}

// Responder methods

func (m *MockChatContext) Reply(msg string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Replies = append(m.Replies, msg)
}

func (m *MockChatContext) ReplyAction(msg string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Actions = append(m.Actions, msg)
}

func (m *MockChatContext) SendAction(string, string) {}

// Controller methods. These are IRC side effects: the mock accepts them and
// reports success, and a test that needs to see one records it itself.
func (m *MockChatContext) Join(string) bool                       { return true }
func (m *MockChatContext) JoinWithKey(string, string) bool        { return true }
func (m *MockChatContext) Nick(string) bool                       { return true }
func (m *MockChatContext) SetMode(string, string, ...string) bool { return true }
func (m *MockChatContext) Kick(string, string, string) bool       { return true }
func (m *MockChatContext) Topic(string, string) bool              { return true }
func (m *MockChatContext) Oper(string, string) bool               { return true }
func (m *MockChatContext) Ban(string, string) bool                { return true }
func (m *MockChatContext) Unban(string, string) bool              { return true }
func (m *MockChatContext) Invite(string, string) bool             { return true }
func (m *MockChatContext) FatalError(error)                       {}

func (m *MockChatContext) GetUser(nick string) *core.UserInfo {
	return m.Users[nick]
}

func (m *MockChatContext) GetChannel(channel string) *core.ChannelInfo {
	return m.Channels[channel]
}

func (m *MockChatContext) GetChannelUsers(channel string) []core.ChannelUser {
	return m.ChannelUsers[channel]
}

func (m *MockChatContext) GetBotNick() string {
	return m.BotNick
}

func (m *MockChatContext) GetServerOption(key string) (string, bool) {
	value, ok := m.ServerOptions[key]
	return value, ok
}

// Runtime methods

// Turn wraps this context as one chat turn over the conversation set by
// WithConversation. Commands and completions take a turn rather than a bare
// context, since that is where the transcript lives.
func (m *MockChatContext) Turn() *core.Turn {
	return &core.Turn{ChatContextInterface: m, Conversation: m.conversation}
}

func (m *MockChatContext) GetConversationKey() string {
	return m.GetConfig().Server.Channel
}

// Value answers the lookup tools use to find their chat context, the way the
// IRC context does, so a tool under test can take this mock as its context
// directly.
func (m *MockChatContext) Value(key any) any {
	if key == core.ChatKey() {
		return core.ChatContextInterface(m)
	}
	return m.Context.Value(key)
}

// Background hands back a context with a deadline of its own that does not end
// when this one is cancelled, matching what the IRC context does for work that
// outlives an event. Replies still land on this mock, so a test asserts on the
// context it built whichever one the background work replied through.
//
// It derives from Parent, which stands in for the bot's own context: a test
// cancels that to shut the bot down under whatever is running.
func (m *MockChatContext) Background(timeout time.Duration) (core.ChatContextInterface, context.CancelFunc) {
	parent := m.Parent
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	return &backgroundMockContext{MockChatContext: m, ctx: ctx}, cancel
}

// backgroundMockContext is the mock with its deadline replaced. It keeps the
// recorders of the context it came from, and overrides the four context
// methods that the embedded mock would otherwise have supplied.
type backgroundMockContext struct {
	*MockChatContext
	ctx context.Context
}

func (b *backgroundMockContext) Deadline() (time.Time, bool) { return b.ctx.Deadline() }
func (b *backgroundMockContext) Done() <-chan struct{}       { return b.ctx.Done() }
func (b *backgroundMockContext) Err() error                  { return b.ctx.Err() }

func (b *backgroundMockContext) Value(key any) any {
	if key == core.ChatKey() {
		return core.ChatContextInterface(b)
	}
	return b.ctx.Value(key)
}

func (b *backgroundMockContext) Background(timeout time.Duration) (core.ChatContextInterface, context.CancelFunc) {
	return b.MockChatContext.Background(timeout)
}

// NewChunkWriter hands model output straight through, so tests see the chunks
// the LLM produced rather than IRC-framed ones.
func (m *MockChatContext) NewChunkWriter(output chan<- string) core.ChunkWriter {
	return passthroughChunkWriter{output: output}
}

type passthroughChunkWriter struct{ output chan<- string }

func (w passthroughChunkWriter) Write(content string) {
	if content != "" {
		w.output <- content
	}
}

func (w passthroughChunkWriter) Flush() {}

// GetConfig returns the settings this context reads: the system's, so settings
// changed through the system are visible, unless a test pinned its own.
func (m *MockChatContext) GetConfig() *config.Configuration {
	if m.cfg == nil {
		if m.sys != nil {
			return m.sys.GetConfig()
		}
		m.cfg = config.NewStore(DefaultTestConfig())
	}
	return m.cfg.Snapshot()
}

// updateConfig changes the settings this context reads, wherever they live.
func (m *MockChatContext) updateConfig(fn func(*config.Configuration) error) {
	if m.cfg == nil && m.sys != nil {
		if err := m.sys.UpdateConfig(fn); err != nil {
			panic(err)
		}
		return
	}
	if m.cfg == nil {
		m.cfg = config.NewStore(DefaultTestConfig())
	}
	if err := m.cfg.Update(fn); err != nil {
		panic(err)
	}
}

func (m *MockChatContext) GetSystem() core.System {
	return m.sys
}

func (m *MockChatContext) GetLogger() *slog.Logger {
	return m.logger
}

// Assertion helpers

// HasReply checks if any reply contains the given substring
func (m *MockChatContext) HasReply(substring string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.Replies {
		if strings.Contains(r, substring) {
			return true
		}
	}
	return false
}

// LastReply returns the last reply, or empty string if none
func (m *MockChatContext) LastReply() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.Replies) == 0 {
		return ""
	}
	return m.Replies[len(m.Replies)-1]
}

// ReplyCount returns the number of replies
func (m *MockChatContext) ReplyCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.Replies)
}

// AllReplies and AllActions copy what has been recorded so far. They are what
// a test reads while background work may still be replying.
func (m *MockChatContext) AllReplies() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string(nil), m.Replies...)
}

func (m *MockChatContext) AllActions() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string(nil), m.Actions...)
}
