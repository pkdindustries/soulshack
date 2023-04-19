package main

import (
	"context"
	"log"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/bwmarrin/discordgo"
	ai "github.com/sashabaranov/go-openai"
	vip "github.com/spf13/viper"
)

type DiscordContext struct {
	context.Context
	ai          *ai.Client
	personality *Personality
	session     *Session
	msg         *discordgo.MessageCreate
	discord     *discordgo.Session
	config      *DiscordConfig
}

type DiscordConfig struct {
	Token string
}

func DiscordFromViper(v *vip.Viper) *DiscordConfig {
	return &DiscordConfig{
		Token: v.GetString("discordtoken"),
	}
}

func NewDiscordContext(parent context.Context, ai *ai.Client, v *vip.Viper, m *discordgo.MessageCreate, s *discordgo.Session) (*DiscordContext, context.CancelFunc) {
	timedctx, cancel := context.WithTimeout(parent, v.GetDuration("timeout"))

	ctx := &DiscordContext{
		Context:     timedctx,
		ai:          ai,
		personality: PersonalityFromViper(v),
		msg:         m,
		discord:     s,
		session:     sessions.Get(m.ChannelID),
		config:      DiscordFromViper(v),
	}

	return ctx, cancel
}

func startDiscord(aiclient *ai.Client) {

	dg, err := discordgo.New("Bot " + vip.GetString("discordtoken"))
	if err != nil {
		log.Println("Error creating Discord session: ", err)
		return
	}

	dg.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		if m.Author.ID == s.State.User.ID {
			return
		}
		ctx, cancel := NewDiscordContext(context.Background(), aiclient, vip.GetViper(), m, s)
		defer cancel()

		ctx.GetSession().Config.Chunkquoted = true
		ctx.GetSession().Config.Chunkmax = 2000
		ctx.discord.ChannelTyping(m.ChannelID)
		handleMessage(ctx)
	})

	dg.Identify.Intents = discordgo.IntentsGuildMessages

	err = dg.Open()
	defer dg.Close()

	if err != nil {
		log.Fatal("Error opening Discord session: ", err)
	}

	log.Println("discord bot server is now running")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
}

// getcommand
func (c *DiscordContext) GetCommand() string {
	return c.GetArgs()[0]
}

func (c *DiscordContext) IsAdmin() bool {
	return true
}

func (c *DiscordContext) IsAddressed() bool {
	return true
}

// is valid
func (c *DiscordContext) IsValid() bool {
	return true
}

func (c *DiscordContext) Reply(message string) {

	if strings.TrimSpace(message) == "" {
		return
	}

	_, err := c.discord.ChannelMessageSend(c.msg.ChannelID, message)
	if err != nil {
		log.Println(err)
	}
}

// resetsource
func (c *DiscordContext) ResetSource() {
}

// changename
func (c *DiscordContext) ChangeName(name string) {
}

// getsession
func (c *DiscordContext) GetSession() *Session {
	return c.session
}

// setsession
func (c *DiscordContext) SetSession(s *Session) {
	c.session = s
}

// get personality
func (c *DiscordContext) GetPersonality() *Personality {
	return c.personality
}

// get args
func (c *DiscordContext) GetArgs() []string {
	return strings.Split(c.msg.Content, " ")
}

// set args
func (c *DiscordContext) SetArgs(args []string) {
	c.msg.Content = strings.Join(args, " ")
}

// ai
func (c *DiscordContext) GetAI() *ai.Client {
	return c.ai
}

func isURL(str string) bool {
	if !strings.HasPrefix(str, "http") {
		return false
	}
	_, err := url.Parse(str)
	return err == nil
}
