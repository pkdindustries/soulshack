package main

import (
	"context"
	"crypto/tls"
	"log"
	"time"

	"github.com/lrstanley/girc"
	ai "github.com/sashabaranov/go-openai"
	vip "github.com/spf13/viper"
)

func startIrc(aiClient *ai.Client) {
	irc := girc.New(girc.Config{
		Server:    vip.GetString("server"),
		Port:      vip.GetInt("port"),
		Nick:      vip.GetString("nick"),
		User:      "soulshack",
		Name:      "soulshack",
		SSL:       vip.GetBool("ssl"),
		TLSConfig: &tls.Config{InsecureSkipVerify: true},
	})

	irc.Handlers.AddBg(girc.CONNECTED, func(c *girc.Client, e girc.Event) {
		ctx, cancel := NewIrcContext(context.Background(), aiClient, vip.GetViper(), c, &e)
		defer cancel()

		log.Println("joining channel:", ctx.config.Channel)
		c.Cmd.Join(ctx.config.Channel)

		time.Sleep(1 * time.Second)
		sendGreeting(ctx)
	})

	irc.Handlers.AddBg(girc.PRIVMSG, func(c *girc.Client, e girc.Event) {

		ctx, cancel := NewIrcContext(context.Background(), aiClient, vip.GetViper(), c, &e)
		defer cancel()

		if ctx.IsValid() {
			switch ctx.GetCommand() {
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
			case "/help":
				fallthrough
			case "/?":
				ctx.Reply("Supported commands: /set, /say [/as], /get, /list, /become, /leave, /help, /version")
			// case "/version":
			// 	ctx.Reply(r.Version)
			default:
				handleDefault(ctx)
			}
		}
	})

	for {
		log.Println("connecting to server:", vip.GetString("server"), "port:", vip.GetInt("port"), "ssl:", vip.GetBool("ssl"))
		if err := irc.Connect(); err != nil {
			log.Println(err)
			log.Println("reconnecting in 5 seconds...")
			time.Sleep(5 * time.Second)
		} else {
			return
		}
	}
}
