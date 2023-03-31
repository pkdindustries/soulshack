package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	ai "github.com/sashabaranov/go-openai"
	vip "github.com/spf13/viper"
)

func sendGreeting(ctx *chatContext) {

	log.Println("sending greeting...")

	ctx.Session.addMessage(ai.ChatMessageRoleUser, vip.GetString("greeting"))

	reply, err := getChatCompletion(ctx, ctx.Session.History)

	if err != nil {
		sendMessage(ctx, err.Error())
		return
	}

	sendMessage(ctx, *reply)
	ctx.Session.addMessage(ai.ChatMessageRoleAssistant, *reply)

}

func sendMessage(ctx *chatContext, message string) {
	log.Println("<<", vip.GetString("become"), message)
	for _, msg := range splitResponse(message, 400) {
		//time.Sleep(500 * time.Millisecond)
		ctx.Reply(msg)
	}
}

var configParams = map[string]string{"prompt": "", "model": "", "nick": "", "greeting": "", "goodbye": "", "answer": "", "directory": "", "session": ""}

func handleSet(ctx *chatContext) {

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

func handleGet(ctx *chatContext) {

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

func handleSave(ctx *chatContext) {

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
	if err := loadPersonality(personality); err != nil {
		log.Println("Error loading personality:", err)
		ctx.Reply(fmt.Sprintf("Error loading personality: %s", err.Error()))
		return
	}
	ctx.Session.Reset()
	log.Printf("changing nick to %s as personality %s", vip.GetString("nick"), personality)

	ctx.Client.Cmd.Nick(vip.GetString("nick"))
	time.Sleep(2 * time.Second)
	sendGreeting(ctx)
}

func handleList(ctx *chatContext) {
	personalities := listPersonalities()
	ctx.Reply(fmt.Sprintf("Available personalities: %s", strings.Join(personalities, ", ")))
}

func handleLeave(ctx *chatContext) {

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

func handleDefault(ctx *chatContext) {

	tokens := ctx.Args
	msg := strings.Join(tokens, " ")

	ctx.Session.addMessage(ai.ChatMessageRoleUser, msg)
	if reply, err := getChatCompletion(ctx, ctx.Session.History); err != nil {
		ctx.Reply(err.Error())
	} else {
		ctx.Session.addMessage(ai.ChatMessageRoleAssistant, *reply)
		sendMessage(ctx, *reply)
	}
}

func handleSay(ctx *chatContext) {

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
	as := vip.GetString("become")
	og := as
	if len(ctx.Args) > 2 && ctx.Args[1] == "/as" {
		as = ctx.Args[2]
		ctx.Args = ctx.Args[2:]
	}

	if as != og {
		if err := loadPersonality(as); err != nil {
			log.Println("Error loading personality:", err)
			ctx.Reply(fmt.Sprintf("Error loading personality: %s", err.Error()))
			return
		}
	}

	// shenanigans
	ctx.Session = sessions.Get("puppet")
	ctx.Session.Reset()
	ctx.Event.Params[0] = vip.GetString("channel")
	ctx.Event.Source.Name = vip.GetString("nick")
	ctx.Args = ctx.Args[1:]

	handleDefault(ctx)
	if as != og {
		if err := loadPersonality(og); err != nil {
			log.Println("Error loading personality:", err)
			ctx.Reply(fmt.Sprintf("Error loading personality: %s", err.Error()))
			return
		}
	}

	// add last message from this session to channel session
	// this is the bot's reply to our /say
	if len(ctx.Session.History) == 0 {
		return
	}
	chsession := sessions.Get(vip.GetString("channel"))
	chsession.History = append(chsession.History, ctx.Session.History[len(ctx.Session.History)-1])
	ctx.Session.Reset()
}

func handleFilter() {
	scanner := bufio.NewScanner(os.Stdin)

	for scanner.Scan() {
		input := scanner.Text()
		msgs := []ai.ChatCompletionMessage{}
		msgs = append(msgs, ai.ChatCompletionMessage{
			Role:    ai.ChatMessageRoleSystem,
			Content: vip.GetString("prompt"),
		})
		msgs = append(msgs, ai.ChatCompletionMessage{
			Role:    ai.ChatMessageRoleUser,
			Content: input,
		})
		response, err := getChatCompletion(context.Background(), msgs)
		if err != nil {
			log.Fatalf("Error generating response: %v", err)
		}
		fmt.Println(*response)
	}
	if err := scanner.Err(); err != nil {
		log.Fatalf("Error reading from stdin: %v", err)
	}
}
