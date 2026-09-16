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
			SubagentTimeout:    time.Minute * 15,
			SubagentMax:        4,
			SubagentMaxPerChat: 2,
		},
		Model: &config.ModelConfig{
			Model:          "test/model",
			MaxTokens:      100,
			Temperature:    0.7,
			TopP:           1.0,
			ThinkingEffort: "off",
		},
		Session: &config.SessionConfig{
			ChunkMax:   350,
			MaxContext: 100000,
			TTL:        time.Minute * 10,
		},
		API: &config.APIConfig{
			Timeout: time.Second * 30,
		},
	}
}
