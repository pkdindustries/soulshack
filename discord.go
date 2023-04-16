package main

import (
	"context"
	"fmt"
	"log"
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

// reply
func (c *DiscordContext) Reply(message string) {
	c.discord.ChannelMessageSend(c.msg.ChannelID, message)
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

func startDiscord(a *ai.Client) {

	dg, err := discordgo.New("Bot " + vip.GetString("discordtoken"))

	//	dg, err := discordgo.New("Bot ODYyNTQ4NDkwNDQxNzE5ODA4.YOZ84Q.TA9toCRKlgieivShb4Z5IrcQsSY")
	if err != nil {
		log.Println("Error creating Discord session: ", err)
		return
	}

	dg.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		if m.Author.ID == s.State.User.ID {
			return
		}
		ctx, cancel := NewDiscordContext(context.Background(), a, vip.GetViper(), m, s)
		defer cancel()
		ctx.discord.ChannelTyping(m.ChannelID)
		handleMessage(ctx)
	})

	dg.Identify.Intents = discordgo.IntentsGuildMessages

	err = dg.Open()
	if err != nil {
		log.Fatal("Error opening Discord session: ", err)
	}

	fmt.Println("Discord bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	dg.Close()
}

// func onMessage(s *discordgo.Session, m *discordgo.MessageCreate) {

// 	// If the message is "ping" reply with "Pong!"
// 	// if m.Content == "ping" {
// 	// 	embed := &discordgo.MessageEmbed{
// 	// 		Title:       "PONG!!",
// 	// 		URL:         "http://image5.sixthtone.com/image/5/41/132.jpg",
// 	// 		Description: "Embed Description",
// 	// 		Color:       0x78141b,
// 	// 		Fields: []*discordgo.MessageEmbedField{
// 	// 			{
// 	// 				Name:   "Inline field 1 title",
// 	// 				Value:  "value 1",
// 	// 				Inline: true,
// 	// 			},
// 	// 		},
// 	// 	}
// 	// 	s.ChannelMessageSendEmbed(m.ChannelID, embed)
// 	// }

// 	log.Println(m.Content)
// 	//	s.ChannelMessageSend(m.ChannelID, "pong!")

// }
