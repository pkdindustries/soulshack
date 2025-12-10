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
type ChatContextInterface = core.ChatContextInterface

type ChatContext struct {
	context.Context
	Sys          core.System
	Session      sessions.Session
	Config       *config.Configuration
	client       *girc.Client
	event        *girc.Event
	args         []string
	logger       *zap.SugaredLogger
	requestID    string
	urlTriggered bool
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
	return &ctx, cancel
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

func (c ChatContext) Kick(channel, nick, reason string) bool {
	c.client.Cmd.Kick(channel, nick, reason)
	return true
}

func (c ChatContext) Topic(channel, topic string) bool {
	c.client.Cmd.Topic(channel, topic)
	return true
}

func (s ChatContext) IsAddressed() bool {
	return CheckAddressed(s.event.Last(), s.client.GetNick())
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

func (c ChatContext) GetBotNick() string {
	return c.client.GetNick()
}

func (c ChatContext) GetSource() string {
	return c.event.Source.Name
}

func (c ChatContext) IsAdmin() bool {
	hostmask := c.event.Source.String()
	c.logger.Debugw("admin_check", "hostmask", hostmask)
	isAdmin := CheckAdmin(hostmask, c.Config.Bot.Admins)
	if isAdmin && len(c.Config.Bot.Admins) == 0 {
		c.logger.Debugw("admin_check_warning")
	} else if isAdmin {
		c.logger.Debugw("admin_verified", "hostmask", hostmask)
	}
	return isAdmin
}

func (c ChatContext) Reply(message string) {
	c.client.Cmd.Reply(*c.event, message)

}

func (c ChatContext) SendAction(target, message string) {
	c.client.Cmd.Action(target, message)
}

func (c ChatContext) ReplyAction(message string) {
	target := c.event.Params[0]
	if !girc.IsValidChannel(target) {
		// For PMs, send a regular message instead of an action
		c.client.Cmd.Message(c.event.Source.Name, message)
		return
	}
	c.client.Cmd.Action(target, message)
}

func (c ChatContext) SetMode(target, flags string, args ...string) bool {
	c.client.Cmd.Mode(target, flags, args...)
	return true
}

func (c ChatContext) Ban(channel, target string) bool {
	c.client.Cmd.Ban(channel, target)
	return true
}

func (c ChatContext) Unban(channel, target string) bool {
	c.client.Cmd.Unban(channel, target)
	return true
}

func (c ChatContext) Invite(channel, nick string) bool {
	c.client.Cmd.Invite(channel, nick)
	return true
}

func (c ChatContext) GetUser(nick string) *core.UserInfo {
	user := c.client.LookupUser(nick)
	if user == nil {
		return nil
	}
	return &core.UserInfo{
		Nick:     user.Nick,
		Ident:    user.Ident,
		Host:     user.Host,
		RealName: user.Extras.Name,
		Account:  user.Extras.Account,
		Away:     user.Extras.Away,
		Channels: user.ChannelList,
	}
}

func (c ChatContext) GetChannel(name string) *core.ChannelInfo {
	ch := c.client.LookupChannel(name)
	if ch == nil {
		return nil
	}
	return &core.ChannelInfo{
		Name:  ch.Name,
		Modes: ch.Modes.String(),
		Topic: ch.Topic,
	}
}

func (c ChatContext) GetChannelUsers(channel string) []core.ChannelUser {
	ch := c.client.LookupChannel(channel)
	if ch == nil {
		return nil
	}

	client := c.client
	users := ch.Users(client)
	admins := ch.Admins(client)
	trusted := ch.Trusted(client)

	adminMap := make(map[string]bool)
	for _, admin := range admins {
		adminMap[admin.Nick] = true
	}
	trustedMap := make(map[string]bool)
	for _, tu := range trusted {
		trustedMap[tu.Nick] = true
	}

	var result []core.ChannelUser
	for _, user := range users {
		result = append(result, core.ChannelUser{
			Nick:    user.Nick,
			IsOp:    adminMap[user.Nick],
			IsVoice: trustedMap[user.Nick],
		})
	}
	return result
}

// checks if the message is valid for processing
func (c ChatContext) Valid() bool {
	return CheckValid(c.IsAddressed(), c.Config.Bot.Addressed, c.IsPrivate(), len(c.args))
}

func (c ChatContext) IsPrivate() bool {
	return CheckPrivate(c.event.Params[0])
}

func (c *ChatContext) SetURLTriggered(triggered bool) {
	c.urlTriggered = triggered
}

func (c ChatContext) IsURLTriggered() bool {
	return c.urlTriggered
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
