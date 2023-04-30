package main

import (
	"context"
	"time"

	ai "github.com/sashabaranov/go-openai"
	vip "github.com/spf13/viper"
)

type Message interface {
	IsAdmin() bool
	IsValid() bool
	IsAddressed() bool
	ResetSource() // xxx
	GetArgs() []string
	SetArgs([]string)
	Sendmessage(string)
}

type Session interface {
	ChangeName(string) error
	GetSession() *Sessions
	SetSession(*Sessions)
	GetPersonality() *Personality
	GetAI() *ai.Client
}

type ChatContext interface {
	context.Context
	// msg
	Message
	Session
	Complete(string)
}

type Personality struct {
	Prompt   string
	Greeting string
	Nick     string
	Model    string
	Temp     float64
}

func PersonalityFromViper(v *vip.Viper) *Personality {
	return &Personality{
		Prompt:   v.GetString("prompt"),
		Greeting: v.GetString("greeting"),
		Nick:     v.GetString("nick"),
		Model:    v.GetString("model"),
		Temp:     v.GetFloat64("temperature"),
	}
}

func (c *Personality) FromViper(v *vip.Viper) {
	c.Prompt = v.GetString("prompt")
	c.Greeting = v.GetString("greeting")
	c.Nick = v.GetString("nick")
	c.Model = v.GetString("model")
}

type SessionConfig struct {
	Chunkdelay    time.Duration
	Chunkmax      int
	Chunkquoted   bool
	ClientTimeout time.Duration
	MaxHistory    int
	MaxTokens     int
	TTL           time.Duration
}

// sessionconfigfromviper
func SessionFromViper(v *vip.Viper) *SessionConfig {
	return &SessionConfig{
		Chunkdelay:    vip.GetDuration("chunkdelay"),
		Chunkmax:      vip.GetInt("chunkmax"),
		ClientTimeout: vip.GetDuration("timeout"),
		MaxHistory:    vip.GetInt("history"),
		MaxTokens:     vip.GetInt("maxtokens"),
		TTL:           vip.GetDuration("session"),
	}
}

type DiscordConfig struct {
	Token string
}

func DiscordFromViper(v *vip.Viper) *DiscordConfig {
	return &DiscordConfig{
		Token: v.GetString("discordtoken"),
	}
}

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
