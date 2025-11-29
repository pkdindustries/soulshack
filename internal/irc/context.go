package irc

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"

	"go.uber.org/zap"

	"github.com/alexschlessinger/pollytool/sessions"
	"github.com/lrstanley/girc"

	"pkdindustries/soulshack/internal/config"
	"pkdindustries/soulshack/internal/core"
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
	GetSystem() core.System
	GetLogger() *zap.SugaredLogger
}

type ChatContext struct {
	context.Context
	Sys       core.System
	Session   sessions.Session
	Config    *config.Configuration
	client    *girc.Client
	event     *girc.Event
	args      []string
	logger    *zap.SugaredLogger
	requestID string
}

var _ ChatContextInterface = (*ChatContext)(nil)

func NewChatContext(parentctx context.Context, config *config.Configuration, system core.System, ircclient *girc.Client, e *girc.Event) (ChatContextInterface, context.CancelFunc) {
	timedctx, cancel := context.WithTimeout(parentctx, config.API.Timeout)

	// Generate a unique request ID for correlation
	requestID := generateRequestID()

	ctx := ChatContext{
		Context:   timedctx,
		Config:    config,
		Sys:       system,
		client:    ircclient,
		event:     e,
		args:      strings.Fields(e.Last()),
		requestID: requestID,
		logger: zap.S().With(
			"request_id", requestID,
			"channel", e.Params[0],
			"source", e.Source.Name,
		),
	}

	if ctx.IsAddressed() {
		ctx.args = ctx.args[1:]
	}

	if e.Source == nil {
		e.Source = &girc.Source{
			Name: config.Server.Channel,
		}
	}

	key := e.Params[0]
	if !girc.IsValidChannel(key) {
		key = e.Source.Name
	}

	session, err := ctx.Sys.GetSessionStore().Get(key)
	if err != nil {
		zap.S().Fatalw("Failed to get session for key", "key", key, "error", err)
	}
	ctx.Session = session
	return ctx, cancel
}

func (c ChatContext) GetSystem() core.System {
	return c.Sys
}

func (c ChatContext) GetConfig() *config.Configuration {
	return c.Config
}

func (c ChatContext) GetLogger() *zap.SugaredLogger {
	return c.logger
}

func (c ChatContext) Oper(channel, nick string) bool {
	c.client.Cmd.Oper(channel, nick)
	return true
}

func (c ChatContext) Mode(channel, target, mode string) bool {
	c.client.Cmd.Mode(channel, target, mode)
	return true
}

func (c ChatContext) Kick(channel, nick, reason string) bool {
	c.client.Cmd.Kick(channel, nick, reason)
	return true
}

func (c ChatContext) Topic(channel, topic string) bool {
	c.client.Cmd.Topic(channel, topic)
	return true
}

func (s ChatContext) IsAddressed() bool {
	return strings.HasPrefix(s.event.Last(), s.client.GetNick())
}

func (c ChatContext) Nick(nickname string) bool {
	c.client.Cmd.Nick(nickname)
	return true
}

func (c ChatContext) Join(channel string) bool {
	c.client.Cmd.Join(channel)
	return true
}

func (c ChatContext) GetArgs() []string {
	return c.args
}

func (c ChatContext) GetSession() sessions.Session {
	return c.Session
}

func (c ChatContext) GetClient() *girc.Client {
	return c.client
}

func (c ChatContext) GetSource() string {
	return c.event.Source.Name
}

func (c ChatContext) IsAdmin() bool {
	hostmask := c.event.Source.String()
	c.logger.Debugw("Checking hostmask", "hostmask", hostmask)
	// XXX: if no admins are configured, all hostmasks are admins
	if len(c.Config.Bot.Admins) == 0 {
		c.logger.Debug("All hostmasks are admin; please configure admins")
		return true
	}
	for _, user := range c.Config.Bot.Admins {
		if user == hostmask {
			c.logger.Debugw("User is admin", "hostmask", hostmask)
			return true
		}
	}
	return false
}

func (c ChatContext) Reply(message string) {
	c.client.Cmd.Reply(*c.event, message)

}

func (c ChatContext) Action(message string) {
	target := c.event.Params[0]
	if !girc.IsValidChannel(target) {
		// For PMs, send a regular message instead of an action
		c.client.Cmd.Message(c.event.Source.Name, message)
		return
	}
	c.client.Cmd.Action(target, message)
}

func (c ChatContext) LookupUser(nick string) (string, string, bool) {
	user := c.client.LookupUser(nick)
	if user == nil {
		return "", "", false
	}
	// Return ident and host separately for flexibility
	return user.Ident, user.Host, true
}

func (c ChatContext) LookupChannel(channel string) *girc.Channel {
	return c.client.LookupChannel(channel)
}

// checks if the message is valid for processing
func (c ChatContext) Valid() bool {
	return (c.IsAddressed() || !c.Config.Bot.Addressed || c.IsPrivate()) && len(c.args) > 0
}

func (c ChatContext) IsPrivate() bool {
	return !strings.HasPrefix(c.event.Params[0], "#")
}

func (c ChatContext) GetCommand() string {
	return strings.ToLower(c.args[0])
}

// generateRequestID creates a unique 8-character request ID for correlation
func generateRequestID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}
