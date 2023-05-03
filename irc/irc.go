package irc

import (
	"bytes"
	"context"
	"crypto/tls"
	"log"
	"pkdindustries/soulshack/action"
	"pkdindustries/soulshack/completion"
	handler "pkdindustries/soulshack/handler"
	model "pkdindustries/soulshack/model"
	session "pkdindustries/soulshack/session"
	"strings"
	"time"

	"github.com/lrstanley/girc"
	ai "github.com/sashabaranov/go-openai"
	vip "github.com/spf13/viper"
)

type IrcConfig struct {
	Channel   string
	Admins    []string
	Server    string
	Port      int
	SSL       bool
	Addressed bool
}

func IrcFromViper(v *vip.Viper) *IrcConfig {
	return &IrcConfig{
		Channel:   v.GetString("channel"),
		Admins:    v.GetStringSlice("admins"),
		Server:    v.GetString("server"),
		Port:      v.GetInt("port"),
		SSL:       v.GetBool("ssl"),
		Addressed: v.GetBool("addressed"),
	}
}

type IrcContext struct {
	context.Context
	ai          *ai.Client
	personality *model.Personality
	config      *IrcConfig
	client      *girc.Client
	event       *girc.Event
	session     *session.Sessions
	args        []string
}

func NewIrcContext(parent context.Context, v *vip.Viper, c *girc.Client, e *girc.Event) (*IrcContext, context.CancelFunc) {
	timedctx, cancel := context.WithTimeout(parent, v.GetDuration("timeout"))

	ctx := &IrcContext{
		Context:     timedctx,
		client:      c,
		event:       e,
		args:        strings.Fields(e.Last()),
		personality: model.PersonalityFromViper(v),
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
	ctx.session = session.SessionStore.Get(key)
	return ctx, cancel
}

func Irc() {
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
		ctx, cancel := NewIrcContext(context.Background(), vip.GetViper(), c, &e)
		defer cancel()

		log.Println("joining channel:", ctx.config.Channel)
		c.Cmd.Join(ctx.config.Channel)

		time.Sleep(1 * time.Second)
		handler.SendGreeting(ctx)
	})

	irc.Handlers.AddBg(girc.PRIVMSG, func(c *girc.Client, e girc.Event) {
		ctx, cancel := NewIrcContext(context.Background(), vip.GetViper(), c, &e)
		defer cancel()
		handler.HandleMessage(ctx)
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

func (c *IrcContext) Complete(msg string) {
	session := c.GetSession()
	personality := c.GetPersonality()
	session.AddMessage(personality, ai.ChatMessageRoleUser, msg)

	respch := completion.ChatCompletionStreamTask(c, &completion.CompletionRequest{
		Client:    completion.GetAI(),
		Timeout:   session.Config.ClientTimeout,
		Model:     personality.Model,
		MaxTokens: session.Config.MaxTokens,
		Messages:  session.GetHistory(),
		Temp:      personality.Temp,
	})

	chunker := &completion.Chunker{
		Buffer: &bytes.Buffer{},
		Max:    session.Config.Chunkmax,
		Delay:  session.Config.Chunkdelay,
		Quote:  session.Config.Chunkquoted,
		Last:   time.Now(),
	}

	chunkch := chunker.ChannelFilter(respch)

	all := strings.Builder{}
	for reply := range chunkch {
		all.WriteString(reply)
		c.Sendmessage(reply)
	}

	session.AddMessage(c.personality, ai.ChatMessageRoleAssistant, all.String())
	action.ReactActionObservation(c, all.String())
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

func (c *IrcContext) ChangeName(nick string) error {
	c.client.Cmd.Nick(nick)
	return nil
}

func (c *IrcContext) Sendmessage(message string) {
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

func (c *IrcContext) GetSession() *session.Sessions {
	return c.session
}
func (c *IrcContext) SetSession(s *session.Sessions) {
	c.session = s
}

func (c *IrcContext) GetPersonality() *model.Personality {
	return c.personality
}
