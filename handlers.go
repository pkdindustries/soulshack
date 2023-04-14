package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	ai "github.com/sashabaranov/go-openai"
	vip "github.com/spf13/viper"
)

func sendGreeting(ctx *ChatContext) {
	log.Println("sending greeting...")
	ctx.Session.Message(ctx, ai.ChatMessageRoleAssistant, ctx.Personality.Greeting)
	rch := ChatCompletionTask(ctx)
	_ = spoolFromChannel(ctx, rch)
	ctx.Session.Reset()
}

func spoolFromChannel(ctx *ChatContext, msgch <-chan *string) *string {
	all := strings.Builder{}
	for reply := range msgch {
		all.WriteString(*reply)
		sendMessage(ctx, reply)
	}
	s := all.String()
	return &s
}

func sendMessage(ctx *ChatContext, message *string) {
	log.Println("<<", ctx.Personality.Nick, *message)
	ctx.Reply(*message)
}

var configParams = map[string]string{"prompt": "", "model": "", "nick": "", "greeting": "", "goodbye": "", "directory": "", "session": "", "addressed": ""}

func handleSet(ctx *ChatContext) {

	if !ctx.IsAdmin() {
		ctx.Reply("You don't have permission to perform this action.")
		return
	}

	if len(ctx.Args) < 3 {
		ctx.Reply(fmt.Sprintf("Usage: /set %s <value>", keysAsString(configParams)))
		return
	}

	param, v := ctx.Args[1], ctx.Args[2:]
	value := strings.Join(v, " ")
	if _, ok := configParams[param]; !ok {
		ctx.Reply(fmt.Sprintf("Unknown parameter. Supported parameters: %v", keysAsString(configParams)))
		return
	}

	// set on global config
	vip.Set(param, value)
	ctx.Reply(fmt.Sprintf("%s set to: %s", param, vip.GetString(param)))

	if param == "nick" {
		ctx.Client.Cmd.Nick(value)
	}

	ctx.Session.Reset()
}

func handleGet(ctx *ChatContext) {

	tokens := ctx.Args
	if len(tokens) < 2 {
		ctx.Reply(fmt.Sprintf("Usage: /get %s", keysAsString(configParams)))
		return
	}

	param := tokens[1]
	if _, ok := configParams[param]; !ok {
		ctx.Reply(fmt.Sprintf("Unknown parameter. Supported parameters: %v", keysAsString(configParams)))
		return
	}

	value := vip.GetString(param)
	ctx.Reply(fmt.Sprintf("%s: %s", param, value))
}

func handleSave(ctx *ChatContext) {

	tokens := ctx.Args
	if !ctx.IsAdmin() {
		ctx.Reply("You don't have permission to perform this action.")
		return
	}

	if len(tokens) < 2 {
		ctx.Reply("Usage: /save <name>")
		return
	}

	filename := tokens[1]

	v := vip.New()

	v.Set("nick", ctx.Personality.Nick)
	v.Set("prompt", ctx.Personality.Prompt)
	v.Set("model", ctx.Personality.Model)
	v.Set("greeting", ctx.Personality.Greeting)
	v.Set("goodbye", ctx.Personality.Goodbye)

	if err := v.WriteConfigAs(vip.GetString("directory") + "/" + filename + ".yml"); err != nil {
		ctx.Reply(fmt.Sprintf("Error saving configuration: %s", err.Error()))
		return
	}

	ctx.Reply(fmt.Sprintf("Configuration saved to: %s", filename))
}

func handleBecome(ctx *ChatContext) {

	if !ctx.IsAdmin() {
		ctx.Reply("You don't have permission to perform this action.")
		return
	}

	tokens := ctx.Args
	if len(tokens) < 2 {
		ctx.Reply("Usage: /become <personality>")
		return
	}

	personality := tokens[1]
	if cfg, err := loadPersonality(personality); err != nil {
		ctx.Reply(fmt.Sprintf("Error loading personality: %s", err.Error()))
		return
	} else {
		vip.MergeConfigMap(cfg.AllSettings())
		ctx.SetConfig(cfg)
	}
	ctx.Session.Reset()
	log.Printf("changing nick to %s", ctx.Personality.Nick)

	ctx.Client.Cmd.Nick(ctx.Personality.Nick)
	time.Sleep(2 * time.Second)
	sendGreeting(ctx)
}

func handleList(ctx *ChatContext) {
	personalities := listPersonalities()
	ctx.Reply(fmt.Sprintf("Available personalities: %s", strings.Join(personalities, ", ")))
}

func handleLeave(ctx *ChatContext) {

	if !ctx.IsAdmin() {
		ctx.Reply("You don't have permission to perform this action.")
		return
	}

	// sendMessage(c, &e, getChatCompletionString(
	// 	[]ai.ChatCompletionMessage{
	// 		{
	// 			Role:    ai.ChatMessageRoleAssistant,
	// 			Content: viper.GetString("prompt") + viper.GetString("goodbye"),
	// 		},
	// 	},
	// ))

	log.Println("exiting...")
	go func() {
		time.Sleep(1 * time.Second)
		os.Exit(0)
	}()
}

func handleDefault(ctx *ChatContext) {
	args := ctx.Args
	msg := strings.Join(args, " ")
	ctx.Session.Message(ctx, ai.ChatMessageRoleUser, msg)
	rch := ChatCompletionTask(ctx)
	reply := spoolFromChannel(ctx, rch)
	ctx.Session.Message(ctx, ai.ChatMessageRoleAssistant, *reply)
}

func handleSay(ctx *ChatContext) {

	if !ctx.IsAdmin() {
		ctx.Reply("You don't have permission to perform this action.")
		return
	}

	if len(ctx.Args) < 2 {
		ctx.Reply("Usage: /say [/as <personality>] <message>")
		ctx.Reply("Example: /msg chatbot /say /as marvin talk about life")
		return
	}

	// if second token is '/as' then third token is a personality
	// and we should play as that personality
	as := vip.GetString("become")
	if len(ctx.Args) > 2 && ctx.Args[1] == "/as" {
		as = ctx.Args[2]
		ctx.Args = ctx.Args[2:]
	}

	if cfg, err := loadPersonality(as); err != nil {
		ctx.Reply(fmt.Sprintf("Error loading personality: %s", err.Error()))
		return
	} else {
		ctx.SetConfig(cfg)
	}

	ctx.Session = sessions.Get(uuid.New().String())
	ctx.Session.Reset()
	ctx.Event.Params[0] = ctx.Config.Channel
	ctx.Event.Source.Name = ctx.Personality.Nick
	ctx.Args = ctx.Args[1:]

	handleDefault(ctx)
}

func keysAsString(m map[string]string) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return strings.Join(keys, ", ")
}
