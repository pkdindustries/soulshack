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
	AI      *ai.Client
	Client  *girc.Client
	Event   *girc.Event
	Session *Session
	Args    []string
}

func NewChatContext(parentctx context.Context, aiclient *ai.Client, ircclient *girc.Client, e *girc.Event) (*ChatContext, context.CancelFunc) {
	timedctx, cancel := context.WithTimeout(parentctx, BotConfig.ClientTimeout)

	ctx := &ChatContext{
		Context: timedctx,
		AI:      aiclient,
		Client:  ircclient,
		Event:   e,
		Args:    strings.Fields(e.Last()),
	}

	if ctx.IsAddressed() {
		ctx.Args = ctx.Args[1:]
	}

	if e.Source == nil {
		e.Source = &girc.Source{
			Name: BotConfig.Channel,
		}
	}

	key := e.Params[0]
	if !girc.IsValidChannel(key) {
		key = e.Source.Name
	}
	ctx.Session = BotConfig.Sessions.Get(key)
	return ctx, cancel
}

func (s *ChatContext) IsAddressed() bool {
	return strings.HasPrefix(s.Event.Last(), s.Client.GetNick())
}

func (c *ChatContext) IsAdmin() bool {

	hostmask := c.Event.Source.String()
	log.Println("checking hostmask:", hostmask)
	// XXX: if no admins are configured, all hostmasks are admins
	if len(BotConfig.Admins) == 0 {
		log.Println("all hostmasks are admin, please configure admins")
		return true
	}
	for _, user := range BotConfig.Admins {
		if user == hostmask {
			log.Println(hostmask, "is admin")
			return true
		}
	}
	return false
}

func (c *ChatContext) Reply(message string) *ChatContext {
	c.Client.Cmd.Reply(*c.Event, message)
	return c
}

// checks if the message is valid for processing
func (c *ChatContext) Valid() bool {
	return (c.IsAddressed() || !BotConfig.Addressed || c.IsPrivate()) && len(c.Args) > 0
}

func (c *ChatContext) IsPrivate() bool {
	return !strings.HasPrefix(c.Event.Params[0], "#")
}

func (c *ChatContext) GetCommand() string {
	return strings.ToLower(c.Args[0])
}
