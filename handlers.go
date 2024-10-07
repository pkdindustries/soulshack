package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

func greeting(ctx *ChatContext) {
	log.Println("sending greeting...")
	Complete(ctx, RoleAssistant, ctx.Session.Config.Greeting)
	ctx.Session.Reset()
}

func slashSet(ctx *ChatContext) {
	if !ctx.IsAdmin() {
		ctx.Reply("You don't have permission to perform this action.")
		return
	}

	if len(ctx.Args) != 3 {
		ctx.Reply(fmt.Sprintf("Usage: /set <key> <value>. Available keys: %s", strings.Join(Modifiables, ", ")))
		return
	}

	param, v := ctx.Args[1], ctx.Args[2:]
	value := strings.Join(v, " ")

	if !contains(Modifiables, param) {
		ctx.Reply(fmt.Sprintf("Available keys: %s", strings.Join(Modifiables, " ")))
		return
	}

	switch param {
	case "addressed":
		addressed, err := strconv.ParseBool(value)
		if err != nil {
			ctx.Reply("Invalid value for addressed. Please provide 'true' or 'false'.")
			return
		}
		ctx.Session.Config.Addressed = addressed
		ctx.Reply(fmt.Sprintf("%s set to: %t", param, ctx.Session.Config.Addressed))
	case "prompt":
		ctx.Session.Config.Prompt = value
		ctx.Reply(fmt.Sprintf("%s set to: %s", param, ctx.Session.Config.Prompt))
	case "model":
		ctx.Session.Config.Model = value
		ctx.Reply(fmt.Sprintf("%s set to: %s", param, ctx.Session.Config.Model))
	case "nick":
		ctx.Session.Config.Nick = value
		ctx.Client.Cmd.Nick(value)
	case "channel":
		ctx.Session.Config.Channel = value
		ctx.Client.Cmd.Part(ctx.Session.Config.Channel)
		ctx.Client.Cmd.Join(value)
	case "maxtokens":
		maxTokens, err := strconv.Atoi(value)
		if err != nil {
			ctx.Reply("Invalid value for maxtokens. Please provide a valid integer.")
			return
		}
		ctx.Session.Config.MaxTokens = maxTokens
		ctx.Reply(fmt.Sprintf("%s set to: %d", param, ctx.Session.Config.MaxTokens))
	case "tempurature":
		tempurature, err := strconv.ParseFloat(value, 32)
		if err != nil {
			ctx.Reply("Invalid value for tempurature. Please provide a valid float.")
			return
		}
		ctx.Session.Config.Tempurature = float32(tempurature)
		ctx.Reply(fmt.Sprintf("%s set to: %f", param, ctx.Session.Config.Tempurature))
	case "admins":
		admins := strings.Split(value, ",")
		for _, admin := range admins {
			if admin == "" {
				ctx.Reply("Invalid value for admins. Please provide a comma-separated list of non-empty nicknames.")
				return
			}
		}
		ctx.Session.Config.Admins = admins
		ctx.Reply(fmt.Sprintf("%s set to: %s", param, strings.Join(ctx.Session.Config.Admins, ", ")))
	}

	ctx.Session.Reset()
}

func slashGet(ctx *ChatContext) {

	if len(ctx.Args) < 2 {
		ctx.Reply(fmt.Sprintf("Usage: /get <key>. Available keys: %s", strings.Join(Modifiables, ", ")))
		return
	}

	param := ctx.Args[1]
	if !contains(Modifiables, param) {
		ctx.Reply(fmt.Sprintf("Unknown key %s. Available keys: %s", param, strings.Join(Modifiables, ", ")))
		return
	}

	switch param {
	case "addressed":
		ctx.Reply(fmt.Sprintf("%s: %t", param, ctx.Session.Config.Addressed))
	case "prompt":
		ctx.Reply(fmt.Sprintf("%s: %s", param, ctx.Session.Config.Prompt))
	case "model":
		ctx.Reply(fmt.Sprintf("%s: %s", param, ctx.Session.Config.Model))
	case "nick":
		ctx.Reply(fmt.Sprintf("%s: %s", param, ctx.Session.Config.Nick))
	case "channel":
		ctx.Reply(fmt.Sprintf("%s: %s", param, ctx.Session.Config.Channel))
	case "maxtokens":
		ctx.Reply(fmt.Sprintf("%s: %d", param, ctx.Session.Config.MaxTokens))
	case "tempurature":
		ctx.Reply(fmt.Sprintf("%s: %f", param, ctx.Session.Config.Tempurature))
	case "admins":
		if len(ctx.Session.Config.Admins) == 0 {
			ctx.Reply("empty admin list, all nicks are permitted to use admin commands")
			return
		}
		ctx.Reply(fmt.Sprintf("%s: %s", param, strings.Join(ctx.Session.Config.Admins, ", ")))
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
	Complete(ctx, RoleUser, msg)
}
