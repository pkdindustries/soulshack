package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/lrstanley/girc"
	ai "github.com/sashabaranov/go-openai"
	vip "github.com/spf13/viper"
)

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

func (s *IrcContext) IsAddressed() bool {
	return strings.HasPrefix(s.event.Last(), s.client.GetNick())
}

// ai
func (c *IrcContext) GetAI() *ai.Client {
	return c.ai
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

func startIrc(aiClient *ai.Client) {
	irc := girc.New(girc.Config{
		Server:    vip.GetString("server"),
		Port:      vip.GetInt("port"),
		Nick:      vip.GetString("nick"),
		User:      "soulshack",
		Name:      "soulshack",
		SSL:       vip.GetBool("ssl"),
		TLSConfig: &tls.Config{InsecureSkipVerify: true},
	})

	irc.Handlers.AddBg(girc.CONNECTED, func(c *girc.Client, e girc.Event) {
		ctx, cancel := NewIrcContext(context.Background(), aiClient, vip.GetViper(), c, &e)
		defer cancel()

		log.Println("joining channel:", ctx.config.Channel)
		c.Cmd.Join(ctx.config.Channel)

		time.Sleep(1 * time.Second)
		sendGreeting(ctx)
	})

	irc.Handlers.AddBg(girc.PRIVMSG, func(c *girc.Client, e girc.Event) {

		ctx, cancel := NewIrcContext(context.Background(), aiClient, vip.GetViper(), c, &e)
		defer cancel()

		if ctx.IsValid() {
			switch ctx.GetCommand() {
			case "/say":
				handleSay(ctx)
			case "/set":
				handleSet(ctx)
			case "/get":
				handleGet(ctx)
			case "/save":
				handleSave(ctx)
			case "/list":
				handleList(ctx)
			case "/become":
				handleBecome(ctx)
			case "/leave":
				handleLeave(ctx)
			case "/help":
				fallthrough
			case "/?":
				ctx.Reply("Supported commands: /set, /say [/as], /get, /list, /become, /leave, /help, /version")
			// case "/version":
			// 	ctx.Reply(r.Version)
			default:
				handleDefault(ctx)
			}
		}
	})

	for {
		log.Println("connecting to server:", vip.GetString("server"), "port:", vip.GetInt("port"), "ssl:", vip.GetBool("ssl"))
		if err := irc.Connect(); err != nil {
			log.Println(err)
			log.Println("reconnecting in 5 seconds...")
			time.Sleep(5 * time.Second)
		} else {
			return
		}
	}
}
