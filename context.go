package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/lrstanley/girc"
	ai "github.com/sashabaranov/go-openai"
	vip "github.com/spf13/viper"
)

type ChatContext interface {
	context.Context
	IsAdmin() bool
	Reply(message string)
	ResetSource()
	ChangeName(nick string)
	GetSession() *Session
	SetSession(*Session)
	GetPersonality() *Personality
	GetArgs() []string
	SetArgs([]string)
	GetAI() *ai.Client
}

type Personality struct {
	Prompt   string
	Greeting string
	Nick     string
	Model    string
	Goodbye  string
}

type IrcConfig struct {
	Channel   string
	Admins    []string
	Directory string
	Verbose   bool
	Server    string
	Port      int
	SSL       bool
	Addressed bool
}

type IrcContext struct {
	context.Context
	ai          *ai.Client
	personality *Personality
	config      *IrcConfig
	client      *girc.Client
	event       *girc.Event
	session     *Session
	args        []string
}

func PersonalityFromViper(v *vip.Viper) *Personality {
	return &Personality{
		Prompt:   v.GetString("prompt"),
		Greeting: v.GetString("greeting"),
		Nick:     v.GetString("nick"),
		Model:    v.GetString("model"),
		Goodbye:  v.GetString("goodbye"),
	}
}

func IrcFromViper(v *vip.Viper) *IrcConfig {
	return &IrcConfig{
		Channel:   v.GetString("channel"),
		Admins:    v.GetStringSlice("admins"),
		Directory: v.GetString("directory"),
		Verbose:   v.GetBool("verbose"),
		Server:    v.GetString("server"),
		Port:      v.GetInt("port"),
		SSL:       v.GetBool("ssl"),
		Addressed: v.GetBool("addressed"),
	}
}

// merge in the viper config
func (c *Personality) SetConfig(v *vip.Viper) {
	c.Prompt = v.GetString("prompt")
	c.Greeting = v.GetString("greeting")
	c.Nick = v.GetString("nick")
	c.Model = v.GetString("model")
	c.Goodbye = v.GetString("goodbye")
}

func (s *IrcContext) IsAddressed() bool {
	return strings.HasPrefix(s.event.Last(), s.client.GetNick())
}

// ai
func (c *IrcContext) GetAI() *ai.Client {
	return c.ai
}

func (c *IrcContext) IsAdmin() bool {
	admins := vip.GetStringSlice("admins")
	nick := c.event.Source.Name
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

func (c *IrcContext) ChangeName(nick string) {
	c.client.Cmd.Nick(nick)
}

func (c *IrcContext) Stats() string {
	return fmt.Sprintf("session: messages %d, bytes %d, maxtokens %d, model %s",
		len(c.session.GetHistory()),
		c.session.Totalchars,
		c.session.Config.MaxTokens,
		c.personality.Model)
}

func (c *IrcContext) Reply(message string) {
	c.client.Cmd.Reply(*c.event, message)
}

func (c *IrcContext) ResetSource() {
	c.event.Params[0] = c.config.Channel
	c.event.Source.Name = c.personality.Nick
}

func (c *IrcContext) IsValid() bool {
	// check if the message is addressed to the bot or if being addressed is not required
	addressed := c.IsAddressed() || !c.config.Addressed
	hasArguments := len(c.args) > 0

	// valid if:
	// - the message is either addressed to the bot or being addressed is not required
	// - or the message is private
	// - and at least one argument
	return (addressed || c.IsPrivate()) && hasArguments
}

func (c *IrcContext) IsPrivate() bool {
	return !strings.HasPrefix(c.event.Params[0], "#")
}

func (c *IrcContext) GetCommand() string {
	return strings.ToLower(c.args[0])
}

func (c *IrcContext) GetMessage() string {
	return c.event.Last()
}

func (c *IrcContext) GetArgument() string {
	return strings.Join(c.args[1:], " ")
}

func (c *IrcContext) GetArgs() []string {
	return c.args
}

// set args
func (c *IrcContext) SetArgs(args []string) {
	c.args = args
}

func (c *IrcContext) SetNick(nick string) {
	log.Printf("changing nick to %s", nick)
	c.personality.Nick = nick
	c.client.Cmd.Nick(c.personality.Nick)
}

func (c *IrcContext) GetSession() *Session {
	return c.session
}
func (c *IrcContext) SetSession(s *Session) {
	c.session = s
}

func (c *IrcContext) GetPersonality() *Personality {
	return c.personality
}

// func spoolFromChannel(ctx *IrcContext, msgch <-chan *string) *string {
// 	all := strings.Builder{}
// 	for reply := range msgch {
// 		all.WriteString(*reply)
// 		log.Printf("<< <%s> %s", ctx.personality.Nick, *reply)
// 		ctx.Reply(*reply)
// 	}
// 	s := all.String()
// 	return &s
// }

func NewIrcContext(parent context.Context, ai *ai.Client, v *vip.Viper, c *girc.Client, e *girc.Event) (*IrcContext, context.CancelFunc) {
	timedctx, cancel := context.WithTimeout(parent, v.GetDuration("timeout"))

	ctx := &IrcContext{
		Context:     timedctx,
		ai:          ai,
		client:      c,
		event:       e,
		args:        strings.Fields(e.Last()),
		personality: PersonalityFromViper(v),
		config:      IrcFromViper(v),
	}

	if ctx.IsAddressed() {
		ctx.args = ctx.args[1:]
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
	ctx.session = sessions.Get(key)
	return ctx, cancel
}
