package commands

import (
	"fmt"
	"os"
	"strings"
	"time"

	"go.uber.org/zap"

	"pkdindustries/soulshack/internal/irc"
	"pkdindustries/soulshack/internal/llm"
)

func Greeting(ctx irc.ChatContextInterface) {
	config := ctx.GetConfig()
	outch, err := llm.CompleteWithText(ctx, config.Bot.Greeting)

	if err != nil {
		ctx.Reply(err.Error())
		return
	}

	for res := range outch {
		ctx.Reply(res)
	}
}

func SlashSet(ctx irc.ChatContextInterface) {
	if !ctx.IsAdmin() {
		ctx.Reply("You don't have permission to perform this action.")
		return
	}

	keys := getConfigKeys()
	if len(ctx.GetArgs()) < 3 {
		ctx.Reply(fmt.Sprintf("Usage: /set <key> <value>. Available keys: %s", strings.Join(keys, ", ")))
		return
	}

	param, v := ctx.GetArgs()[1], ctx.GetArgs()[2:]
	value := strings.Join(v, " ")
	cfg := ctx.GetConfig()

	ctx.GetLogger().With("param", param, "value", value).Debug("Configuration change request")

	// Handle special cases first
	switch param {
	case "admins":
		admins := strings.Split(value, ",")
		for _, admin := range admins {
			if admin == "" {
				ctx.Reply("Invalid value for admins. Please provide a comma-separated list of hostmasks.")
				return
			}
		}
		cfg.Bot.Admins = admins
		ctx.Reply(fmt.Sprintf("%s set to: %s", param, strings.Join(cfg.Bot.Admins, ", ")))
		ctx.GetSession().Clear()
		return

	case "tools":
		handleToolsSet(ctx, value)
		return
	}

	// Handle standard config fields
	field, ok := configFields[param]
	if !ok {
		ctx.Reply(fmt.Sprintf("Unknown key. Available keys: %s", strings.Join(keys, ", ")))
		return
	}

	if err := field.setter(cfg, value); err != nil {
		ctx.Reply(err.Error())
		return
	}

	ctx.Reply(fmt.Sprintf("%s set to: %s", param, field.getter(cfg)))
	ctx.GetSession().Clear()
}

func SlashGet(ctx irc.ChatContextInterface) {
	keys := getConfigKeys()
	if len(ctx.GetArgs()) < 2 {
		ctx.Reply(fmt.Sprintf("Usage: /get <key>. Available keys: %s", strings.Join(keys, ", ")))
		return
	}

	param := ctx.GetArgs()[1]
	cfg := ctx.GetConfig()

	// Handle special cases first
	switch param {
	case "admins":
		if len(cfg.Bot.Admins) == 0 {
			ctx.Reply("empty admin list, all nicks are permitted to use admin commands")
			return
		}
		ctx.Reply(fmt.Sprintf("%s: %s", param, strings.Join(cfg.Bot.Admins, ", ")))
		return

	case "tools":
		handleToolsGet(ctx)
		return
	}

	// Handle standard config fields
	field, ok := configFields[param]
	if !ok {
		ctx.Reply(fmt.Sprintf("Unknown key %s. Available keys: %s", param, strings.Join(keys, ", ")))
		return
	}

	ctx.Reply(fmt.Sprintf("%s: %s", param, field.getter(cfg)))
}

func SlashLeave(ctx irc.ChatContextInterface) {
	if !ctx.IsAdmin() {
		ctx.Reply("You don't have permission to perform this action.")
		return
	}

	zap.S().Info("Exiting application")
	go func() {
		time.Sleep(1 * time.Second)
		os.Exit(0)
	}()
}
