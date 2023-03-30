package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/lrstanley/girc"
	ai "github.com/sashabaranov/go-openai"
	vip "github.com/spf13/viper"
)

func sendGreeting(ctx *chatContext) {

	_, c, e, session, _ := getFromContext(ctx)

	log.Println("sending greeting...")

	session.Clear()
	session.addMessage(ai.ChatMessageRoleAssistant, vip.GetString("greeting"), "")

	reply, err := getChatCompletion(session.History)

	if err != nil {
		sendMessage(c, e, err.Error())
		return
	}

	sendMessage(ctx.Client, ctx.Event, *reply)

	session.addMessage(ai.ChatMessageRoleAssistant, *reply, "")

}

func sendMessage(c *girc.Client, e *girc.Event, message string) {
	log.Printf("params [%s] name: [%s] msg: %s", e, e.Source.Name, message)
	sendMessageChunks(c, e, &message)
}

func sendMessageChunks(c *girc.Client, e *girc.Event, message *string) {
	chunks := splitResponse(*message, 400)
	for _, msg := range chunks {
		time.Sleep(500 * time.Millisecond)
		c.Cmd.Reply(*e, msg)
	}
}

var configParams = map[string]string{"prompt": "", "model": "", "nick": "", "greeting": "", "goodbye": "", "answer": "", "directory": "", "session": ""}

func handleSet(ctx *chatContext) {

	_, c, e, session, tokens := getFromContext(ctx)
	if !isAdmin(e.Source.Name) {
		c.Cmd.Reply(*e, "You don't have permission to perform this action.")
		return
	}

	if len(tokens) < 3 {
		c.Cmd.Reply(*e, fmt.Sprintf("Usage: /set %s <value>", keysAsString(configParams)))
		return
	}

	param, v := tokens[1], tokens[2:]
	value := strings.Join(v, " ")
	if _, ok := configParams[param]; !ok {
		c.Cmd.Reply(*e, fmt.Sprintf("Unknown parameter. Supported parameters: %v", keysAsString(configParams)))
		return
	}

	vip.Set(param, value)
	c.Cmd.Reply(*e, fmt.Sprintf("%s set to: %s", param, vip.GetString(param)))

	if param == "nick" {
		c.Cmd.Nick(value)
	}

	session.Clear()
}

func handleGet(ctx *chatContext) {

	_, c, e, _, tokens := getFromContext(ctx)
	if len(tokens) < 2 {
		c.Cmd.Reply(*e, fmt.Sprintf("Usage: /get %s", keysAsString(configParams)))
		return
	}

	param := tokens[1]
	if _, ok := configParams[param]; !ok {
		c.Cmd.Reply(*e, fmt.Sprintf("Unknown parameter. Supported parameters: %v", keysAsString(configParams)))
		return
	}

	value := vip.GetString(param)
	c.Cmd.Reply(*e, fmt.Sprintf("%s: %s", param, value))
}

func handleSave(ctx *chatContext) {

	_, c, e, _, tokens := getFromContext(ctx)
	if !isAdmin(e.Source.Name) {
		c.Cmd.Reply(*e, "You don't have permission to perform this action.")
		return
	}

	if len(tokens) < 2 {
		c.Cmd.Reply(*e, "Usage: /save <name>")
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
		c.Cmd.Reply(*e, fmt.Sprintf("Error saving configuration: %s", err.Error()))
		return
	}

	c.Cmd.Reply(*e, fmt.Sprintf("Configuration saved to: %s", filename))
}

func handleBecome(ctx *chatContext) {

	_, c, e, _, tokens := getFromContext(ctx)
	if !isAdmin(e.Source.Name) {
		c.Cmd.Reply(*e, "You don't have permission to perform this action.")
		return
	}

	if len(tokens) < 2 {
		c.Cmd.Reply(*e, "Usage: /become <personality>")
		return
	}

	personality := tokens[1]
	if err := loadPersonality(personality); err != nil {
		log.Println("Error loading personality:", err)
		c.Cmd.Reply(*e, fmt.Sprintf("Error loading personality: %s", err.Error()))
		return
	}

	log.Printf("changing nick to %s as personality %s", vip.GetString("nick"), personality)

	c.Cmd.Nick(vip.GetString("nick"))
	time.Sleep(2 * time.Second)
	sendGreeting(ctx)
}

func handleList(ctx *chatContext) {

	_, c, e, _, _ := getFromContext(ctx)
	personalities := listPersonalities()
	c.Cmd.Reply(*e, fmt.Sprintf("Available personalities: %s", strings.Join(personalities, ", ")))
}

func handleLeave(ctx *chatContext) {

	_, c, e, _, _ := getFromContext(ctx)

	if !isAdmin(e.Source.Name) {
		c.Cmd.Reply(*e, "You don't have permission to perform this action.")
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

	_, c, e, session, tokens := getFromContext(ctx)

	msg := strings.Join(tokens, " ")

	session.addMessage(ai.ChatMessageRoleUser, msg, e.Source.Name)
	if reply, err := getChatCompletion(session.History); err != nil {
		c.Cmd.Reply(*e, err.Error())
	} else {

		session.addMessage(ai.ChatMessageRoleUser, *reply, e.Source.Name)

		sendMessage(c, e, *reply)
	}
}
