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
	case "tools":
		toolUse, err := strconv.ParseBool(value)
		if err != nil {
			ctx.Reply("Invalid value for tools. Please provide 'true' or 'false'.")
			return
		}
		config.Bot.ToolsEnabled = toolUse
		ctx.Reply(fmt.Sprintf("%s set to: %t", param, config.Bot.ToolsEnabled))
	case "apiurl":
		config.API.URL = value
		ctx.Reply(fmt.Sprintf("%s set to: %s", param, config.API.URL))
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
	case "apiurl":
		ctx.Reply(fmt.Sprintf("%s: %s", param, config.API.URL))
	case "tools":
		ctx.Reply(fmt.Sprintf("%s: %t", param, config.Bot.ToolsEnabled))
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
