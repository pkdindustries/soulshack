package main

import (
	"context"
	"log"
	"strings"

	"github.com/lrstanley/girc"
	ai "github.com/sashabaranov/go-openai"
	vip "github.com/spf13/viper"
)

type Personality struct {
	Prompt   string
	Greeting string
	Nick     string
	Model    string
	Goodbye  string
}

type Config struct {
	Channel   string
	Nick      string
	Admins    []string
	Directory string
	Verbose   bool
	Server    string
	Port      int
	SSL       bool
	Addressed bool
}

type ChatContext struct {
	context.Context
	AI          *ai.Client
	Personality *Personality
	Config      *Config
	Client      *girc.Client
	Event       *girc.Event
	Session     *ChatSession
	Args        []string
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

func IrcFromViper(v *vip.Viper) *Config {
	return &Config{
		Channel:   v.GetString("channel"),
		Nick:      v.GetString("nick"),
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
func (c *ChatContext) SetConfig(v *vip.Viper) {
	c.Personality = PersonalityFromViper(v)
}

func (s *ChatContext) IsAddressed() bool {
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

func (c *ChatContext) Stats() {
	log.Printf("session: messages %d, bytes %d, maxtokens %d, model %s",
		len(c.Session.GetHistory()),
		c.Session.Totalchars,
		c.Session.Config.MaxTokens,
		c.Personality.Model)
}

func (c *ChatContext) Reply(message string) *ChatContext {
	c.Client.Cmd.Reply(*c.Event, message)
	return c
}

func (c *ChatContext) Valid() bool {
	// check if the message is addressed to the bot or if being addressed is not required
	addressed := c.IsAddressed() || !c.Config.Addressed
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

func CreateChatContext(parent context.Context, ai *ai.Client, v *vip.Viper, c *girc.Client, e *girc.Event) (*ChatContext, context.CancelFunc) {
	timedctx, cancel := context.WithTimeout(parent, v.GetDuration("timeout"))

	ctx := &ChatContext{
		Context:     timedctx,
		AI:          ai,
		Client:      c,
		Event:       e,
		Args:        strings.Fields(e.Last()),
		Personality: PersonalityFromViper(v),
		Config:      IrcFromViper(v),
	}

	if ctx.IsAddressed() {
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
