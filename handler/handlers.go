package handler

import (
	"fmt"
	"log"
	"os"
	"pkdindustries/soulshack/action"
	"pkdindustries/soulshack/completion"
	"pkdindustries/soulshack/config"
	model "pkdindustries/soulshack/model"
	"strings"
	"time"

	ai "github.com/sashabaranov/go-openai"
	vip "github.com/spf13/viper"
	"github.com/subosito/shorturl"
)

func HandleMessage(ctx model.ChatContext) {
	log.Println(ctx.GetArgs())
	if ctx.IsValid() {
		switch strings.ToLower(ctx.GetArgs()[0]) {
		// case "/say":
		// 	handleSay(ctx)
		case "/config":
			Config(ctx)
		case "/save":
			Save(ctx)
		case "/list":
			List(ctx)
		case "/become":
			Become(ctx)
		case "/leave":
			Leave(ctx)
		case "/image":
			Image(ctx)
		case "/wikipedia":
			Wiki(ctx)
		case "/help":
			fallthrough
		case "/?":
			ctx.Sendmessage("Supported commands: /set, /say [/as], /get, /list, /become, /leave, /help, /version, /image")
		// case "/version":
		// 	ctx.Reply(r.Version)
		default:
			HandleDefault(ctx)
		}
	}
}

func Config(ctx model.ChatContext) {
	if !ctx.IsAdmin() {
		ctx.Sendmessage("You don't have permission to perform this action.")
		return
	}

	config := &action.ConfigAction{}
	r, e := config.Execute(ctx, strings.Join(ctx.GetArgs(), " "))
	if e != nil {
		ctx.Sendmessage(e.Error())
		return
	}
	ctx.Sendmessage(r)
}

func Wiki(ctx model.ChatContext) {
	crawl := &action.WikipediaAction{}
	r, e := crawl.Execute(ctx, strings.Join(ctx.GetArgs(), " "))
	if e != nil {
		ctx.Sendmessage(e.Error())
		return
	}
	ctx.Sendmessage(r)
}

func HandleDefault(ctx model.ChatContext) {
	ctx.Complete(strings.Join(ctx.GetArgs(), " "))
}

func SendGreeting(ctx model.ChatContext) {
	log.Println("sending greeting...")
	ctx.Complete(ctx.GetPersonality().Greeting)
}

func Save(ctx model.ChatContext) {

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

func Become(ctx model.ChatContext) {

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
	if cfg, err := config.Load(personality); err != nil {
		ctx.Sendmessage(fmt.Sprintf("Error loading personality: %s", err.Error()))
		return
	} else {
		vip.MergeConfigMap(cfg.AllSettings())
		ctx.GetPersonality().FromViper(cfg)
	}
	//SessionMap.GReset()

	ctx.ChangeName(ctx.GetPersonality().Nick)
	time.Sleep(2 * time.Second)
	SendGreeting(ctx)
}

func List(ctx model.ChatContext) {
	personalities := config.List()
	ctx.Sendmessage(fmt.Sprintf("Available personalities: %s", strings.Join(personalities, ", ")))
}

func Leave(ctx model.ChatContext) {
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

// func handleSay(ctx model.ChatContext) {

// 	if !ctx.IsAdmin() {
// 		ctx.Sendmessage("You don't have permission to perform this action.")
// 		return
// 	}

// 	args := ctx.GetArgs()
// 	if len(args) < 2 {
// 		ctx.Sendmessage("Usage: /say [/as <personality>] <message>")
// 		ctx.Sendmessage("Example: /msg chatbot /say /as marvin talk about life")
// 		return
// 	}

// 	// if second token is '/as' then third token is a personality
// 	// and we should play as that personality
// 	as := vip.GetString("become")
// 	if len(args) > 2 && args[1] == "/as" {
// 		as = args[2]
// 		ctx.SetArgs(args[2:])
// 	}

// 	if cfg, err := LoadConfig(as); err != nil {
// 		ctx.Sendmessage(fmt.Sprintf("Error loading personality: %s", err.Error()))
// 		return
// 	} else {
// 		ctx.GetPersonality().FromViper(cfg)
// 	}

// 	ctx.SetSession(SessionStore.Get(uuid.New().String()))
// 	ctx.GetSession().Reset()
// 	ctx.ResetSource()
// 	ctx.SetArgs(ctx.GetArgs()[1:])

// 	handleDefault(ctx)
// }

// handleimage
func Image(ctx model.ChatContext) {
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

	resp, err := completion.GetAI().CreateImage(ctx, req)
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
