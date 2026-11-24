package core

import (
	"context"
	"log/slog"
	"time"

	"github.com/alexschlessinger/pollytool/artifacts"
	"github.com/alexschlessinger/pollytool/llm"
	"github.com/alexschlessinger/pollytool/tools"

	"pkdindustries/soulshack/internal/config"
	"pkdindustries/soulshack/internal/memory"
)

// ChunkWriter splits model output into transport-sized messages.
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
	NewChunkWriter(output chan<- string) ChunkWriter

	// Background returns a copy that outlives the event, with only the deadline changed.
	// Caller owns the cancel func.
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

// chatKey: a chat context answers it itself; WithChat is for bare contexts.
type chatKey struct{}

// ChatKey is the key a ChatContextInterface answers Value with.
func ChatKey() any { return chatKey{} }

// WithChat attaches a chat context to ctx.
func WithChat(ctx context.Context, chat ChatContextInterface) context.Context {
	return context.WithValue(ctx, chatKey{}, chat)
}

// ChatFromContext returns the chat context in ctx.
func ChatFromContext(ctx context.Context) (ChatContextInterface, bool) {
	chat, ok := ctx.Value(chatKey{}).(ChatContextInterface)
	return chat, ok
}

// CompletionStream carries IRC output on Chunks and the result on Done.
// Drain Chunks before reading Done; both close after history is saved.
type CompletionStream struct {
	Chunks <-chan string
	Done   <-chan error
}

// LLM defines the interface for the language model client
type LLM interface {
	ChatCompletionStream(*Turn, *llm.CompletionRequest) CompletionStream
	// RunSubagent runs a child agent to completion. It blocks.
	RunSubagent(context.Context, SubagentSpec) (SubagentResult, error)
}

// SubagentSpec is a child agent's full input; the child outlives its turn, so nothing comes from ctx.
type SubagentSpec struct {
	// Task is the brief; the child knows nothing else.
	Task string
	// Label is a short display name.
	Label string
	Model string
	// Tools are allowed names or globs; nil inherits the parent's, minus child-forbidden ones.
	Tools []string
	// Registry is the parent's registry the child's tools derive from.
	Registry *tools.ToolRegistry
	// Config is the child's snapshot.
	Config *config.Configuration
	// Artifacts stores large tool output; nil disables it.
	Artifacts artifacts.Store
}

// SubagentResult is a child's reply and token usage.
type SubagentResult struct {
	Text                      string
	InputTokens, OutputTokens int
}

// AgentInfo describes a running child agent.
type AgentInfo struct {
	Label        string
	Conversation string
	Source       string
	Started      time.Time
}

// Agents tracks running child agents, process-wide.
type Agents interface {
	// List returns the running children, oldest first.
	List() []AgentInfo
}

type System interface {
	GetToolRegistry() *tools.ToolRegistry
	GetMemory() *memory.Memory
	GetLLM() LLM
	// GetAgents returns nil when subagents are disabled.
	GetAgents() Agents
	UpdateLLM(config.APIConfig) error
	// GetConfig returns a snapshot.
	GetConfig() *config.Configuration
	// UpdateConfig applies and publishes a change if fn succeeds.
	UpdateConfig(func(*config.Configuration) error) error
}
