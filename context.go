package main

import (
	"context"
	"log"
	"strings"

	"github.com/lrstanley/girc"
	ai "github.com/sashabaranov/go-openai"
)

type ChatContext struct {
	context.Context
	AI *ai.Client
	//	Config  *Config
	Client  *girc.Client
	Event   *girc.Event
	Session *Session
	Args    []string
}

func (s *ChatContext) IsAddressed() bool {
	return strings.HasPrefix(s.Event.Last(), s.Client.GetNick())
}

func (c *ChatContext) IsAdmin() bool {
	admins := c.Session.Config.Admins
	nick := c.Event.Source.Name
	if len(admins) == 0 {
		return true
	}
	for _, user := range admins {
		if user == nick {
			return true
		}
	}
	return false
}

func (c *ChatContext) Stats() {
	log.Printf("session: messages %d, bytes %d, maxtokens %d, model %s",
		len(c.Session.GetHistory()),
		c.Session.Totalchars,
		c.Session.Config.MaxTokens,
		c.Session.Config.Model)
}

func (c *ChatContext) Reply(message string) *ChatContext {
	c.Client.Cmd.Reply(*c.Event, message)
	return c
}

func (c *ChatContext) Valid() bool {
	// check if the message is addressed to the bot or if being addressed is not required
	addressed := c.IsAddressed() || !c.Session.Config.Addressed
	hasArguments := len(c.Args) > 0

	// valid if:
	// - the message is either addressed to the bot or being addressed is not required
	// - or the message is private
	// - and at least one argument
	return (addressed || c.IsPrivate()) && hasArguments
}

func (c *ChatContext) IsPrivate() bool {
	return !strings.HasPrefix(c.Event.Params[0], "#")
}

func (c *ChatContext) GetCommand() string {
	return strings.ToLower(c.Args[0])
}

func NewChatContext(parentctx context.Context, config *Config, ircclient *girc.Client, e *girc.Event) (*ChatContext, context.CancelFunc) {
	timedctx, cancel := context.WithTimeout(parentctx, config.ClientTimeout)

	ctx := &ChatContext{
		Context: timedctx,
		AI:      NewAI(&config.OpenAI),
		Client:  ircclient,
		Event:   e,
		Args:    strings.Fields(e.Last()),
	}

	if ctx.IsAddressed() {
		ctx.Args = ctx.Args[1:]
	}

	if e.Source == nil {
		e.Source = &girc.Source{
			Name: config.Channel,
		}
	}

	key := e.Params[0]
	if !girc.IsValidChannel(key) {
		key = e.Source.Name
	}
	ctx.Session = Sessions.Get(key, config)
	return ctx, cancel
}
