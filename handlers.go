package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	ai "github.com/sashabaranov/go-openai"
	vip "github.com/spf13/viper"
)

func sendGreeting(ctx *ChatContext) {

	log.Println("sending greeting...")

	ctx.Session.Message(ctx, ai.ChatMessageRoleUser, ctx.GetConfig().GetString("greeting"))

	reply, err := getChatCompletion(ctx, ctx.Session.History)

	if err != nil {
		sendMessage(ctx, err.Error())
		return
	}

	sendMessage(ctx, *reply)
	ctx.Session.Message(ctx, ai.ChatMessageRoleAssistant, *reply)

}

func sendMessage(ctx *ChatContext, message string) {
	log.Println("<<", ctx.GetConfig().GetString("become"), message)
	for _, msg := range splitResponse(message, 400) {
		//time.Sleep(250 * time.Millisecond)
		ctx.Reply(msg)
	}
}

var configParams = map[string]string{"prompt": "", "model": "", "nick": "", "greeting": "", "goodbye": "", "answer": "", "directory": "", "session": ""}

func handleSet(ctx *ChatContext) {

	tokens := ctx.Args
	if !isAdmin(ctx.Event.Source.Name) {
		ctx.Reply("You don't have permission to perform this action.")
		return
	}

	if len(tokens) < 3 {
		ctx.Reply(fmt.Sprintf("Usage: /set %s <value>", keysAsString(configParams)))
		return
	}

	param, v := tokens[1], tokens[2:]
	value := strings.Join(v, " ")
	if _, ok := configParams[param]; !ok {
		ctx.Reply(fmt.Sprintf("Unknown parameter. Supported parameters: %v", keysAsString(configParams)))
		return
	}

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

	value := ctx.Cfg.GetString(param)
	ctx.Reply(fmt.Sprintf("%s: %s", param, value))
}

func handleSave(ctx *ChatContext) {

	tokens := ctx.Args
	if !isAdmin(ctx.Event.Source.Name) {
		ctx.Reply("You don't have permission to perform this action.")
		return
	}

	if len(tokens) < 2 {
		ctx.Reply("Usage: /save <name>")
		return
	}

	filename := tokens[1]

	v := vip.New()

	v.Set("nick", ctx.Cfg.GetString("nick"))
	v.Set("prompt", ctx.Cfg.GetString("prompt"))
	v.Set("model", ctx.Cfg.GetString("model"))
	v.Set("maxtokens", ctx.Cfg.GetInt("maxtokens"))
	v.Set("greeting", ctx.Cfg.GetString("greeting"))
	v.Set("goodbye", ctx.Cfg.GetString("goodbye"))
	v.Set("answer", ctx.Cfg.GetString("answer"))

	if err := v.WriteConfigAs(vip.GetString("directory") + "/" + filename + ".yml"); err != nil {
		ctx.Reply(fmt.Sprintf("Error saving configuration: %s", err.Error()))
		return
	}

	ctx.Reply(fmt.Sprintf("Configuration saved to: %s", filename))
}

func handleBecome(ctx *ChatContext) {

	if !isAdmin(ctx.Event.Source.Name) {
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
		ctx.MergeConfig(cfg)
	}
	ctx.Session.Reset()
	log.Printf("changing nick to %s", personality)

	ctx.Client.Cmd.Nick(ctx.GetConfig().GetString("nick"))
	time.Sleep(2 * time.Second)
	sendGreeting(ctx)
}

func handleList(ctx *ChatContext) {
	personalities := listPersonalities()
	ctx.Reply(fmt.Sprintf("Available personalities: %s", strings.Join(personalities, ", ")))
}

func handleLeave(ctx *ChatContext) {

	if !isAdmin(ctx.Event.Source.Name) {
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
	if reply, err := getChatCompletion(ctx, ctx.Session.History); err != nil {
		ctx.Reply(err.Error())
	} else {
		ctx.Session.Message(ctx, ai.ChatMessageRoleAssistant, *reply)
		sendMessage(ctx, *reply)
	}
}

func handleSay(ctx *ChatContext) {

	if !isAdmin(ctx.Event.Source.Name) {
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
	as := ctx.Cfg.GetString("become")
	if len(ctx.Args) > 2 && ctx.Args[1] == "/as" {
		as = ctx.Args[2]
		ctx.Args = ctx.Args[2:]
	}

	if cfg, err := loadPersonality(as); err != nil {
		ctx.Reply(fmt.Sprintf("Error loading personality: %s", err.Error()))
		return
	} else {
		ctx.MergeConfig(cfg)
	}

	// shenanigans
	ctx.Session = sessions.Get("puppet")
	ctx.Session.Reset()
	ctx.Event.Params[0] = ctx.Cfg.GetString("channel")
	ctx.Event.Source.Name = ctx.Cfg.GetString("nick")
	ctx.Args = ctx.Args[1:]

	handleDefault(ctx)
}
