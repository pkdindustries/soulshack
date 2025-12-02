package testing

import (
	"time"

	"pkdindustries/soulshack/internal/config"
)

// DefaultTestConfig returns a minimal configuration for testing
func DefaultTestConfig() *config.Configuration {
	return &config.Configuration{
		Server: &config.ServerConfig{
			Nick:    "testbot",
			Server:  "irc.test.local",
			Port:    6667,
			Channel: "#test",
			SSL:     false,
		},
		Bot: &config.BotConfig{
			Admins:             []string{},
			Verbose:            false,
			Addressed:          true,
			Prompt:             "You are a test bot.",
			Greeting:           "hello",
			Tools:              []string{},
			ShowThinkingAction: false,
			ShowToolActions:    false,
		},
		Model: &config.ModelConfig{
			Model:       "test/model",
			MaxTokens:   100,
			Temperature: 0.7,
			TopP:        1.0,
			Thinking:    false,
		},
		Session: &config.SessionConfig{
			ChunkMax:   350,
			MaxHistory: 50,
			TTL:        time.Minute * 10,
		},
		API: &config.APIConfig{
			Timeout: time.Second * 30,
		},
	}
}

// KickCall records a Kick() invocation
type KickCall struct {
	Channel string
	Nick    string
	Reason  string
}

// ModeCall records a Mode() invocation
type ModeCall struct {
	Channel string
	Mode    string
	Target  string
}

// TopicCall records a Topic() invocation
type TopicCall struct {
	Channel string
	Topic   string
}

// OperCall records an Oper() invocation
type OperCall struct {
	Channel string
	Nick    string
}
