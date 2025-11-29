package commands

import (
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"

	"pkdindustries/soulshack/internal/config"
	"pkdindustries/soulshack/internal/irc"
)

// handleToolsGet handles the /get tools command
func handleToolsGet(ctx irc.ChatContextInterface) {
	registry := ctx.GetSystem().GetToolRegistry()
	allTools := registry.All()

	if len(allTools) == 0 {
		ctx.Reply("No tools loaded")
		return
	}

	var toolNames []string
	for _, tool := range allTools {
		toolNames = append(toolNames, tool.GetName())
	}

	message := "Tools: " + strings.Join(toolNames, ", ")
	maxLen := ctx.GetConfig().Session.ChunkMax
	if maxLen <= 0 {
		maxLen = 350
	}
	if len(message) > maxLen {
		message = message[:maxLen-3] + "..."
	}
	ctx.Reply(message)
}

// handleToolsSet handles the /set tools subcommand
func handleToolsSet(ctx irc.ChatContextInterface, value string) {
	registry := ctx.GetSystem().GetToolRegistry()
	parts := strings.Fields(value)

	if len(parts) == 0 {
		ctx.Reply("Usage: /set tools [add|remove]")
		return
	}

	subcommand := parts[0]
	switch subcommand {
	case "add":
		if len(parts) < 2 {
			ctx.Reply("Usage: /set tools add <path>")
			return
		}
		toolPath := strings.Join(parts[1:], " ")

		_, err := registry.LoadToolAuto(toolPath)
		if err != nil {
			ctx.Reply(fmt.Sprintf("Failed: %v", err))
		} else {
			ctx.Reply(fmt.Sprintf("Added tool: %s", toolPath))
		}

	case "remove":
		if len(parts) < 2 {
			ctx.Reply("Usage: /set tools remove <name or pattern>")
			return
		}
		pattern := strings.Join(parts[1:], " ")

		if strings.Contains(pattern, "*") {
			var removed []string
			for _, tool := range registry.All() {
				name := tool.GetName()
				matched, _ := path.Match(pattern, name)
				if matched {
					registry.Remove(name)
					removed = append(removed, name)
				}
			}

			if len(removed) > 0 {
				ctx.Reply(fmt.Sprintf("Removed %d tools: %s", len(removed), strings.Join(removed, ", ")))
			} else {
				ctx.Reply(fmt.Sprintf("No tools matched pattern: %s", pattern))
			}
		} else {
			if _, exists := registry.Get(pattern); !exists {
				ctx.Reply(fmt.Sprintf("Not found: %s", pattern))
			} else {
				registry.Remove(pattern)
				ctx.Reply(fmt.Sprintf("Removed: %s", pattern))
			}
		}

	default:
		ctx.Reply("Usage: /set tools [add|remove]")
	}
}

// configField defines how to get and set a configuration value
type configField struct {
	setter func(*config.Configuration, string) error
	getter func(*config.Configuration) string
}

// configFields maps parameter names to their handlers
var configFields = map[string]configField{
	"addressed": {
		setter: func(c *config.Configuration, v string) error {
			b, err := strconv.ParseBool(v)
			if err != nil {
				return fmt.Errorf("invalid value for addressed. Please provide 'true' or 'false'")
			}
			c.Bot.Addressed = b
			return nil
		},
		getter: func(c *config.Configuration) string { return fmt.Sprintf("%t", c.Bot.Addressed) },
	},
	"prompt": {
		setter: func(c *config.Configuration, v string) error { c.Bot.Prompt = v; return nil },
		getter: func(c *config.Configuration) string { return c.Bot.Prompt },
	},
	"model": {
		setter: func(c *config.Configuration, v string) error { c.Model.Model = v; return nil },
		getter: func(c *config.Configuration) string { return c.Model.Model },
	},
	"maxtokens": {
		setter: func(c *config.Configuration, v string) error {
			n, err := strconv.Atoi(v)
			if err != nil {
				return fmt.Errorf("invalid value for maxtokens. Please provide a valid integer")
			}
			c.Model.MaxTokens = n
			return nil
		},
		getter: func(c *config.Configuration) string { return fmt.Sprintf("%d", c.Model.MaxTokens) },
	},
	"temperature": {
		setter: func(c *config.Configuration, v string) error {
			f, err := strconv.ParseFloat(v, 32)
			if err != nil {
				return fmt.Errorf("invalid value for temperature. Please provide a valid float")
			}
			c.Model.Temperature = float32(f)
			return nil
		},
		getter: func(c *config.Configuration) string { return fmt.Sprintf("%f", c.Model.Temperature) },
	},
	"top_p": {
		setter: func(c *config.Configuration, v string) error {
			f, err := strconv.ParseFloat(v, 32)
			if err != nil {
				return fmt.Errorf("invalid value for top_p. Please provide a valid float")
			}
			if f < 0 || f > 1 {
				return fmt.Errorf("invalid value for top_p. Please provide a float between 0 and 1")
			}
			c.Model.TopP = float32(f)
			return nil
		},
		getter: func(c *config.Configuration) string { return fmt.Sprintf("%f", c.Model.TopP) },
	},
	"openaiurl": {
		setter: func(c *config.Configuration, v string) error { c.API.OpenAIURL = v; return nil },
		getter: func(c *config.Configuration) string { return c.API.OpenAIURL },
	},
	"ollamaurl": {
		setter: func(c *config.Configuration, v string) error { c.API.OllamaURL = v; return nil },
		getter: func(c *config.Configuration) string { return c.API.OllamaURL },
	},
	"ollamakey": {
		setter: func(c *config.Configuration, v string) error { c.API.OllamaKey = v; return nil },
		getter: func(c *config.Configuration) string { return maskAPIKey(c.API.OllamaKey) },
	},
	"openaikey": {
		setter: func(c *config.Configuration, v string) error { c.API.OpenAIKey = v; return nil },
		getter: func(c *config.Configuration) string { return maskAPIKey(c.API.OpenAIKey) },
	},
	"anthropickey": {
		setter: func(c *config.Configuration, v string) error { c.API.AnthropicKey = v; return nil },
		getter: func(c *config.Configuration) string { return maskAPIKey(c.API.AnthropicKey) },
	},
	"geminikey": {
		setter: func(c *config.Configuration, v string) error { c.API.GeminiKey = v; return nil },
		getter: func(c *config.Configuration) string { return maskAPIKey(c.API.GeminiKey) },
	},
	"thinking": {
		setter: func(c *config.Configuration, v string) error {
			b, err := strconv.ParseBool(v)
			if err != nil {
				return fmt.Errorf("invalid value for thinking. Please provide 'true' or 'false'")
			}
			c.Model.Thinking = b
			return nil
		},
		getter: func(c *config.Configuration) string { return fmt.Sprintf("%t", c.Model.Thinking) },
	},
	"showthinkingaction": {
		setter: func(c *config.Configuration, v string) error {
			b, err := strconv.ParseBool(v)
			if err != nil {
				return fmt.Errorf("invalid value for showthinkingaction. Please provide 'true' or 'false'")
			}
			c.Bot.ShowThinkingAction = b
			return nil
		},
		getter: func(c *config.Configuration) string { return fmt.Sprintf("%t", c.Bot.ShowThinkingAction) },
	},
	"showtoolactions": {
		setter: func(c *config.Configuration, v string) error {
			b, err := strconv.ParseBool(v)
			if err != nil {
				return fmt.Errorf("invalid value for showtoolactions. Please provide 'true' or 'false'")
			}
			c.Bot.ShowToolActions = b
			return nil
		},
		getter: func(c *config.Configuration) string { return fmt.Sprintf("%t", c.Bot.ShowToolActions) },
	},
	"sessionduration": {
		setter: func(c *config.Configuration, v string) error {
			d, err := time.ParseDuration(v)
			if err != nil {
				return fmt.Errorf("invalid value for sessionduration. Please provide a valid duration (e.g. 10m, 1h)")
			}
			c.Session.TTL = d
			return nil
		},
		getter: func(c *config.Configuration) string { return c.Session.TTL.String() },
	},
	"apitimeout": {
		setter: func(c *config.Configuration, v string) error {
			d, err := time.ParseDuration(v)
			if err != nil {
				return fmt.Errorf("invalid value for apitimeout. Please provide a valid duration (e.g. 30s, 5m)")
			}
			c.API.Timeout = d
			return nil
		},
		getter: func(c *config.Configuration) string { return c.API.Timeout.String() },
	},
	"sessionhistory": {
		setter: func(c *config.Configuration, v string) error {
			n, err := strconv.Atoi(v)
			if err != nil {
				return fmt.Errorf("invalid value for sessionhistory. Please provide a valid integer")
			}
			c.Session.MaxHistory = n
			return nil
		},
		getter: func(c *config.Configuration) string { return fmt.Sprintf("%d", c.Session.MaxHistory) },
	},
	"chunkmax": {
		setter: func(c *config.Configuration, v string) error {
			n, err := strconv.Atoi(v)
			if err != nil {
				return fmt.Errorf("invalid value for chunkmax. Please provide a valid integer")
			}
			c.Session.ChunkMax = n
			return nil
		},
		getter: func(c *config.Configuration) string { return fmt.Sprintf("%d", c.Session.ChunkMax) },
	},
	"urlwatcher": {
		setter: func(c *config.Configuration, v string) error {
			b, err := strconv.ParseBool(v)
			if err != nil {
				return fmt.Errorf("invalid value for urlwatcher. Please provide 'true' or 'false'")
			}
			c.Bot.URLWatcher = b
			return nil
		},
		getter: func(c *config.Configuration) string { return fmt.Sprintf("%t", c.Bot.URLWatcher) },
	},
}

// getConfigKeys returns all available config keys
func getConfigKeys() []string {
	keys := make([]string, 0, len(configFields)+2)
	for k := range configFields {
		keys = append(keys, k)
	}
	keys = append(keys, "admins", "tools")
	return keys
}

// maskAPIKey returns a masked version of an API key showing only first 4 chars
func maskAPIKey(key string) string {
	if key == "" {
		return "(not set)"
	}
	if len(key) <= 4 {
		return strings.Repeat("*", len(key))
	}
	return key[:4] + strings.Repeat("*", len(key)-4)
}
