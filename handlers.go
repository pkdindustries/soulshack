package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	vip "github.com/spf13/viper"
)

func sendGreeting(ctx *ChatContext) {
	log.Println("sending greeting...")
	Complete(ctx, RoleAssistant, ctx.Session.Config.Greeting)
	ctx.Session.Reset()
}

func handleSet(ctx *ChatContext) {
	if !ctx.IsAdmin() {
		ctx.Reply("You don't have permission to perform this action.")
		return
	}

	if len(ctx.Args) < 3 {
		ctx.Reply(fmt.Sprintf("Usage: /set <key> <value>. Available keys: %s", strings.Join(vip.AllKeys(), ", ")))
		return
	}

	param, v := ctx.Args[1], ctx.Args[2:]
	value := strings.Join(v, " ")

	// Check if the key exists in the configuration
	if !vip.IsSet(param) {
		ctx.Reply(fmt.Sprintf("Unknown parameter. Available parameters: %s", strings.Join(vip.AllKeys(), ", ")))
		return
	}

	// Set on global config
	vip.Set(param, value)
	ctx.Reply(fmt.Sprintf("%s set to: %s", param, vip.GetString(param)))

	if param == "nick" {
		ctx.Client.Cmd.Nick(value)
	}

	ctx.Session.Reset()
}

func handleGet(ctx *ChatContext) {
	if len(ctx.Args) < 2 {
		ctx.Reply(fmt.Sprintf("Usage: /get <key>. Available keys: %s", strings.Join(vip.AllKeys(), ", ")))
		return
	}

	param := ctx.Args[1]
	if !vip.IsSet(param) {
		ctx.Reply(fmt.Sprintf("Unknown parameter. Available parameters: %s", strings.Join(vip.AllKeys(), ", ")))
		return
	}

	value := vip.GetString(param)
	ctx.Reply(fmt.Sprintf("%s: %s", param, value))
}

func handleSave(ctx *ChatContext) {
	if !ctx.IsAdmin() {
		ctx.Reply("You don't have permission to perform this action.")
		return
	}

	if len(ctx.Args) < 2 {
		ctx.Reply("Usage: /save <name>")
		return
	}

	filename := ctx.Args[1]

	v := vip.New()

	// Save all current configuration keys
	for _, key := range vip.AllKeys() {
		v.Set(key, vip.Get(key))
	}

	if err := v.WriteConfigAs(vip.GetString("directory") + "/" + filename + ".yml"); err != nil {
		ctx.Reply(fmt.Sprintf("Error saving configuration: %s", err.Error()))
		return
	}

	ctx.Reply(fmt.Sprintf("Configuration saved to: %s", filename))
}

func handleLeave(ctx *ChatContext) {

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

func handleDefault(ctx *ChatContext) {
	args := ctx.Args
	msg := strings.Join(args, " ")
	Complete(ctx, RoleUser, msg)
}
