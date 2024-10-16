package main

import (
	"context"
	"log"
	"strings"

	"github.com/lrstanley/girc"
	ai "github.com/sashabaranov/go-openai"
)

// ChatContextInterface defines the methods needed for interaction within the chat context.
type ChatContextInterface interface {
	context.Context
	GetSession() *Session
	GetClient() *girc.Client
	GetAI() *ai.Client
	IsAddressed() bool
	IsAdmin() bool
	Reply(string)
	Valid() bool
	IsPrivate() bool
	GetCommand() string
	GetEvent() *girc.Event
	GetSource() string
	GetArgs() []string
	Join(string) bool
	Part(string) bool
	Nick(string) bool
	Action(string, string)
}

type ChatContext struct {
	context.Context
	AI      *ai.Client
	Client  *girc.Client
	Event   *girc.Event
	Session *Session
	Args    []string
}

var _ ChatContextInterface = (*ChatContext)(nil)

func NewChatContext(parentctx context.Context, aiclient *ai.Client, ircclient *girc.Client, e *girc.Event) (ChatContextInterface, context.CancelFunc) {
	timedctx, cancel := context.WithTimeout(parentctx, Config.ClientTimeout)

	ctx := ChatContext{
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
			Name: Config.Channel,
		}
	}

	key := e.Params[0]
	if !girc.IsValidChannel(key) {
		key = e.Source.Name
	}
	ctx.Session = Config.Sessions.Get(key)
	return ctx, cancel
}

func (s ChatContext) IsAddressed() bool {
	return strings.HasPrefix(s.Event.Last(), s.Client.GetNick())
}

func (c ChatContext) Nick(nickname string) bool {
	c.Client.Cmd.Nick(nickname)
	return true
}

func (c ChatContext) Join(channel string) bool {
	c.Client.Cmd.Join(channel)
	return true
}

func (c ChatContext) Part(channel string) bool {
	c.Client.Cmd.Part(channel)
	return true
}

func (c ChatContext) GetArgs() []string {
	return c.Args
}

func (c ChatContext) GetSession() *Session {
	return c.Session
}

func (c ChatContext) GetClient() *girc.Client {
	return c.Client
}

func (c ChatContext) GetEvent() *girc.Event {
	return c.Event
}

func (c ChatContext) GetSource() string {
	return c.Event.Source.Name
}

func (c ChatContext) GetAI() *ai.Client {
	return c.AI
}

func (c ChatContext) IsAdmin() bool {
	hostmask := c.Event.Source.String()
	log.Println("checking hostmask:", hostmask)
	// XXX: if no admins are configured, all hostmasks are admins
	if len(Config.Admins) == 0 {
		log.Println("all hostmasks are admin, please configure admins")
		return true
	}
	for _, user := range Config.Admins {
		if user == hostmask {
			log.Println(hostmask, "is admin")
			return true
		}
	}
	return false
}

func (c ChatContext) Reply(message string) {
	c.Client.Cmd.Reply(*c.Event, message)
}

func (c ChatContext) Action(target string, message string) {
	c.Client.Cmd.Action(target, message)
}

// checks if the message is valid for processing
func (c ChatContext) Valid() bool {
	return (c.IsAddressed() || !Config.Addressed || c.IsPrivate()) && len(c.Args) > 0
}

func (c ChatContext) IsPrivate() bool {
	return !strings.HasPrefix(c.Event.Params[0], "#")
}

func (c ChatContext) GetCommand() string {
	return strings.ToLower(c.Args[0])
}
