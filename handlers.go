package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	ai "github.com/sashabaranov/go-openai"
	vip "github.com/spf13/viper"
)

func complete(c ChatContext, msg string) {
	session := c.GetSession()
	personality := c.GetPersonality()
	session.AddMessage(c, ai.ChatMessageRoleUser, msg)

	respch := CompletionTask(c, &CompletionRequest{
		Client:    c.GetAI(),
		Timeout:   session.Config.ClientTimeout,
		Model:     personality.Model,
		MaxTokens: session.Config.MaxTokens,
		Messages:  session.GetHistory(),
	})

	chunker := &Chunker{
		Size:       session.Config.Chunkmax,
		Chunkdelay: session.Config.Chunkdelay,
		Last:       time.Now(),
		Buffer:     &bytes.Buffer{},
	}

	chunkch := chunker.ChunkFilter(respch)

	all := strings.Builder{}
	for reply := range chunkch {
		all.WriteString(reply)
		log.Printf("<< <%s> %s", personality.Nick, reply)
		c.Reply(reply)
	}

	session.AddMessage(c, ai.ChatMessageRoleAssistant, all.String())
}

func sendGreeting(ctx ChatContext) {
	log.Println("sending greeting...")
	complete(ctx, ctx.GetPersonality().Greeting)
	ctx.GetSession().Reset()
}

var configParams = map[string]string{"prompt": "", "model": "", "nick": "", "greeting": "", "goodbye": "", "directory": "", "session": "", "addressed": ""}

func handleSet(ctx ChatContext) {

	if !ctx.IsAdmin() {
		ctx.Reply("You don't have permission to perform this action.")
		return
	}

	args := ctx.GetArgs()
	if len(args) < 3 {
		ctx.Reply(fmt.Sprintf("Usage: /set %s <value>", keysAsString(configParams)))
		return
	}

	param, v := args[1], args[2:]
	value := strings.Join(v, " ")
	if _, ok := configParams[param]; !ok {
		ctx.Reply(fmt.Sprintf("Unknown parameter. Supported parameters: %v", keysAsString(configParams)))
		return
	}

	// set on global config
	vip.Set(param, value)
	ctx.Reply(fmt.Sprintf("%s set to: %s", param, vip.GetString(param)))

	if param == "nick" {
		ctx.ChangeName(value)
	}

	ctx.GetSession().Reset()
}

func handleGet(ctx ChatContext) {

	tokens := ctx.GetArgs()
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

func handleSave(ctx ChatContext) {

	tokens := ctx.GetArgs()
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

	v.Set("nick", ctx.GetPersonality().Nick)
	v.Set("prompt", ctx.GetPersonality().Prompt)
	v.Set("model", ctx.GetPersonality().Model)
	v.Set("greeting", ctx.GetPersonality().Greeting)
	v.Set("goodbye", ctx.GetPersonality().Goodbye)

	if err := v.WriteConfigAs(vip.GetString("directory") + "/" + filename + ".yml"); err != nil {
		ctx.Reply(fmt.Sprintf("Error saving configuration: %s", err.Error()))
		return
	}

	ctx.Reply(fmt.Sprintf("Configuration saved to: %s", filename))
}

func handleBecome(ctx ChatContext) {

	if !ctx.IsAdmin() {
		ctx.Reply("You don't have permission to perform this action.")
		return
	}

	tokens := ctx.GetArgs()
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
		ctx.GetPersonality().SetConfig(cfg)
	}
	ctx.GetSession().Reset()

	ctx.ChangeName(ctx.GetPersonality().Nick)
	time.Sleep(2 * time.Second)
	sendGreeting(ctx)
}

func handleList(ctx ChatContext) {
	personalities := listPersonalities()
	ctx.Reply(fmt.Sprintf("Available personalities: %s", strings.Join(personalities, ", ")))
}

func handleLeave(ctx ChatContext) {

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

func handleDefault(ctx ChatContext) {
	complete(ctx, strings.Join(ctx.GetArgs(), " "))
}

func handleSay(ctx ChatContext) {

	if !ctx.IsAdmin() {
		ctx.Reply("You don't have permission to perform this action.")
		return
	}

	args := ctx.GetArgs()
	if len(args) < 2 {
		ctx.Reply("Usage: /say [/as <personality>] <message>")
		ctx.Reply("Example: /msg chatbot /say /as marvin talk about life")
		return
	}

	// if second token is '/as' then third token is a personality
	// and we should play as that personality
	as := vip.GetString("become")
	if len(args) > 2 && args[1] == "/as" {
		as = args[2]
		ctx.SetArgs(args[2:])
	}

	if cfg, err := loadPersonality(as); err != nil {
		ctx.Reply(fmt.Sprintf("Error loading personality: %s", err.Error()))
		return
	} else {
		ctx.GetPersonality().SetConfig(cfg)
	}

	ctx.SetSession(sessions.Get(uuid.New().String()))
	ctx.GetSession().Reset()
	ctx.ResetSource()
	ctx.SetArgs(ctx.GetArgs()[1:])

	handleDefault(ctx)
}

func keysAsString(m map[string]string) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return strings.Join(keys, ", ")
}
