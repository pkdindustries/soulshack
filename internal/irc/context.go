package irc

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"strings"

	"github.com/lrstanley/girc"

	"pkdindustries/soulshack/internal/config"
	"pkdindustries/soulshack/internal/core"
)

// ChatContextInterface provides all context needed for handling IRC messages
type ChatContextInterface = core.ChatContextInterface

type ChatContext struct {
	context.Context
	Sys core.System
	// conversationKey is what identifies this event's conversation: the
	// channel for channel traffic, the source nick for anything else.
	conversationKey string
	client          *girc.Client
	event           *girc.Event
	args            []string
	logger          *slog.Logger
	fatalCh         chan<- error
}

var _ ChatContextInterface = (*ChatContext)(nil)

func NewChatContext(parentctx context.Context, system core.System, ircclient *girc.Client, e *girc.Event, fatalCh chan<- error) (ChatContextInterface, context.CancelFunc) {
	// One snapshot for the request: its deadline and its defaults come from
	// the settings in force now.
	settings := system.GetConfig()
	timedctx, cancel := context.WithTimeout(parentctx, settings.API.Timeout)

	// Generate a unique request ID for correlation
	requestID := generateRequestID()

	// Ensure Source is not nil for events like CONNECTED
	if e.Source == nil {
		e.Source = &girc.Source{
			Name: settings.Server.Channel,
		}
	}

	// Get channel safely
	channel := settings.Server.Channel
	if len(e.Params) > 0 {
		channel = e.Params[0]
	}

	ctx := ChatContext{
		Context: timedctx,
		Sys:     system,
		client:  ircclient,
		event:   e,
		args:    strings.Fields(e.Last()),
		fatalCh: fatalCh,
		logger: slog.Default().With(
			"request_id", requestID,
			"channel", channel,
			"source", e.Source.Name,
		),
	}

	if ctx.IsAddressed() {
		ctx.args = ctx.args[1:]
	}

	key := channel
	if !girc.IsValidChannel(key) {
		key = e.Source.Name
	}

	ctx.conversationKey = key
	return &ctx, cancel
}

// Value answers the lookup IRC tools use to find their chat context. A turn's
// context is the chat context, so the lookup resolves by itself: no layer has
// to inject a value into tool calls.
func (c *ChatContext) Value(key any) any {
	if k, ok := key.(contextKey); ok && k == kContextKey {
		return ChatContextInterface(c)
	}
	return c.Context.Value(key)
}

func (c ChatContext) GetSystem() core.System {
	return c.Sys
}

func (c ChatContext) GetConfig() *config.Configuration {
	return c.Sys.GetConfig()
}

func (c ChatContext) GetLogger() *slog.Logger {
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

func (c ChatContext) JoinWithKey(channel, key string) bool {
	c.client.Cmd.Join(channel, key)
	return true
}

func (c ChatContext) FatalError(err error) {
	select {
	case c.fatalCh <- err:
	default:
	}
	c.client.Close()
}

func (c ChatContext) GetArgs() []string {
	return c.args
}

func (c ChatContext) GetBotNick() string {
	return c.client.GetNick()
}

func (c ChatContext) GetServerOption(key string) (string, bool) {
	return c.client.GetServerOption(key)
}

func (c ChatContext) GetSource() string {
	return c.event.Source.Name
}

func (c ChatContext) IsAdmin() bool {
	hostmask := c.event.Source.String()
	c.logger.Debug("admin_check", "hostmask", hostmask)
	admins := c.GetConfig().Bot.Admins
	isAdmin := CheckAdmin(hostmask, admins)
	if isAdmin && len(admins) == 0 {
		c.logger.Debug("admin_check_warning")
	} else if isAdmin {
		c.logger.Debug("admin_verified", "hostmask", hostmask)
	}
	return isAdmin
}

func (c ChatContext) Reply(message string) {
	c.client.Cmd.Reply(*c.event, message)

}

// NewChunkWriter frames model output into IRC-sized messages.
func (c ChatContext) NewChunkWriter(output chan<- string) core.ChunkWriter {
	return NewChunker(output, c.chunkSize())
}

var _ core.ChunkWriter = (*Chunker)(nil)

// defaultChunkSize applies when chunkmax is disabled.
const defaultChunkSize = 400

func (c ChatContext) chunkSize() int {
	if size := c.GetConfig().Session.ChunkMax; size > 0 {
		return size
	}
	return defaultChunkSize
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

func (c ChatContext) GetConversationKey() string {
	return c.conversationKey
}

func (c ChatContext) IsPrivate() bool {
	if len(c.event.Params) == 0 {
		return false
	}
	return CheckPrivate(c.event.Params[0])
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
