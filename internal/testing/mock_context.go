package testing

import (
	"context"
	"log/slog"
	"strings"

	"github.com/alexschlessinger/pollytool/sessions"
	"github.com/lrstanley/girc"

	"pkdindustries/soulshack/internal/config"
	"pkdindustries/soulshack/internal/core"
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

	// Recorded calls (for assertions)
	Replies          []string
	Actions          []string
	JoinCalls        []string
	JoinWithKeyCalls []JoinWithKeyCall
	NickCalls        []string
	FatalErrors      []error
	KickCalls       []KickCall
	SetModeCalls    []ModeCall
	TopicCalls      []TopicCall
	OperCalls       []OperCall
	BanCalls        []string
	UnbanCalls      []string
	InviteCalls     []InviteCall
	SendActionCalls []ActionCall

	// Injected dependencies
	session sessions.Session
	cfg     *config.Configuration
	sys     core.System
	logger  *slog.Logger
	client  *girc.Client

	// Mock data for lookups
	Users        map[string]*core.UserInfo
	Channels     map[string]*core.ChannelInfo
	ChannelUsers map[string][]core.ChannelUser
	BotNick      string
}

type InviteCall struct {
	Channel string
	Nick    string
}

type JoinWithKeyCall struct {
	Channel string
	Key     string
}

type ActionCall struct {
	Target  string
	Message string
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
		cfg:          DefaultTestConfig(),
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

// WithConfig sets the configuration
func (m *MockChatContext) WithConfig(cfg *config.Configuration) *MockChatContext {
	m.cfg = cfg
	return m
}

// WithSystem sets the system
func (m *MockChatContext) WithSystem(sys core.System) *MockChatContext {
	m.sys = sys
	return m
}

// WithSession sets the session
func (m *MockChatContext) WithSession(session sessions.Session) *MockChatContext {
	m.session = session
	return m
}

// WithLogger sets the logger
func (m *MockChatContext) WithLogger(logger *slog.Logger) *MockChatContext {
	m.logger = logger
	return m
}

// WithURLWatcher sets the URLWatcher config flag
func (m *MockChatContext) WithURLWatcher(enabled bool) *MockChatContext {
	m.cfg.Bot.URLWatcher = enabled
	return m
}

// WithUser adds a mock user for LookupUser
func (m *MockChatContext) WithUser(nick, ident, host string) *MockChatContext {
	m.Users[nick] = &core.UserInfo{Nick: nick, Ident: ident, Host: host}
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
	m.Replies = append(m.Replies, msg)
}

func (m *MockChatContext) ReplyAction(msg string) {
	m.Actions = append(m.Actions, msg)
}

func (m *MockChatContext) SendAction(target, msg string) {
	m.SendActionCalls = append(m.SendActionCalls, ActionCall{Target: target, Message: msg})
}

// Controller methods

func (m *MockChatContext) Join(channel string) bool {
	m.JoinCalls = append(m.JoinCalls, channel)
	return true
}

func (m *MockChatContext) JoinWithKey(channel, key string) bool {
	m.JoinWithKeyCalls = append(m.JoinWithKeyCalls, JoinWithKeyCall{Channel: channel, Key: key})
	return true
}

func (m *MockChatContext) FatalError(err error) {
	m.FatalErrors = append(m.FatalErrors, err)
}

func (m *MockChatContext) Nick(nickname string) bool {
	m.NickCalls = append(m.NickCalls, nickname)
	return true
}

func (m *MockChatContext) SetMode(target, flags string, args ...string) bool {
	m.SetModeCalls = append(m.SetModeCalls, ModeCall{Channel: target, Mode: flags, Target: strings.Join(args, " ")})
	return true
}

func (m *MockChatContext) Kick(channel, nick, reason string) bool {
	m.KickCalls = append(m.KickCalls, KickCall{Channel: channel, Nick: nick, Reason: reason})
	return true
}

func (m *MockChatContext) Topic(channel, topic string) bool {
	m.TopicCalls = append(m.TopicCalls, TopicCall{Channel: channel, Topic: topic})
	return true
}

func (m *MockChatContext) Oper(channel, nick string) bool {
	m.OperCalls = append(m.OperCalls, OperCall{Channel: channel, Nick: nick})
	return true
}

func (m *MockChatContext) Ban(channel, target string) bool {
	m.BanCalls = append(m.BanCalls, target)
	return true
}

func (m *MockChatContext) Unban(channel, target string) bool {
	m.UnbanCalls = append(m.UnbanCalls, target)
	return true
}

func (m *MockChatContext) Invite(channel, nick string) bool {
	m.InviteCalls = append(m.InviteCalls, InviteCall{Channel: channel, Nick: nick})
	return true
}

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

func (m *MockChatContext) GetLockKey() string {
	if m.cfg != nil {
		return m.cfg.Server.Channel
	}
	return "#test"
}

func (m *MockChatContext) IsOp(channel, nick string) bool {
	return false // override in tests as needed
}

// Runtime methods

func (m *MockChatContext) GetSession() sessions.Session {
	if m.session != nil {
		return m.session
	}
	// Create a default session if none set
	if m.sys != nil {
		sess, _ := m.sys.GetSessionStore().Get("test")
		return sess
	}
	return nil
}

func (m *MockChatContext) GetConfig() *config.Configuration {
	return m.cfg
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
	for _, r := range m.Replies {
		if strings.Contains(r, substring) {
			return true
		}
	}
	return false
}

// LastReply returns the last reply, or empty string if none
func (m *MockChatContext) LastReply() string {
	if len(m.Replies) == 0 {
		return ""
	}
	return m.Replies[len(m.Replies)-1]
}

// ReplyCount returns the number of replies
func (m *MockChatContext) ReplyCount() int {
	return len(m.Replies)
}
