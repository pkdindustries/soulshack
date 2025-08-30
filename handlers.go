package main

import (
	"fmt"
	"log"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"
	
	"github.com/alexschlessinger/pollytool/tools"
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
		// Handle tools subcommands only
		registry := ctx.GetSystem().GetToolRegistry()

		// Parse to check for subcommands
		parts := strings.Fields(value)
		
		// Check if first word is a known subcommand
		var subcommand string
		if len(parts) > 0 {
			subcommand = parts[0]
		}
		
		// Handle subcommands
		switch subcommand {
		case "add":
			if len(parts) < 2 {
				ctx.Reply("Usage: /set tools add <path>")
				return
			}
			toolPath := strings.Join(parts[1:], " ")

			// Get current tools before loading
			toolsBefore := make(map[string]bool)
			for _, tool := range registry.All() {
				toolsBefore[tool.GetName()] = true
			}

			// Try to auto-detect and load
			_, err := registry.LoadToolAuto(toolPath)
			if err != nil {
				ctx.Reply(fmt.Sprintf("Failed to load tool: %v", err))
				return
			}

			// Find newly loaded tools
			var newTools []string
			for _, tool := range registry.All() {
				name := tool.GetName()
				if !toolsBefore[name] {
					newTools = append(newTools, name)
				}
			}

			if len(newTools) > 0 {
				ctx.Reply(fmt.Sprintf("Loaded tools: %s", strings.Join(newTools, ", ")))
			} else {
				ctx.Reply("No new tools were loaded")
			}

		case "remove":
			if len(parts) < 2 {
				ctx.Reply("Usage: /set tools remove <name>")
				return
			}
			toolName := strings.Join(parts[1:], " ")

			// Check if tool exists
			_, exists := registry.Get(toolName)
			if !exists {
				ctx.Reply(fmt.Sprintf("Tool not found: %s", toolName))
				return
			}

			// Remove the tool from registry
			registry.Remove(toolName)
			ctx.Reply(fmt.Sprintf("Removed tool: %s", toolName))

		case "reload":
			// Reload all tools from config
			ctx.Reply("Reloading all tools...")

			// Clear non-IRC tools
			for _, tool := range registry.All() {
				name := tool.GetName()
				if name != "" && !strings.HasPrefix(name, "irc_") {
					registry.Remove(name)
				}
			}

			// Reload from config
			for _, toolPath := range config.Bot.Tools {
				_, err := registry.LoadToolAuto(toolPath)
				if err != nil {
					log.Printf("warning reloading tool %s: %v", toolPath, err)
				}
			}
			ctx.Reply("All tools reloaded")

		case "mcp":
			// Only support remove for MCP under /set
			if len(parts) < 2 || parts[1] != "remove" {
				ctx.Reply("Usage: /set tools mcp remove <server>")
				ctx.Reply("To list MCP servers, use: /get tools mcp")
				return
			}

			if len(parts) < 3 {
				ctx.Reply("Usage: /set tools mcp remove <server>")
				return
			}
			server := strings.Join(parts[2:], " ")

			// Try to unload the server
			err := registry.UnloadMCPServer(server)
			if err != nil {
				ctx.Reply(fmt.Sprintf("Failed to unload MCP server: %v", err))
			} else {
				ctx.Reply(fmt.Sprintf("Unloaded MCP server: %s", tools.GetMCPDisplayName(server)))
			}
		default:
			// Invalid or missing subcommand
			ctx.Reply("Usage: /set tools [add|remove|reload|mcp]")
			ctx.Reply("  add <path> - Add a tool")
			ctx.Reply("  remove <name> - Remove a tool")
			ctx.Reply("  reload - Reload all tools from config")
			ctx.Reply("  mcp remove <server> - Remove an MCP server")
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
		registry := ctx.GetSystem().GetToolRegistry()

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
	case "tools":
		// Check for subcommands in /get tools
		args := ctx.GetArgs()
		registry := ctx.GetSystem().GetToolRegistry()
		
		if len(args) > 2 {
			subcommand := args[2]
			switch subcommand {
			case "mcp":
				// List MCP tools grouped by server
				mcpToolsByServer := make(map[string][]string)
				for _, tool := range registry.All() {
					if tool.GetType() == "mcp" {
						source := tool.GetSource()
						mcpToolsByServer[source] = append(mcpToolsByServer[source], tool.GetName())
					}
				}
				
				if len(mcpToolsByServer) == 0 {
					ctx.Reply("No MCP tools loaded")
				} else {
					// Build message grouped by server
					var serverGroups []string
					for server, toolNames := range mcpToolsByServer {
						// Just use the server path/filename, not the full command display
						serverGroup := fmt.Sprintf("%s: %s", server, strings.Join(toolNames, ", "))
						serverGroups = append(serverGroups, serverGroup)
					}
					
					fullMessage := "MCP tools by server: " + strings.Join(serverGroups, " | ")
					chunks := chunkMessage(ctx, fullMessage)
					for _, chunk := range chunks {
						ctx.Reply(chunk)
					}
				}
				return
			
			case "shell":
				// List shell tools grouped by source
				shellToolsBySource := make(map[string][]string)
				for _, tool := range registry.All() {
					if tool.GetType() == "shell" {
						source := tool.GetSource()
						if source == "" {
							source = "unknown"
						}
						shellToolsBySource[source] = append(shellToolsBySource[source], tool.GetName())
					}
				}
				
				if len(shellToolsBySource) == 0 {
					ctx.Reply("No shell tools loaded")
				} else {
					// If only one tool per source, show inline. Otherwise group.
					var toolItems []string
					for source, toolNames := range shellToolsBySource {
						if len(toolNames) == 1 {
							// Single tool from this source
							if source != "unknown" {
								toolItems = append(toolItems, fmt.Sprintf("%s (%s)", toolNames[0], source))
							} else {
								toolItems = append(toolItems, toolNames[0])
							}
						} else {
							// Multiple tools from same source - group them
							toolItems = append(toolItems, fmt.Sprintf("%s: %s", source, strings.Join(toolNames, ", ")))
						}
					}
					
					fullMessage := "Shell tools: " + strings.Join(toolItems, ", ")
					chunks := chunkMessage(ctx, fullMessage)
					for _, chunk := range chunks {
						ctx.Reply(chunk)
					}
				}
				return
			
			case "native":
				// List native tools (IRC tools)
				nativeTools := []tools.Tool{}
				for _, tool := range registry.All() {
					if tool.GetType() == "native" {
						nativeTools = append(nativeTools, tool)
					}
				}
				
				if len(nativeTools) == 0 {
					ctx.Reply("No native tools loaded")
				} else {
					// Build complete list of native tools
					var toolNames []string
					for _, tool := range nativeTools {
						toolNames = append(toolNames, tool.GetName())
					}
					
					fullMessage := "Native tools (IRC): " + strings.Join(toolNames, ", ")
					chunks := chunkMessage(ctx, fullMessage)
					for _, chunk := range chunks {
						ctx.Reply(chunk)
					}
				}
				return
			
			case "servers":
				// List MCP servers (not individual tools)
				mcpServers := make(map[string]bool)
				for _, tool := range registry.All() {
					if tool.GetType() == "mcp" {
						mcpServers[tool.GetSource()] = true
					}
				}

				if len(mcpServers) > 0 {
					// Build complete list of MCP servers
					var serverNames []string
					for server := range mcpServers {
						displayName := tools.GetMCPDisplayName(server)
						serverNames = append(serverNames, displayName)
					}
					
					fullMessage := "MCP servers: " + strings.Join(serverNames, ", ")
					chunks := chunkMessage(ctx, fullMessage)
					for _, chunk := range chunks {
						ctx.Reply(chunk)
					}
				} else {
					ctx.Reply("No MCP servers loaded")
				}
				return
			}
		}

		// List all loaded tools (summary)
		allTools := registry.All()
		
		if len(allTools) == 0 {
			ctx.Reply("No tools currently loaded")
			if len(config.Bot.Tools) > 0 {
				ctx.Reply(fmt.Sprintf("Configured but not loaded: %s", strings.Join(config.Bot.Tools, ", ")))
			}
		} else {
			// Count tools by type
			shellCount := 0
			mcpCount := 0
			nativeCount := 0
			mcpServers := make(map[string]bool)
			
			for _, tool := range allTools {
				switch tool.GetType() {
				case "shell":
					shellCount++
				case "mcp":
					mcpCount++
					mcpServers[tool.GetSource()] = true
				case "native":
					nativeCount++
				}
			}
			
			// Build summary message
			var summary []string
			if shellCount > 0 {
				summary = append(summary, fmt.Sprintf("%d shell", shellCount))
			}
			if mcpCount > 0 {
				summary = append(summary, fmt.Sprintf("%d MCP (%d servers)", mcpCount, len(mcpServers)))
			}
			if nativeCount > 0 {
				summary = append(summary, fmt.Sprintf("%d native", nativeCount))
			}
			
			ctx.Reply(fmt.Sprintf("Tools loaded: %s. Use /get tools [shell|mcp|native|servers] for details", strings.Join(summary, ", ")))
		}
	case "irctool":
		if len(config.Bot.IrcTools) == 0 {
			ctx.Reply("irctool: none")
		} else {
			ctx.Reply(fmt.Sprintf("irctool: %s", strings.Join(config.Bot.IrcTools, ", ")))
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

// chunkMessage splits a long message into IRC-safe chunks
func chunkMessage(ctx ChatContextInterface, message string) []string {
	config := ctx.GetConfig()
	maxSize := config.Session.ChunkMax
	if maxSize <= 0 {
		maxSize = 350 // default
	}
	
	if len(message) <= maxSize {
		return []string{message}
	}

	var chunks []string
	remaining := message

	for len(remaining) > 0 {
		if len(remaining) <= maxSize {
			chunks = append(chunks, remaining)
			break
		}

		// Find last space or comma before maxSize for cleaner breaks
		chunk := remaining[:maxSize]
		lastBreak := -1
		
		// Try to find a good break point (comma or space)
		if idx := strings.LastIndex(chunk, ", "); idx > 0 {
			lastBreak = idx + 1 // Keep the comma, break after the space
		} else if idx := strings.LastIndex(chunk, " "); idx > 0 {
			lastBreak = idx
		}

		if lastBreak > 0 {
			// Break at word/item boundary
			chunks = append(chunks, strings.TrimSpace(remaining[:lastBreak]))
			remaining = strings.TrimSpace(remaining[lastBreak:])
		} else {
			// No good break found, break at maxSize
			chunks = append(chunks, chunk)
			remaining = remaining[maxSize:]
		}
	}

	return chunks
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
