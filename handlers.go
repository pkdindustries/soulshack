package main

import (
	"fmt"
	"log"
	"os"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"
)

func greeting(ctx ChatContextInterface) {
	config := ctx.GetConfig()
	outch, err := CompleteWithText(ctx, config.Bot.Greeting)

	if err != nil {
		ctx.Reply(err.Error())
		return
	}

	for res := range outch {
		ctx.Reply(res)
	}

}

func slashSet(ctx ChatContextInterface) {
	if !ctx.IsAdmin() {
		ctx.Reply("You don't have permission to perform this action.")
		return
	}

	if len(ctx.GetArgs()) < 3 {
		ctx.Reply(fmt.Sprintf("Usage: /set <key> <value>. Available keys: %s", strings.Join(ModifiableConfigKeys, ", ")))
		return
	}

	param, v := ctx.GetArgs()[1], ctx.GetArgs()[2:]
	value := strings.Join(v, " ")
	config := ctx.GetConfig()

	if !contains(ModifiableConfigKeys, param) {
		ctx.Reply(fmt.Sprintf("Available keys: %s", strings.Join(ModifiableConfigKeys, " ")))
		return
	}

	switch param {
	case "addressed":
		addressed, err := strconv.ParseBool(value)
		if err != nil {
			ctx.Reply("Invalid value for addressed. Please provide 'true' or 'false'.")
			return
		}
		config.Bot.Addressed = addressed
		ctx.Reply(fmt.Sprintf("%s set to: %t", param, config.Bot.Addressed))
	case "prompt":
		config.Bot.Prompt = value
		ctx.Reply(fmt.Sprintf("%s set to: %s", param, config.Bot.Prompt))
	case "model":
		config.Model.Model = value
		ctx.Reply(fmt.Sprintf("%s set to: %s", param, config.Model.Model))
	case "maxtokens":
		maxTokens, err := strconv.Atoi(value)
		if err != nil {
			ctx.Reply("Invalid value for maxtokens. Please provide a valid integer.")
			return
		}
		config.Model.MaxTokens = maxTokens
		ctx.Reply(fmt.Sprintf("%s set to: %d", param, config.Model.MaxTokens))
	case "temperature":
		temperature, err := strconv.ParseFloat(value, 32)
		if err != nil {
			ctx.Reply("Invalid value for temperature. Please provide a valid float.")
			return
		}
		config.Model.Temperature = float32(temperature)
		ctx.Reply(fmt.Sprintf("%s set to: %f", param, config.Model.Temperature))
	case "top_p":
		topP, err := strconv.ParseFloat(value, 32)
		if err != nil {
			ctx.Reply("Invalid value for top_p. Please provide a valid float.")
			return
		}
		if topP < 0 || topP > 1 {
			ctx.Reply("Invalid value for top_p. Please provide a float between 0 and 1.")
			return
		}
		config.Model.TopP = float32(topP)
		ctx.Reply(fmt.Sprintf("%s set to: %f", param, config.Model.TopP))
	case "admins":
		admins := strings.Split(value, ",")
		for _, admin := range admins {
			if admin == "" {
				ctx.Reply("Invalid value for admins. Please provide a comma-separated list of hostmasks.")
				return
			}
		}
		config.Bot.Admins = admins
		ctx.Reply(fmt.Sprintf("%s set to: %s", param, strings.Join(config.Bot.Admins, ", ")))
	case "openaiurl":
		config.API.OpenAIURL = value
		ctx.Reply(fmt.Sprintf("%s set to: %s", param, config.API.OpenAIURL))
	case "ollamaurl":
		config.API.OllamaURL = value
		ctx.Reply(fmt.Sprintf("%s set to: %s", param, config.API.OllamaURL))
	case "ollamakey":
		config.API.OllamaKey = value
		ctx.Reply(fmt.Sprintf("%s set to: %s", param, maskAPIKey(value)))
	case "openaikey":
		config.API.OpenAIKey = value
		ctx.Reply(fmt.Sprintf("%s set to: %s", param, maskAPIKey(value)))
	case "anthropickey":
		config.API.AnthropicKey = value
		ctx.Reply(fmt.Sprintf("%s set to: %s", param, maskAPIKey(value)))
	case "geminikey":
		config.API.GeminiKey = value
		ctx.Reply(fmt.Sprintf("%s set to: %s", param, maskAPIKey(value)))
	case "tools":
		// Simple tools management - just add and remove
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

			// Try to load the tool (polly now handles native, shell, and MCP tools)
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

			// Check if it's a wildcard pattern
			if strings.Contains(pattern, "*") {
				// Wildcard removal
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
				// Exact match removal
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
	case "thinking":
		thinking, err := strconv.ParseBool(value)
		if err != nil {
			ctx.Reply("Invalid value for thinking. Please provide 'true' or 'false'.")
			return
		}
		config.Model.Thinking = thinking
		ctx.Reply(fmt.Sprintf("%s set to: %t", param, config.Model.Thinking))
	case "showthinkingaction":
		showThinking, err := strconv.ParseBool(value)
		if err != nil {
			ctx.Reply("Invalid value for showthinkingaction. Please provide 'true' or 'false'.")
			return
		}
		config.Bot.ShowThinkingAction = showThinking
		ctx.Reply(fmt.Sprintf("%s set to: %t", param, config.Bot.ShowThinkingAction))
	case "showtoolactions":
		showTools, err := strconv.ParseBool(value)
		if err != nil {
			ctx.Reply("Invalid value for showtoolactions. Please provide 'true' or 'false'.")
			return
		}
		config.Bot.ShowToolActions = showTools
		ctx.Reply(fmt.Sprintf("%s set to: %t", param, config.Bot.ShowToolActions))
	case "sessionduration":
		duration, err := time.ParseDuration(value)
		if err != nil {
			ctx.Reply("Invalid value for sessionduration. Please provide a valid duration (e.g. 10m, 1h).")
			return
		}
		config.Session.TTL = duration
		ctx.Reply(fmt.Sprintf("%s set to: %s", param, config.Session.TTL))
	case "apitimeout":
		duration, err := time.ParseDuration(value)
		if err != nil {
			ctx.Reply("Invalid value for apitimeout. Please provide a valid duration (e.g. 30s, 5m).")
			return
		}
		config.API.Timeout = duration
		ctx.Reply(fmt.Sprintf("%s set to: %s", param, config.API.Timeout))
	case "sessionhistory":
		history, err := strconv.Atoi(value)
		if err != nil {
			ctx.Reply("Invalid value for sessionhistory. Please provide a valid integer.")
			return
		}
		config.Session.MaxHistory = history
		ctx.Reply(fmt.Sprintf("%s set to: %d", param, config.Session.MaxHistory))
	case "chunkmax":
		chunkMax, err := strconv.Atoi(value)
		if err != nil {
			ctx.Reply("Invalid value for chunkmax. Please provide a valid integer.")
			return
		}
		config.Session.ChunkMax = chunkMax
		ctx.Reply(fmt.Sprintf("%s set to: %d", param, config.Session.ChunkMax))
	}

	ctx.GetSession().Clear()
}

func slashGet(ctx ChatContextInterface) {

	if len(ctx.GetArgs()) < 2 {
		ctx.Reply(fmt.Sprintf("Usage: /get <key>. Available keys: %s", strings.Join(ModifiableConfigKeys, ", ")))
		return
	}

	param := ctx.GetArgs()[1]
	if !contains(ModifiableConfigKeys, param) {
		ctx.Reply(fmt.Sprintf("Unknown key %s. Available keys: %s", param, strings.Join(ModifiableConfigKeys, ", ")))
		return
	}
	config := ctx.GetConfig()

	switch param {
	case "addressed":
		ctx.Reply(fmt.Sprintf("%s: %t", param, config.Bot.Addressed))
	case "prompt":
		ctx.Reply(fmt.Sprintf("%s: %s", param, config.Bot.Prompt))
	case "model":
		ctx.Reply(fmt.Sprintf("%s: %s", param, config.Model.Model))
	case "maxtokens":
		ctx.Reply(fmt.Sprintf("%s: %d", param, config.Model.MaxTokens))
	case "temperature":
		ctx.Reply(fmt.Sprintf("%s: %f", param, config.Model.Temperature))
	case "top_p":
		ctx.Reply(fmt.Sprintf("%s: %f", param, config.Model.TopP))
	case "admins":
		if len(config.Bot.Admins) == 0 {
			ctx.Reply("empty admin list, all nicks are permitted to use admin commands")
			return
		}
		ctx.Reply(fmt.Sprintf("%s: %s", param, strings.Join(config.Bot.Admins, ", ")))
	case "openaiurl":
		ctx.Reply(fmt.Sprintf("%s: %s", param, config.API.OpenAIURL))
	case "ollamaurl":
		ctx.Reply(fmt.Sprintf("%s: %s", param, config.API.OllamaURL))
	case "ollamakey":
		masked := maskAPIKey(config.API.OllamaKey)
		ctx.Reply(fmt.Sprintf("%s: %s", param, masked))
	case "openaikey":
		masked := maskAPIKey(config.API.OpenAIKey)
		ctx.Reply(fmt.Sprintf("%s: %s", param, masked))
	case "anthropickey":
		masked := maskAPIKey(config.API.AnthropicKey)
		ctx.Reply(fmt.Sprintf("%s: %s", param, masked))
	case "geminikey":
		masked := maskAPIKey(config.API.GeminiKey)
		ctx.Reply(fmt.Sprintf("%s: %s", param, masked))
	case "tools":
		// Simple tool listing
		registry := ctx.GetSystem().GetToolRegistry()
		allTools := registry.All()

		if len(allTools) == 0 {
			ctx.Reply("No tools loaded")
		} else {
			// Just list tool names, comma-separated
			var toolNames []string
			for _, tool := range allTools {
				toolNames = append(toolNames, tool.GetName())
			}

			message := "Tools: " + strings.Join(toolNames, ", ")
			// Truncate if too long for IRC
			maxLen := config.Session.ChunkMax
			if maxLen <= 0 {
				maxLen = 350
			}
			if len(message) > maxLen {
				message = message[:maxLen-3] + "..."
			}
			ctx.Reply(message)
		}
	case "thinking":
		ctx.Reply(fmt.Sprintf("%s: %t", param, config.Model.Thinking))
	case "showthinkingaction":
		ctx.Reply(fmt.Sprintf("%s: %t", param, config.Bot.ShowThinkingAction))
	case "showtoolactions":
		ctx.Reply(fmt.Sprintf("%s: %t", param, config.Bot.ShowToolActions))
	case "sessionduration":
		ctx.Reply(fmt.Sprintf("%s: %s", param, config.Session.TTL))
	case "apitimeout":
		ctx.Reply(fmt.Sprintf("%s: %s", param, config.API.Timeout))
	case "sessionhistory":
		ctx.Reply(fmt.Sprintf("%s: %d", param, config.Session.MaxHistory))
	case "chunkmax":
		ctx.Reply(fmt.Sprintf("%s: %d", param, config.Session.ChunkMax))
	}
}

func contains(slice []string, item string) bool {
	return slices.Contains(slice, item)
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

func slashLeave(ctx ChatContextInterface) {

	if !ctx.IsAdmin() {
		ctx.Reply("You don't have permission to perform this action.")
		return
	}

	log.Println("exiting...")
	go func() {
		time.Sleep(1 * time.Second)
		os.Exit(0)
	}()
}

func completionResponse(ctx ChatContextInterface) {
	msg := strings.Join(ctx.GetArgs(), " ")

	outch, err := CompleteWithText(ctx, fmt.Sprintf("(nick:%s) %s", ctx.GetSource(), msg))

	if err != nil {
		log.Println("completionResponse:", err)
		ctx.Reply(err.Error())
		return
	}

	for res := range outch {
		ctx.Reply(res)
	}

}
