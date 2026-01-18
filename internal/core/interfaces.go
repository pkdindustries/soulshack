package core

import (
	"context"

	"github.com/alexschlessinger/pollytool/llm"
	"github.com/alexschlessinger/pollytool/sessions"
	"github.com/alexschlessinger/pollytool/tools"
	"go.uber.org/zap"

	"pkdindustries/soulshack/internal/config"
)

// ChatContextInterface provides all context needed for handling IRC messages
type ChatContextInterface interface {
	context.Context

	// Event methods
	IsAddressed() bool
	IsAdmin() bool
	Valid() bool
	IsPrivate() bool
	GetCommand() string
	GetSource() string
	GetArgs() []string

	// Responder methods
	Reply(string)
	ReplyAction(string)
	SendAction(target, message string)

	// Controller methods
	Join(string) bool
	Nick(string) bool
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

	// Runtime methods
	GetSession() sessions.Session
	GetConfig() *config.Configuration
	GetSystem() System
	GetLogger() *zap.SugaredLogger
}

// LLM defines the interface for the language model client
type LLM interface {
	// ChatCompletionStream returns a channel of string chunks for IRC output
	ChatCompletionStream(ChatContextInterface, *llm.CompletionRequest) <-chan string
}

type System interface {
	GetToolRegistry() *tools.ToolRegistry
	GetSessionStore() sessions.SessionStore
	GetLLM() LLM
	UpdateLLM(config.APIConfig) error
}
