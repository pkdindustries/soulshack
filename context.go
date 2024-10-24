package main

import (
	"context"
	"log"
	"strings"

	"github.com/lrstanley/girc"
	ai "github.com/sashabaranov/go-openai"
)

type Message interface {
	IsAddressed() bool
	IsAdmin() bool
	Reply(string)
	Valid() bool
	IsPrivate() bool
	GetCommand() string
	GetSource() string
	GetArgs() []string
}

type Server interface {
	Join(string) bool
	Nick(string) bool
	Action(string, string) bool
	Mode(string, string, string) bool
	Kick(string, string, string) bool
	Topic(string, string) bool
	Oper(string, string) bool
}

type System interface {
	GetSession() Session
	GetConfig() *Configuration
}

type ChatContextInterface interface {
	context.Context
	System
	Message
	Server
}

type ChatContext struct {
	context.Context
	AI      *ai.Client
	Session Session
	Config  *Configuration
	client  *girc.Client
	event   *girc.Event
	args    []string
}

var _ ChatContextInterface = (*ChatContext)(nil)

func NewChatContext(parentctx context.Context, config *Configuration, ircclient *girc.Client, e *girc.Event) (ChatContextInterface, context.CancelFunc) {
	timedctx, cancel := context.WithTimeout(parentctx, config.API.Timeout)

	ctx := ChatContext{
		Context: timedctx,
		Config:  config,
		client:  ircclient,
		event:   e,
		args:    strings.Fields(e.Last()),
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
	ctx.Session = config.Store.Get(key)
	return ctx, cancel
}

func (c ChatContext) GetConfig() *Configuration {
	return c.Config
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

func (c ChatContext) GetSession() Session {
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
	log.Println("checking hostmask:", hostmask)
	// XXX: if no admins are configured, all hostmasks are admins
	if len(c.Config.Bot.Admins) == 0 {
		log.Println("all hostmasks are admin, please configure admins")
		return true
	}
	for _, user := range c.Config.Bot.Admins {
		if user == hostmask {
			log.Println(hostmask, "is admin")
			return true
		}
	}
	return false
}

func (c ChatContext) Reply(message string) {
	c.client.Cmd.Reply(*c.event, message)
}

func (c ChatContext) Action(target string, message string) bool {
	c.client.Cmd.Action(target, message)
	return true
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
