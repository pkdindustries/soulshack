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

func greeting(ctx ChatContextInterface) {
	outch := Complete(ctx, ChatText{
		Role: ai.ChatMessageRoleAssistant,
		Text: Config.Greeting,
	})

	res := <-outch
	switch res.Role {
	case ai.ChatMessageRoleAssistant:
		ctx.Reply(res.Text)
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
		Config.Addressed = addressed
		ctx.Reply(fmt.Sprintf("%s set to: %t", param, Config.Addressed))
	case "prompt":
		Config.Prompt = value
		ctx.Reply(fmt.Sprintf("%s set to: %s", param, Config.Prompt))
	case "model":
		Config.Model = value
		ctx.Reply(fmt.Sprintf("%s set to: %s", param, Config.Model))
	case "nick":
		Config.Nick = value
		ctx.Nick(value)
	case "channel":
		Config.Channel = value
		ctx.Part(Config.Channel)
		ctx.Join(value)
	case "maxtokens":
		maxTokens, err := strconv.Atoi(value)
		if err != nil {
			ctx.Reply("Invalid value for maxtokens. Please provide a valid integer.")
			return
		}
		Config.MaxTokens = maxTokens
		ctx.Reply(fmt.Sprintf("%s set to: %d", param, Config.MaxTokens))
	case "temperature":
		temperature, err := strconv.ParseFloat(value, 32)
		if err != nil {
			ctx.Reply("Invalid value for temperature. Please provide a valid float.")
			return
		}
		Config.Temperature = float32(temperature)
		ctx.Reply(fmt.Sprintf("%s set to: %f", param, Config.Temperature))
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
		Config.TopP = float32(topP)
		ctx.Reply(fmt.Sprintf("%s set to: %f", param, Config.TopP))
	case "admins":
		admins := strings.Split(value, ",")
		for _, admin := range admins {
			if admin == "" {
				ctx.Reply("Invalid value for admins. Please provide a comma-separated list of hostmasks.")
				return
			}
		}
		Config.Admins = admins
		ctx.Reply(fmt.Sprintf("%s set to: %s", param, strings.Join(Config.Admins, ", ")))
	case "tools":
		toolUse, err := strconv.ParseBool(value)
		if err != nil {
			ctx.Reply("Invalid value for tools. Please provide 'true' or 'false'.")
			return
		}
		Config.Tools = toolUse
		ctx.Reply(fmt.Sprintf("%s set to: %t", param, Config.Tools))
	}

	ctx.GetSession().Reset()
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

	switch param {
	case "addressed":
		ctx.Reply(fmt.Sprintf("%s: %t", param, Config.Addressed))
	case "prompt":
		ctx.Reply(fmt.Sprintf("%s: %s", param, Config.Prompt))
	case "model":
		ctx.Reply(fmt.Sprintf("%s: %s", param, Config.Model))
	case "nick":
		ctx.Reply(fmt.Sprintf("%s: %s", param, Config.Nick))
	case "channel":
		ctx.Reply(fmt.Sprintf("%s: %s", param, Config.Channel))
	case "maxtokens":
		ctx.Reply(fmt.Sprintf("%s: %d", param, Config.MaxTokens))
	case "temperature":
		ctx.Reply(fmt.Sprintf("%s: %f", param, Config.Temperature))
	case "top_p":
		ctx.Reply(fmt.Sprintf("%s: %f", param, Config.TopP))
	case "admins":
		if len(Config.Admins) == 0 {
			ctx.Reply("empty admin list, all nicks are permitted to use admin commands")
			return
		}
		ctx.Reply(fmt.Sprintf("%s: %s", param, strings.Join(Config.Admins, ", ")))
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
	nick := ctx.GetSource()

	outch := Complete(ctx, ChatText{
		Role: ai.ChatMessageRoleUser,
		Text: fmt.Sprintf("(nick:%s) %s", nick, msg),
	})

	for res := range outch {
		switch res.Role {
		case ai.ChatMessageRoleAssistant:
			ctx.Reply(res.Text)
		default:
			log.Println("non-assistant response:", res)
		}

	}

}
