package main

import (
	"bytes"
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	ai "github.com/sashabaranov/go-openai"
	vip "github.com/spf13/viper"
)

type DiscordContext struct {
	context.Context
	ai          *ai.Client
	personality *Personality
	session     *Sessions
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

func Discord(aiclient *ai.Client) {

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

func (c *DiscordContext) Complete(msg string) {
	log.Println("DiscordContext.Complete", msg)
	session := c.GetSession()
	personality := c.GetPersonality()

	c.discord.ChannelTyping(c.msg.ChannelID)

	session.AddMessage(c.personality, ai.ChatMessageRoleUser, msg)
	respch := ChatCompletionStreamTask(c, &CompletionRequest{
		Client:    c.GetAI(),
		Timeout:   session.Config.ClientTimeout,
		Model:     personality.Model,
		Temp:      personality.Temp,
		MaxTokens: session.Config.MaxTokens,
		Messages:  session.GetHistory(),
	})

	chunker := &Chunker{
		buffer: &bytes.Buffer{},
		delay:  0,
		max:    9999,
		quote:  false,
		last:   time.Now(),
	}
	chunkch := chunker.ChannelFilter(respch)

	typer := time.NewTicker(8 * time.Second)
	donetyping := make(chan struct{})
	defer typer.Stop()
	defer close(donetyping)
	go func() {
		for {
			select {
			case <-typer.C:
				c.discord.ChannelTyping(c.msg.ChannelID)
			case <-donetyping:
				return
			}
		}
	}()

	all := strings.Builder{}
	sent := ""
	for chunk := range chunkch {
		all.WriteString(chunk)
		if sent == "" {
			messageID, err := c.initmessage(chunk)
			if err == nil {
				sent = messageID
			}
		} else {
			msg := all.String()
			quotes := strings.Count(msg, "```")
			if quotes%2 != 0 {
				msg += "```"
			}
			if len(msg) > 2000 {
				all.Reset()
				all.WriteString(chunk)
				messageID, err := c.initmessage(chunk)
				if err == nil {
					sent = messageID
				}
			} else {
				c.editmessage(sent, msg)
			}
		}
	}

	session.AddMessage(c.personality, ai.ChatMessageRoleAssistant, all.String())
	ReactActionObservation(c, all.String())
}

func (c *DiscordContext) initmessage(message string) (string, error) {
	if strings.TrimSpace(message) == "" {
		return "", errors.New("empty message")
	}

	sentMessage, err := c.discord.ChannelMessageSend(c.msg.ChannelID, message)
	if err != nil {
		log.Println(err)
		return "", err
	}

	return sentMessage.ID, nil
}

func (c *DiscordContext) editmessage(messageID, content string) {
	if strings.TrimSpace(content) == "" {
		return
	}
	_, err := c.discord.ChannelMessageEdit(c.msg.ChannelID, messageID, content)
	if err != nil {
		log.Println(err)
	}
}

func (c *DiscordContext) Sendmessage(message string) {
	if strings.TrimSpace(message) == "" {
		return
	}
	_, err := c.discord.ChannelMessageSend(c.msg.ChannelID, message)
	if err != nil {
		log.Println(err)
	}
}

// changename
func (c *DiscordContext) ChangeName(name string) error {
	err := c.discord.GuildMemberNickname(c.msg.GuildID, c.discord.State.User.ID, name)
	return err
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

// resetsource
func (c *DiscordContext) ResetSource() {
}

// getsession
func (c *DiscordContext) GetSession() *Sessions {
	return c.session
}

// setsession
func (c *DiscordContext) SetSession(s *Sessions) {
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
