package context

import (
	"context"

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

type ChatContext interface {
	context.Context
	Message
	GetPersonality() *Personality
	ChangeName(string) error
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
