package main

import (
	"context"
	"strings"

	"github.com/lrstanley/girc"
	vip "github.com/spf13/viper"
)

type ChatContext struct {
	context.Context
	Cfg     *vip.Viper
	Client  *girc.Client
	Event   *girc.Event
	Args    []string
	Session *ChatSession
}

func createChatContext(v *vip.Viper, c *girc.Client, e *girc.Event) (*ChatContext, context.CancelFunc) {
	timedctx, cancel := context.WithTimeout(context.Background(), v.GetDuration("timeout"))

	ctx := &ChatContext{
		Context: timedctx,
		Client:  c,
		Event:   e,
		Args:    strings.Fields(e.Last()),
		Cfg:     v,
	}

	if ctx.isAddressed() {
		ctx.Args = ctx.Args[1:]
	}

	if e.Source == nil {
		e.Source = &girc.Source{
			Name: ctx.Cfg.GetString("channel"),
		}
	}

	key := e.Params[0]
	if !girc.IsValidChannel(key) {
		key = e.Source.Name
	}
	ctx.Session = sessions.Get(key)
	return ctx, cancel
}

// merge in the viper config
func (c *ChatContext) MergeConfig(v *vip.Viper) {
	c.Cfg.MergeConfigMap(v.AllSettings())
}

func (c *ChatContext) GetConfig() *vip.Viper {
	return c.Cfg
}
func (s *ChatContext) isAddressed() bool {
	return strings.HasPrefix(s.Event.Last(), s.Client.GetNick())
}
func (c *ChatContext) Reply(message string) *ChatContext {
	c.Client.Cmd.Reply(*c.Event, message)
	return c
}
func (c *ChatContext) isValid() bool {
	return (c.isAddressed() || c.isPrivate()) && len(c.Args) > 0
}
func (c *ChatContext) isPrivate() bool {
	return !strings.HasPrefix(c.Event.Params[0], "#")
}
func (c *ChatContext) getCommand() string {
	return strings.ToLower(c.Args[0])
}
