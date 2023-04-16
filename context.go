package main

import (
	"context"
	"time"

	ai "github.com/sashabaranov/go-openai"
	vip "github.com/spf13/viper"
)

type ChatContext interface {
	context.Context
	// msg
	IsAdmin() bool
	IsValid() bool
	GetCommand() string
	IsAddressed() bool
	Reply(message string)
	ResetSource()
	GetArgs() []string
	SetArgs([]string)
	// session
	ChangeName(nick string)
	GetSession() *Session
	SetSession(*Session)
	GetPersonality() *Personality
	GetAI() *ai.Client
}

type Personality struct {
	Prompt   string
	Greeting string
	Nick     string
	Model    string
	Goodbye  string
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

// merge in the viper config
func (c *Personality) SetConfig(v *vip.Viper) {
	c.Prompt = v.GetString("prompt")
	c.Greeting = v.GetString("greeting")
	c.Nick = v.GetString("nick")
	c.Model = v.GetString("model")
	c.Goodbye = v.GetString("goodbye")
}

type SessionConfig struct {
	Chunkdelay     time.Duration
	Chunkmax       int
	ClientTimeout  time.Duration
	MaxHistory     int
	MaxTokens      int
	SessionTimeout time.Duration
}

// sessionconfigfromviper
func SessionFromViper(v *vip.Viper) *SessionConfig {
	return &SessionConfig{
		MaxTokens:      vip.GetInt("maxtokens"),
		SessionTimeout: vip.GetDuration("session"),
		MaxHistory:     vip.GetInt("history"),
		ClientTimeout:  vip.GetDuration("timeout"),
		Chunkdelay:     vip.GetDuration("chunkdelay"),
		Chunkmax:       vip.GetInt("chunkmax"),
	}
}
