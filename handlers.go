package main

import (
	"fmt"
	"log"
	"os"
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
	case "shelltool":
		// Parse comma-separated shell tool paths
		var toolPaths []string
		if value != "" && value != "none" {
			toolPaths = strings.Split(value, ",")
			for i := range toolPaths {
				toolPaths[i] = strings.TrimSpace(toolPaths[i])
			}
		}
		config.Bot.ShellToolPaths = toolPaths

		// Get the tool registry
		sys := ctx.GetSystem()
		if sys != nil && sys.GetToolRegistry() != nil {
			registry := sys.GetToolRegistry()

			// Clear non-IRC tools (keep IRC tools)
			for _, tool := range registry.All() {
				schema := tool.GetSchema()
				if schema != nil && !strings.HasPrefix(schema.Title, "irc_") {
					registry.Remove(schema.Title)
				}
			}

			// Load and add new tools
			if len(toolPaths) > 0 {
				newTools, err := LoadTools(toolPaths)
				if err != nil {
					log.Printf("warning loading tools: %v", err)
				}
				for _, tool := range newTools {
					registry.Register(tool)
				}
			}
		}

		if len(toolPaths) == 0 {
			ctx.Reply("shelltool disabled")
		} else {
			ctx.Reply(fmt.Sprintf("shelltool set to: %s", strings.Join(toolPaths, ", ")))
		}
	case "mcptool":
		// Parse comma-separated MCP server commands
		var mcpServers []string
		if value != "" && value != "none" {
			mcpServers = strings.Split(value, ",")
			for i := range mcpServers {
				mcpServers[i] = strings.TrimSpace(mcpServers[i])
			}
		}
		config.Bot.MCPServers = mcpServers

		// Get the tool registry
		sys := ctx.GetSystem()
		if sys != nil && sys.GetToolRegistry() != nil {
			registry := sys.GetToolRegistry()

			// For now, we can't type-check pollytool's MCPTool directly
			// TODO: Find a better way to identify and remove MCP tools
			// For simplicity, just clear and reload all tools when MCP servers change

			// Load and add new MCP tools
			if len(mcpServers) > 0 {
				newTools, err := LoadMCPTools(mcpServers)
				if err != nil {
					log.Printf("warning loading MCP tools: %v", err)
				}
				for _, tool := range newTools {
					registry.Register(tool)
				}
			}
		}

		if len(mcpServers) == 0 {
			ctx.Reply("mcptool disabled")
		} else {
			ctx.Reply(fmt.Sprintf("mcptool set to: %s", strings.Join(mcpServers, ", ")))
		}
	case "irctool":
		// Parse comma-separated IRC tool names
		var ircTools []string
		if value != "" && value != "none" {
			ircTools = strings.Split(value, ",")
			for i := range ircTools {
				ircTools[i] = strings.TrimSpace(ircTools[i])
			}
		}
		config.Bot.IrcTools = ircTools

		// Get the tool registry
		sys := ctx.GetSystem()
		if sys != nil && sys.GetToolRegistry() != nil {
			registry := sys.GetToolRegistry()

			// Remove all existing IRC tools
			for _, tool := range registry.All() {
				schema := tool.GetSchema()
				if schema != nil && strings.HasPrefix(schema.Title, "irc_") {
					registry.Remove(schema.Title)
				}
			}

			// Add newly enabled IRC tools
			newIrcTools := GetIrcTools(ircTools)
			for _, tool := range newIrcTools {
				registry.Register(tool)
			}
		}

		if len(ircTools) == 0 {
			ctx.Reply("irctool disabled")
		} else {
			ctx.Reply(fmt.Sprintf("irctool set to: %s", strings.Join(ircTools, ", ")))
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
	case "shelltool":
		if len(config.Bot.ShellToolPaths) == 0 {
			ctx.Reply("shelltool: none")
		} else {
			ctx.Reply(fmt.Sprintf("shelltool: %s", strings.Join(config.Bot.ShellToolPaths, ", ")))
		}
	case "irctool":
		if len(config.Bot.IrcTools) == 0 {
			ctx.Reply("irctool: none")
		} else {
			ctx.Reply(fmt.Sprintf("irctool: %s", strings.Join(config.Bot.IrcTools, ", ")))
		}
	case "mcptool":
		if len(config.Bot.MCPServers) == 0 {
			ctx.Reply("mcptool: none")
		} else {
			ctx.Reply(fmt.Sprintf("mcptool: %s", strings.Join(config.Bot.MCPServers, ", ")))
		}
	case "thinking":
		ctx.Reply(fmt.Sprintf("%s: %t", param, config.Model.Thinking))
	case "showthinkingaction":
		ctx.Reply(fmt.Sprintf("%s: %t", param, config.Bot.ShowThinkingAction))
	case "showtoolactions":
		ctx.Reply(fmt.Sprintf("%s: %t", param, config.Bot.ShowToolActions))
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
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
