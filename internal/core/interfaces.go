package core

import (
	"context"
	"log/slog"

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

// LLM defines the interface for the language model client
type LLM interface {
	// ChatCompletionStream returns IRC output and closes after history saving finishes.
	ChatCompletionStream(*Turn, *llm.CompletionRequest) <-chan string
}

type System interface {
	GetToolRegistry() *tools.ToolRegistry
	GetMemory() *memory.Memory
	GetLLM() LLM
	UpdateLLM(config.APIConfig) error
	// GetConfig returns the settings this turn runs with: a snapshot, so
	// reading them is safe while another turn changes them.
	GetConfig() *config.Configuration
	// UpdateConfig changes the running configuration under the write lock.
	UpdateConfig(func(*config.Configuration) error) error
}
