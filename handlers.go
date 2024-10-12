package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	ai "github.com/sashabaranov/go-openai"
)

func greeting(ctx *ChatContext) {
	log.Println("sending greeting...")
	Complete(ctx, ai.ChatCompletionMessage{
		Role:    ai.ChatMessageRoleAssistant,
		Content: BotConfig.Greeting,
	})
	ctx.Session.Reset()
}

func slashSet(ctx *ChatContext) {
	if !ctx.IsAdmin() {
		ctx.Reply("You don't have permission to perform this action.")
		return
	}

	if len(ctx.Args) < 3 {
		ctx.Reply(fmt.Sprintf("Usage: /set <key> <value>. Available keys: %s", strings.Join(ModifiableConfigKeys, ", ")))
		return
	}

	param, v := ctx.Args[1], ctx.Args[2:]
	value := strings.Join(v, " ")

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
		BotConfig.Addressed = addressed
		ctx.Reply(fmt.Sprintf("%s set to: %t", param, BotConfig.Addressed))
	case "prompt":
		BotConfig.Prompt = value
		ctx.Reply(fmt.Sprintf("%s set to: %s", param, BotConfig.Prompt))
	case "model":
		BotConfig.Model = value
		ctx.Reply(fmt.Sprintf("%s set to: %s", param, BotConfig.Model))
	case "nick":
		BotConfig.Nick = value
		ctx.Client.Cmd.Nick(value)
	case "channel":
		BotConfig.Channel = value
		ctx.Client.Cmd.Part(BotConfig.Channel)
		ctx.Client.Cmd.Join(value)
	case "maxtokens":
		maxTokens, err := strconv.Atoi(value)
		if err != nil {
			ctx.Reply("Invalid value for maxtokens. Please provide a valid integer.")
			return
		}
		BotConfig.MaxTokens = maxTokens
		ctx.Reply(fmt.Sprintf("%s set to: %d", param, BotConfig.MaxTokens))
	case "temperature":
		temperature, err := strconv.ParseFloat(value, 32)
		if err != nil {
			ctx.Reply("Invalid value for temperature. Please provide a valid float.")
			return
		}
		BotConfig.Temperature = float32(temperature)
		ctx.Reply(fmt.Sprintf("%s set to: %f", param, BotConfig.Temperature))
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
		BotConfig.TopP = float32(topP)
		ctx.Reply(fmt.Sprintf("%s set to: %f", param, BotConfig.TopP))
	case "admins":
		admins := strings.Split(value, ",")
		for _, admin := range admins {
			if admin == "" {
				ctx.Reply("Invalid value for admins. Please provide a comma-separated list of hostmasks.")
				return
			}
		}
		BotConfig.Admins = admins
		ctx.Reply(fmt.Sprintf("%s set to: %s", param, strings.Join(BotConfig.Admins, ", ")))
	case "tools":
		toolUse, err := strconv.ParseBool(value)
		if err != nil {
			ctx.Reply("Invalid value for tools. Please provide 'true' or 'false'.")
			return
		}
		BotConfig.Tools = toolUse
		ctx.Reply(fmt.Sprintf("%s set to: %t", param, BotConfig.Tools))
	}

	ctx.Session.Reset()
}

func slashGet(ctx *ChatContext) {

	if len(ctx.Args) < 2 {
		ctx.Reply(fmt.Sprintf("Usage: /get <key>. Available keys: %s", strings.Join(ModifiableConfigKeys, ", ")))
		return
	}

	param := ctx.Args[1]
	if !contains(ModifiableConfigKeys, param) {
		ctx.Reply(fmt.Sprintf("Unknown key %s. Available keys: %s", param, strings.Join(ModifiableConfigKeys, ", ")))
		return
	}

	switch param {
	case "addressed":
		ctx.Reply(fmt.Sprintf("%s: %t", param, BotConfig.Addressed))
	case "prompt":
		ctx.Reply(fmt.Sprintf("%s: %s", param, BotConfig.Prompt))
	case "model":
		ctx.Reply(fmt.Sprintf("%s: %s", param, BotConfig.Model))
	case "nick":
		ctx.Reply(fmt.Sprintf("%s: %s", param, BotConfig.Nick))
	case "channel":
		ctx.Reply(fmt.Sprintf("%s: %s", param, BotConfig.Channel))
	case "maxtokens":
		ctx.Reply(fmt.Sprintf("%s: %d", param, BotConfig.MaxTokens))
	case "temperature":
		ctx.Reply(fmt.Sprintf("%s: %f", param, BotConfig.Temperature))
	case "top_p":
		ctx.Reply(fmt.Sprintf("%s: %f", param, BotConfig.TopP))
	case "admins":
		if len(BotConfig.Admins) == 0 {
			ctx.Reply("empty admin list, all nicks are permitted to use admin commands")
			return
		}
		ctx.Reply(fmt.Sprintf("%s: %s", param, strings.Join(BotConfig.Admins, ", ")))
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

func slashLeave(ctx *ChatContext) {

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

func completionResponse(ctx *ChatContext) {
	msg := strings.Join(ctx.Args, " ")
	nick := ctx.Event.Source.Name
	Complete(ctx, ai.ChatCompletionMessage{
		Role:    ai.ChatMessageRoleUser,
		Content: fmt.Sprintf("(nick:%s) %s", nick, msg),
	})
}
