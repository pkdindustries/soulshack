package core

import (
	"context"
	"log/slog"
	"time"

	"github.com/alexschlessinger/pollytool/llm"
	"github.com/alexschlessinger/pollytool/tools"

	"pkdindustries/soulshack/internal/config"
	"pkdindustries/soulshack/internal/memory"
)

// ChunkWriter frames model output into messages a transport can send, emitting
// each finished message on the channel it was built with.
type ChunkWriter interface {
	Write(content string)
	Flush()
}

// ChatContextInterface provides all context needed for handling IRC messages
type ChatContextInterface interface {
	context.Context

	// Event methods
	IsAddressed() bool
	IsAdmin() bool
	IsPrivate() bool
	GetCommand() string
	GetSource() string
	GetArgs() []string

	// Responder methods
	Reply(string)
	ReplyAction(string)
	SendAction(target, message string)
	// NewChunkWriter frames output for this chat, since only the transport
	// knows the message size limit.
	NewChunkWriter(output chan<- string) ChunkWriter

	// Background returns a copy of this chat context whose lifetime is the
	// process's rather than this event's, so work started here can finish
	// after the event handler has returned and still reach the channel. It
	// is the lifetime that changes: the conversation, the target, and the
	// tools are the ones this context already had. Callers own the cancel
	// func. Note the distinct axis: WithDetachedConversation detaches the
	// transcript, this detaches the deadline.
	Background(timeout time.Duration) (ChatContextInterface, context.CancelFunc)

	// Controller methods
	Join(string) bool
	JoinWithKey(channel, key string) bool
	Nick(string) bool
	FatalError(err error)
	SetMode(target, flags string, args ...string) bool
	Kick(channel, nick, reason string) bool
	Ban(channel, target string) bool
	Unban(channel, target string) bool
	Invite(channel, nick string) bool
	Topic(channel, topic string) bool
	Oper(string, string) bool

	// State methods
	GetUser(nick string) *UserInfo
	GetChannel(name string) *ChannelInfo
	GetChannelUsers(channel string) []ChannelUser
	GetBotNick() string
	GetServerOption(key string) (string, bool)
	GetConversationKey() string

	// Runtime methods
	GetConfig() *config.Configuration
	GetSystem() System
	GetLogger() *slog.Logger
}

// chatKey is what a tool looks a chat context up by. A turn's context is the
// chat context, so a context built from one resolves the lookup by itself and
// no layer has to inject a value into tool calls; WithChat is for the callers
// that have a bare context, such as tests.
type chatKey struct{}

// ChatKey is the key a ChatContextInterface answers Value with.
func ChatKey() any { return chatKey{} }

// WithChat attaches a chat context to ctx.
func WithChat(ctx context.Context, chat ChatContextInterface) context.Context {
	return context.WithValue(ctx, chatKey{}, chat)
}

// ChatFromContext returns the chat context a tool is running under.
func ChatFromContext(ctx context.Context) (ChatContextInterface, bool) {
	chat, ok := ctx.Value(chatKey{}).(ChatContextInterface)
	return chat, ok
}

// LLM defines the interface for the language model client
type LLM interface {
	// ChatCompletionStream returns IRC output and closes after history saving finishes.
	ChatCompletionStream(*Turn, *llm.CompletionRequest) <-chan string
	// RunSubagent runs one child agent to completion and returns its reply.
	// It blocks, so callers that must not block run it on their own
	// goroutine.
	RunSubagent(context.Context, SubagentSpec) (SubagentResult, error)
}

// SubagentSpec is one child agent's brief. Everything the child needs is
// passed explicitly rather than read back out of the context: a background
// child outlives the turn that asked for it, so it cannot rely on the turn's
// settings still being the current ones.
type SubagentSpec struct {
	// Task is the brief. It is everything the child knows.
	Task string
	// Label names the job in a few words, for the people watching.
	Label string
	// Model overrides the configured subagent model when set.
	Model string
	// Tools lists the tool names or globs the child may use. Nil inherits
	// what the parent has, minus what a child is never given.
	Tools []string
	// Registry is the parent's tools, which the child's view derives from.
	Registry *tools.ToolRegistry
	// Config is the snapshot the child runs under.
	Config *config.Configuration
}

// SubagentResult is what a child returned.
type SubagentResult struct {
	Text                      string
	InputTokens, OutputTokens int
}

// AgentInfo describes one child agent that is currently running.
type AgentInfo struct {
	Label        string
	Conversation string
	Source       string
	Started      time.Time
}

// Agents tracks the child agents running right now. Children outlive the
// turns that spawn them, so the tracker belongs to the process, not the turn.
type Agents interface {
	// List returns the running children, oldest first.
	List() []AgentInfo
}

type System interface {
	GetToolRegistry() *tools.ToolRegistry
	GetMemory() *memory.Memory
	GetLLM() LLM
	// GetAgents returns the running child agents, or nil when subagents are
	// disabled.
	GetAgents() Agents
	UpdateLLM(config.APIConfig) error
	// GetConfig returns the settings this turn runs with: a snapshot, so
	// reading them is safe while another turn changes them.
	GetConfig() *config.Configuration
	// UpdateConfig changes the running configuration under the write lock.
	UpdateConfig(func(*config.Configuration) error) error
}
