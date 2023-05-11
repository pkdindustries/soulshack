package discord

import (
	"bytes"
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	action "pkdindustries/soulshack/action"
	completion "pkdindustries/soulshack/completion"
	handler "pkdindustries/soulshack/handler"
	model "pkdindustries/soulshack/model"
	session "pkdindustries/soulshack/session"
	"strings"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	vip "github.com/spf13/viper"
)

const DISCORD_MAX_MSG = 2000

type DiscordContext struct {
	context.Context
	session *session.Session
	msg     *discordgo.MessageCreate
	discord *discordgo.Session
	config  *DiscordConfig
}

type DiscordConfig struct {
	Token string
}

func DiscordFromViper(v *vip.Viper) *DiscordConfig {
	return &DiscordConfig{
		Token: v.GetString("discordtoken"),
	}
}

func NewDiscordContext(parent context.Context, v *vip.Viper, m *discordgo.MessageCreate, s *discordgo.Session) (*DiscordContext, context.CancelFunc) {
	timedctx, cancel := context.WithTimeout(parent, v.GetDuration("timeout"))

	ctx := &DiscordContext{
		Context: timedctx,
		msg:     m,
		discord: s,
		session: session.SessionStore.Get(m.ChannelID),
		config:  DiscordFromViper(v),
	}

	return ctx, cancel
}

func Discord() {

	dg, err := discordgo.New("Bot " + vip.GetString("discordtoken"))
	if err != nil {
		log.Println("Error creating Discord session: ", err)
		return
	}

	dg.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		if m.Author.ID == s.State.User.ID {
			return
		}
		ctx, cancel := NewDiscordContext(context.Background(), vip.GetViper(), m, s)
		defer cancel()
		handler.HandleMessage(ctx)
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
	s := c.session
	p := c.GetPersonality()
	c.discord.ChannelTyping(c.msg.ChannelID)
	s.AddMessage(session.RoleUser, msg)

	respch := completion.ChatCompletionStreamTask(c, &completion.CompletionRequest{
		Client:    completion.GetAI(),
		Timeout:   s.Config.ClientTimeout,
		Model:     p.Model,
		Temp:      p.Temp,
		MaxTokens: s.Config.MaxTokens,
		Messages:  s.GetHistory(),
	})

	chunker := &completion.Chunker{
		Buffer: &bytes.Buffer{},
		Delay:  0,
		Max:    DISCORD_MAX_MSG,
		Quote:  false,
		Last:   time.Now(),
	}
	chunkch := chunker.Filter(respch)

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
			case <-c.Done():
				return
			}
		}
	}()

	all := strings.Builder{}
	sentmsgid := ""
	for chunk := range chunkch {
		all.WriteString(chunk)
		if sentmsgid == "" {
			messageID, err := c.initmessage(chunk)
			if err == nil {
				sentmsgid = messageID
			}
		} else {
			msg := all.String()
			quotes := strings.Count(msg, "```")
			if quotes%2 != 0 {
				msg += "```"
			}
			if len(msg) > DISCORD_MAX_MSG {
				all.Reset()
				all.WriteString(chunk)
				messageID, err := c.initmessage(chunk)
				if err == nil {
					sentmsgid = messageID
				}
			} else {
				c.editmessage(sentmsgid, msg)
			}
		}
	}

	s.AddMessage(session.RoleAssistant, all.String())
	if s.Config.ReactMode {
		action.ReactObservation(c, all.String())
	}
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

func (c *DiscordContext) Send(message string) {
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

func (c *DiscordContext) ResetSession() {
	c.session.Reset()
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

// get personality
func (c *DiscordContext) GetPersonality() *model.Personality {
	return &c.session.Config.Personality
}

// get args
func (c *DiscordContext) GetArgs() []string {
	return strings.Split(c.msg.Content, " ")
}

// set args
func (c *DiscordContext) SetArgs(args []string) {
	c.msg.Content = strings.Join(args, " ")
}
