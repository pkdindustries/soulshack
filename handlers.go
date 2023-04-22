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
	"github.com/subosito/shorturl"
)

func handleMessage(ctx ChatContext) {
	if ctx.IsValid() {
		switch strings.ToLower(ctx.GetArgs()[0]) {
		case "/say":
			handleSay(ctx)
		case "/set":
			handleSet(ctx)
		case "/get":
			handleGet(ctx)
		case "/save":
			handleSave(ctx)
		case "/list":
			handleList(ctx)
		case "/become":
			handleBecome(ctx)
		case "/leave":
			handleLeave(ctx)
		case "/image":
			handleImage(ctx)
		case "/crawl":
			handleCrawl(ctx)
		case "/help":
			fallthrough
		case "/?":
			ctx.Sendmessage("Supported commands: /set, /say [/as], /get, /list, /become, /leave, /help, /version, /image")
		// case "/version":
		// 	ctx.Reply(r.Version)
		default:
			handleDefault(ctx)
		}
	}
}

func handleDefault(ctx ChatContext) {
	ctx.Complete(strings.Join(ctx.GetArgs(), " "))
}

func sendGreeting(ctx ChatContext) {
	log.Println("sending greeting...")
	ctx.Complete(ctx.GetPersonality().Greeting)
	ctx.GetSession().Reset()
}

var configParams = map[string]string{"prompt": "", "model": "", "nick": "", "greeting": "", "directory": "", "session": "", "addressed": ""}

func handleSet(ctx ChatContext) {

	if !ctx.IsAdmin() {
		ctx.Sendmessage("You don't have permission to perform this action.")
		return
	}

	args := ctx.GetArgs()
	if len(args) < 3 {
		ctx.Sendmessage(fmt.Sprintf("Usage: /set %s <value>", keysAsString(configParams)))
		return
	}

	param, v := args[1], args[2:]
	value := strings.Join(v, " ")
	if _, ok := configParams[param]; !ok {
		ctx.Sendmessage(fmt.Sprintf("Unknown parameter. Supported parameters: %v", keysAsString(configParams)))
		return
	}

	// set on global config
	vip.Set(param, value)
	ctx.Sendmessage(fmt.Sprintf("%s set to: %s", param, vip.GetString(param)))

	if param == "nick" {
		ctx.ChangeName(value)
	}

	ctx.GetSession().Reset()
}

func handleGet(ctx ChatContext) {

	tokens := ctx.GetArgs()
	if len(tokens) < 2 {
		ctx.Sendmessage(fmt.Sprintf("Usage: /get %s", keysAsString(configParams)))
		return
	}

	param := tokens[1]
	if _, ok := configParams[param]; !ok {
		ctx.Sendmessage(fmt.Sprintf("Unknown parameter. Supported parameters: %v", keysAsString(configParams)))
		return
	}

	value := vip.GetString(param)
	ctx.Sendmessage(fmt.Sprintf("%s: %s", param, value))
}

func handleSave(ctx ChatContext) {

	tokens := ctx.GetArgs()
	if !ctx.IsAdmin() {
		ctx.Sendmessage("You don't have permission to perform this action.")
		return
	}

	if len(tokens) < 2 {
		ctx.Sendmessage("Usage: /save <name>")
		return
	}

	filename := tokens[1]

	v := vip.New()

	v.Set("nick", ctx.GetPersonality().Nick)
	v.Set("prompt", ctx.GetPersonality().Prompt)
	v.Set("model", ctx.GetPersonality().Model)
	v.Set("greeting", ctx.GetPersonality().Greeting)

	if err := v.WriteConfigAs(vip.GetString("directory") + "/" + filename + ".yml"); err != nil {
		ctx.Sendmessage(fmt.Sprintf("Error saving configuration: %s", err.Error()))
		return
	}

	ctx.Sendmessage(fmt.Sprintf("Configuration saved to: %s", filename))
}

func handleBecome(ctx ChatContext) {

	if !ctx.IsAdmin() {
		ctx.Sendmessage("You don't have permission to perform this action.")
		return
	}

	tokens := ctx.GetArgs()
	if len(tokens) < 2 {
		ctx.Sendmessage("Usage: /become <personality>")
		return
	}

	personality := tokens[1]
	if cfg, err := loadConfig(personality); err != nil {
		ctx.Sendmessage(fmt.Sprintf("Error loading personality: %s", err.Error()))
		return
	} else {
		vip.MergeConfigMap(cfg.AllSettings())
		ctx.GetPersonality().SetPersonality(cfg)
	}
	ctx.GetSession().Reset()

	ctx.ChangeName(ctx.GetPersonality().Nick)
	time.Sleep(2 * time.Second)
	sendGreeting(ctx)
}

func handleList(ctx ChatContext) {
	personalities := listConfigs()
	ctx.Sendmessage(fmt.Sprintf("Available personalities: %s", strings.Join(personalities, ", ")))
}

func handleLeave(ctx ChatContext) {
	if !ctx.IsAdmin() {
		ctx.Sendmessage("You don't have permission to perform this action.")
		return
	}
	log.Println("exiting...")
	go func() {
		time.Sleep(1 * time.Second)
		os.Exit(0)
	}()
}

func handleSay(ctx ChatContext) {

	if !ctx.IsAdmin() {
		ctx.Sendmessage("You don't have permission to perform this action.")
		return
	}

	args := ctx.GetArgs()
	if len(args) < 2 {
		ctx.Sendmessage("Usage: /say [/as <personality>] <message>")
		ctx.Sendmessage("Example: /msg chatbot /say /as marvin talk about life")
		return
	}

	// if second token is '/as' then third token is a personality
	// and we should play as that personality
	as := vip.GetString("become")
	if len(args) > 2 && args[1] == "/as" {
		as = args[2]
		ctx.SetArgs(args[2:])
	}

	if cfg, err := loadConfig(as); err != nil {
		ctx.Sendmessage(fmt.Sprintf("Error loading personality: %s", err.Error()))
		return
	} else {
		ctx.GetPersonality().SetPersonality(cfg)
	}

	ctx.SetSession(sessions.Get(uuid.New().String()))
	ctx.GetSession().Reset()
	ctx.ResetSource()
	ctx.SetArgs(ctx.GetArgs()[1:])

	handleDefault(ctx)
}

// handleimage
func handleImage(ctx ChatContext) {
	args := ctx.GetArgs()

	validrez := map[string]bool{
		"256x256":   true,
		"512x512":   true,
		"1024x1024": true,
	}

	if len(args) < 2 {
		ctx.Sendmessage("Usage: /image [resolution] prompt")
		return
	}

	resolution := "256x256"
	prompt := strings.Join(args[1:], " ")
	if validrez[args[1]] {
		resolution = args[1]
		prompt = strings.Join(args[2:], " ")
	}

	ctx.Sendmessage(fmt.Sprintf("creating %s image...", resolution))
	req := ai.ImageRequest{
		Prompt:         prompt,
		Size:           resolution,
		ResponseFormat: ai.CreateImageResponseFormatURL,
		N:              1,
	}

	resp, err := ctx.GetAI().CreateImage(ctx, req)
	if err != nil {
		ctx.Sendmessage(fmt.Sprintf("image creation error: %v", err))
		return
	}

	u, err := shorturl.Shorten(resp.Data[0].URL, "tinyurl")
	if err != nil {
		log.Printf("error shortening url: %v", err)
		ctx.Sendmessage(resp.Data[0].URL)
	} else {
		ctx.Sendmessage(string(u))
	}
}

func keysAsString(m map[string]string) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return strings.Join(keys, ", ")
}

func handleAction(ctx ChatContext) {
	action := ctx.GetArgs()[1]
	args := ctx.GetArgs()[2:]
	ctx.SetArgs(ctx.GetArgs()[1:])
	switch action {
	case "set":
		handleSet(ctx)
	case "get":
		handleGet(ctx)
	case "become":
		handleBecome(ctx)
	case "image":
		handleImage(ctx)
	case "crawl":
		handleCrawl(ctx)
	default:
		ctx.Sendmessage(fmt.Sprintf("* %s %s", action, strings.Join(args, " ")))
	}
}

func handleCrawl(_ ChatContext) {
	log.Println("crawl unimplemented")
}
