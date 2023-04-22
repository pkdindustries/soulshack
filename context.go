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
	IsAddressed() bool
	Sendmessage(string)
	Complete(string)
	ResetSource()
	GetArgs() []string
	SetArgs([]string)
	// session
	ChangeName(string)
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
}

func PersonalityFromViper(v *vip.Viper) *Personality {
	return &Personality{
		Prompt:   v.GetString("prompt"),
		Greeting: v.GetString("greeting"),
		Nick:     v.GetString("nick"),
		Model:    v.GetString("model"),
	}
}

func (c *Personality) SetPersonality(v *vip.Viper) {
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
