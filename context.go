package main

import (
	"context"
	"strings"

	"github.com/lrstanley/girc"
	vip "github.com/spf13/viper"
)

type chatContext struct {
	context.Context
	Client  *girc.Client
	Event   *girc.Event
	Args    []string
	Session *chatSession
}

func (s *chatContext) isAddressed() bool {
	return strings.HasPrefix(s.Event.Last(), s.Client.GetNick())
}
func createChatContext(c *girc.Client, e *girc.Event) (*chatContext, context.CancelFunc) {
	timedctx, cancel := context.WithTimeout(context.Background(), vip.GetDuration("timeout"))

	ctx := &chatContext{
		Context: timedctx,
		Client:  c,
		Event:   e,
		Args:    strings.Fields(e.Last()),
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

func (c *chatContext) Reply(message string) *chatContext {
	c.Client.Cmd.Reply(*c.Event, message)
	return c
}
func (c *chatContext) isValid() bool {
	return (c.isAddressed() || c.isPrivate()) && len(c.Args) > 0
}

func (c *chatContext) isPrivate() bool {
	return !strings.HasPrefix(c.Event.Params[0], "#")
}

func (c *chatContext) getCommand() string {
	return strings.ToLower(c.Args[0])
}
