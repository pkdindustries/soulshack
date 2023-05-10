package handler

import (
	"fmt"
	"log"
	"os"
	"pkdindustries/soulshack/action"
	"pkdindustries/soulshack/config"
	model "pkdindustries/soulshack/model"
	"strings"
	"time"

	vip "github.com/spf13/viper"
)

func HandleMessage(ctx model.ChatContext) {
	log.Println(ctx.GetArgs())
	if ctx.IsValid() {
		switch strings.ToLower(ctx.GetArgs()[0]) {
		case "/say":
			Say(ctx)
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
			ctx.Send("Supported commands: /set, /say [/as], /get, /list, /become, /leave, /help, /version, /image, /wiki")
		default:
			HandleDefault(ctx)
		}
	}
}

func Config(ctx model.ChatContext) {
	if !ctx.IsAdmin() {
		ctx.Send("You don't have permission to perform this action.")
		return
	}

	config := &action.ConfigAction{}
	r, e := config.Execute(ctx, strings.Join(ctx.GetArgs(), " "))
	if e != nil {
		ctx.Send(e.Error())
		return
	}
	ctx.Send(r)
}

func Wiki(ctx model.ChatContext) {
	crawl := &action.WikipediaAction{}
	r, e := crawl.Execute(ctx, strings.Join(ctx.GetArgs(), " "))
	if e != nil {
		ctx.Send(e.Error())
		return
	}
	ctx.Complete("summarize: " + r)
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
		ctx.Send("You don't have permission to perform this action.")
		return
	}

	if len(tokens) < 2 {
		ctx.Send("Usage: /save <name>")
		return
	}

	filename := tokens[1]

	v := vip.New()

	v.Set("nick", ctx.GetPersonality().Nick)
	v.Set("prompt", ctx.GetPersonality().Prompt)
	v.Set("model", ctx.GetPersonality().Model)
	v.Set("greeting", ctx.GetPersonality().Greeting)

	if err := v.WriteConfigAs(vip.GetString("directory") + "/" + filename + ".yml"); err != nil {
		ctx.Send(fmt.Sprintf("Error saving configuration: %s", err.Error()))
		return
	}

	ctx.Send(fmt.Sprintf("Configuration saved to: %s", filename))
}

func Become(ctx model.ChatContext) {

	if !ctx.IsAdmin() {
		ctx.Send("You don't have permission to perform this action.")
		return
	}

	tokens := ctx.GetArgs()
	if len(tokens) < 2 {
		ctx.Send("Usage: /become <personality>")
		return
	}

	personality := tokens[1]
	if cfg, err := config.Load(personality); err != nil {
		ctx.Send(fmt.Sprintf("Error loading personality: %s", err.Error()))
		return
	} else {
		vip.MergeConfigMap(cfg.AllSettings())
		ctx.GetPersonality().FromViper(cfg)
	}

	ctx.ChangeName(ctx.GetPersonality().Nick)
	ctx.ResetSession()
	time.Sleep(2 * time.Second)
	SendGreeting(ctx)
}

func List(ctx model.ChatContext) {
	personalities := config.List()
	ctx.Send(fmt.Sprintf("Available personalities: %s", strings.Join(personalities, ", ")))
}

func Leave(ctx model.ChatContext) {
	if !ctx.IsAdmin() {
		ctx.Send("You don't have permission to perform this action.")
		return
	}
	log.Println("exiting...")
	os.Exit(0)
}

func Say(ctx model.ChatContext) {

	if !ctx.IsAdmin() {
		ctx.Send("You don't have permission to perform this action.")
		return
	}

	args := ctx.GetArgs()
	if len(args) < 2 {
		ctx.Send("Usage: /say [/as <personality>] <message>")
		ctx.Send("Example: /msg chatbot /say /as marvin talk about life")
		return
	}

	// if second token is '/as' then third token is a personality
	// and we should play as that personality
	as := vip.GetString("become")
	if len(args) > 2 && args[1] == "/as" {
		as = args[2]
		ctx.SetArgs(args[2:])
	}

	if cfg, err := config.Load(as); err != nil {
		ctx.Send(fmt.Sprintf("Error loading personality: %s", err.Error()))
		return
	} else {
		ctx.GetPersonality().FromViper(cfg)
	}

	ctx.ResetSession()
	ctx.ResetSource()
	ctx.SetArgs(ctx.GetArgs()[1:])

	HandleDefault(ctx)
}

// handleimage
func Image(ctx model.ChatContext) {
	image := &action.ImageAction{}
	r, e := image.Execute(ctx, strings.Join(ctx.GetArgs(), " "))
	if e != nil {
		ctx.Send(e.Error())
		return
	}
	ctx.Send(r)
}
