package core

import (
	"context"

	"github.com/alexschlessinger/pollytool/llm"
	"github.com/alexschlessinger/pollytool/sessions"
	"github.com/alexschlessinger/pollytool/tools"
	"github.com/lrstanley/girc"
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
	Action(string)

	// Controller methods
	Join(string) bool
	Nick(string) bool
	Mode(string, string, string) bool
	Kick(string, string, string) bool
	Topic(string, string) bool
	Oper(string, string) bool
	LookupUser(string) (string, string, bool)
	LookupChannel(string) *girc.Channel
	GetClient() *girc.Client

	// Runtime methods
	GetSession() sessions.Session
	GetConfig() *config.Configuration
	GetSystem() System
	GetLogger() *zap.SugaredLogger
}

// LLM defines the interface for the language model client
type LLM interface {
	// ChatCompletionStream returns a single byte channel with chunked output for IRC
	ChatCompletionStream(*llm.CompletionRequest, ChatContextInterface) <-chan []byte
}

type System interface {
	GetToolRegistry() *tools.ToolRegistry
	GetSessionStore() sessions.SessionStore
	GetLLM() LLM
	UpdateLLM(config.APIConfig) error
}
