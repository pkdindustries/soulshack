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

func sendGreeting(ctx *chatContext) {

	_, _, _, session, _ := getFromContext(ctx)

	log.Println("sending greeting...")

	session.Clear()
	session.addMessage(ai.ChatMessageRoleAssistant, vip.GetString("greeting"), "")

	reply, err := getChatCompletion(session.History)

	if err != nil {
		sendMessage(ctx, err.Error())
		return
	}

	sendMessage(ctx, *reply)
	session.addMessage(ai.ChatMessageRoleAssistant, *reply, "")

}

func sendMessage(ctx *chatContext, message string) {
	log.Printf("params [%s] name: [%s] msg: %s", ctx.Event, ctx.Event.Source.Name, message)
	for _, msg := range splitResponse(message, 400) {
		//time.Sleep(500 * time.Millisecond)
		ctx.Reply(msg)
	}
}

var configParams = map[string]string{"prompt": "", "model": "", "nick": "", "greeting": "", "goodbye": "", "answer": "", "directory": "", "session": ""}

func handleSet(ctx *chatContext) {

	_, c, e, session, tokens := getFromContext(ctx)
	if !isAdmin(e.Source.Name) {
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
		c.Cmd.Nick(value)
	}

	session.Clear()
}

func handleGet(ctx *chatContext) {

	_, _, _, _, tokens := getFromContext(ctx)
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

func handleSave(ctx *chatContext) {

	_, _, e, _, tokens := getFromContext(ctx)
	if !isAdmin(e.Source.Name) {
		ctx.Reply("You don't have permission to perform this action.")
		return
	}

	if len(tokens) < 2 {
		ctx.Reply("Usage: /save <name>")
		return
	}

	filename := tokens[1]

	v := vip.New()

	v.Set("nick", vip.GetString("nick"))
	v.Set("prompt", vip.GetString("prompt"))
	v.Set("model", vip.GetString("model"))
	v.Set("maxtokens", vip.GetInt("maxtokens"))
	v.Set("greeting", vip.GetString("greeting"))
	v.Set("goodbye", vip.GetString("goodbye"))
	v.Set("answer", vip.GetString("answer"))

	if err := v.WriteConfigAs(vip.GetString("directory") + "/" + filename + ".yml"); err != nil {
		ctx.Reply(fmt.Sprintf("Error saving configuration: %s", err.Error()))
		return
	}

	ctx.Reply(fmt.Sprintf("Configuration saved to: %s", filename))
}

func handleBecome(ctx *chatContext) {

	_, c, e, _, tokens := getFromContext(ctx)
	if !isAdmin(e.Source.Name) {
		ctx.Reply("You don't have permission to perform this action.")
		return
	}

	if len(tokens) < 2 {
		ctx.Reply("Usage: /become <personality>")
		return
	}

	personality := tokens[1]
	if err := loadPersonality(personality); err != nil {
		log.Println("Error loading personality:", err)
		ctx.Reply(fmt.Sprintf("Error loading personality: %s", err.Error()))
		return
	}

	log.Printf("changing nick to %s as personality %s", vip.GetString("nick"), personality)

	c.Cmd.Nick(vip.GetString("nick"))
	time.Sleep(2 * time.Second)
	sendGreeting(ctx)
}

func handleList(ctx *chatContext) {
	personalities := listPersonalities()
	ctx.Reply(fmt.Sprintf("Available personalities: %s", strings.Join(personalities, ", ")))
}

func handleLeave(ctx *chatContext) {

	_, _, e, _, _ := getFromContext(ctx)

	if !isAdmin(e.Source.Name) {
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

func handleDefault(ctx *chatContext) {

	_, _, e, session, tokens := getFromContext(ctx)

	msg := strings.Join(tokens, " ")

	session.addMessage(ai.ChatMessageRoleUser, msg, e.Source.Name)
	if reply, err := getChatCompletion(session.History); err != nil {
		ctx.Reply(err.Error())
	} else {
		session.addMessage(ai.ChatMessageRoleUser, *reply, e.Source.Name)
		sendMessage(ctx, *reply)
	}
}

func handleSay(ctx *chatContext) {

	_, _, e, _, _ := getFromContext(ctx)

	if !isAdmin(e.Source.Name) {
		ctx.Reply("You don't have permission to perform this action.")
		return
	}

	// shenanigans
	// get the channel session and play over there
	ctx.Session = getSession(vip.GetString("channel"))
	ctx.Event.Params[0] = vip.GetString("channel")
	e.Source.Name = vip.GetString("nick")
	handleDefault(ctx)
	// remove second to last entry from history, which is our /say msg
	// now it looks like we never said it and the bot just decided to say it
	ctx.Session.History = ctx.Session.History[:len(ctx.Session.History)-2]

}
