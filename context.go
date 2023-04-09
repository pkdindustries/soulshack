package main

import (
	"context"
	"strings"

	"github.com/lrstanley/girc"
	vip "github.com/spf13/viper"
)

type Personality struct {
	Prompt   string
	Greeting string
	Nick     string
	Model    string
	Goodbye  string
	Answer   string
}

type ChatContext struct {
	context.Context
	Personality *Personality
	Client      *girc.Client
	Event       *girc.Event
	Args        []string
	Session     *ChatSession
}

func NewFromViper(v *vip.Viper) *Personality {
	return &Personality{
		Prompt:   v.GetString("prompt"),
		Greeting: v.GetString("greeting"),
		Nick:     v.GetString("nick"),
		Model:    v.GetString("model"),
		Goodbye:  v.GetString("goodbye"),
		Answer:   v.GetString("answer"),
	}
}

// merge in the viper config
func (c *ChatContext) SetConfig(v *vip.Viper) {
	c.Personality = NewFromViper(v)
}

func (s *ChatContext) isAddressed() bool {
	return strings.HasPrefix(s.Event.Last(), s.Client.GetNick())
}

func (c *ChatContext) IsAdmin() bool {
	admins := vip.GetStringSlice("admins")
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

func (c *ChatContext) Reply(message string) *ChatContext {
	c.Client.Cmd.Reply(*c.Event, message)
	return c
}
func (c *ChatContext) Valid() bool {
	return (c.isAddressed() || c.isPrivate()) && len(c.Args) > 0
}
func (c *ChatContext) isPrivate() bool {
	return !strings.HasPrefix(c.Event.Params[0], "#")
}
func (c *ChatContext) GetCommand() string {
	return strings.ToLower(c.Args[0])
}

func createChatContext(parent context.Context, v *vip.Viper, c *girc.Client, e *girc.Event) (*ChatContext, context.CancelFunc) {
	timedctx, cancel := context.WithTimeout(parent, v.GetDuration("timeout"))

	ctx := &ChatContext{
		Context:     timedctx,
		Client:      c,
		Event:       e,
		Args:        strings.Fields(e.Last()),
		Personality: NewFromViper(v),
	}

	if ctx.isAddressed() {
		ctx.Args = ctx.Args[1:]
	}

	if e.Source == nil {
		e.Source = &girc.Source{
			Name: vip.GetString("channel"),
		}
	}

	key := e.Params[0]
	if !girc.IsValidChannel(key) {
		key = e.Source.Name
	}
	ctx.Session = sessions.Get(key)
	return ctx, cancel
}
