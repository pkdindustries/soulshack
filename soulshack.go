package main

//  ____                    _   ____    _                      _
// / ___|    ___    _   _  | | / ___|  | |__     __ _    ___  | | __
// \___ \   / _ \  | | | | | | \___ \  | '_ \   / _` |  / __| | |/ /
//  ___) | | (_) | | |_| | | |  ___) | | | | | | (_| | | (__  |   <
// |____/   \___/   \__,_| |_| |____/  |_| |_|  \__,_|  \___| |_|\_\
//  .  .  .  because  real  people  are  overrated

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/common-nighthawk/go-figure"
	"github.com/spf13/cobra"

	"github.com/lrstanley/girc"
)

var root = &cobra.Command{
	Use:     "soulshack --channel <channel> [--nick <nickname>] [--server <server>] [--port <port>] [--tls] [--openaikey <key>]",
	Example: "soulshack --nick chatbot --server irc.freenode.net --port 6697 --channel '#soulshack' --tls --openaikey ****************",
	Run:     runBot,
	Version: "0.7 - http://github.com/pkdindustries/soulshack",
}

func main() {
	fmt.Printf("%s\n", getBanner())
	InitializeConfig()

	if err := root.Execute(); err != nil {
		log.Fatal(err)
	}
}

func getBanner() string {
	return fmt.Sprintf("%s\n%s",
		figure.NewColorFigure("SoulShack", "", "green", true).ColorString(),
		figure.NewColorFigure(" . . . because real people are overrated", "term", "green", true).ColorString())
}

func runBot(r *cobra.Command, _ []string) {

	irc := girc.New(girc.Config{
		Server:    Config.Server,
		Port:      Config.Port,
		Nick:      Config.Nick,
		User:      "soulshack",
		Name:      "soulshack",
		SSL:       Config.SSL,
		TLSConfig: &tls.Config{InsecureSkipVerify: Config.TLSInsecure},
	})

	if Config.SASLNick != "" && Config.SASLPass != "" {
		irc.Config.SASL = &girc.SASLPlain{
			User: Config.SASLNick,
			Pass: Config.SASLPass,
		}
	}

	irc.Handlers.AddBg(girc.CONNECTED, func(irc *girc.Client, e girc.Event) {
		log.Println("joining channel:", Config.Channel)
		irc.Cmd.Join(Config.Channel)
	})

	irc.Handlers.AddBg(girc.JOIN, func(irc *girc.Client, e girc.Event) {
		if e.Source.Name == Config.Nick {
			ctx, cancel := NewChatContext(context.Background(), Config.OpenAiClient, irc, &e)
			defer cancel()
			greeting(ctx)
		}
	})

	irc.Handlers.AddBg(girc.PRIVMSG, func(irc *girc.Client, e girc.Event) {

		ctx, cancel := NewChatContext(context.Background(), Config.OpenAiClient, irc, &e)
		defer cancel()

		if ctx.Valid() {
			log.Println(">>", strings.Join(e.Params[1:], " "))
			switch ctx.GetCommand() {
			case "/set":
				slashSet(ctx)
			case "/get":
				slashGet(ctx)
			case "/leave":
				slashLeave(ctx)
			case "/help":
				fallthrough
			case "/?":
				ctx.Reply("Supported commands: /set, /get, /leave, /help, /version")
			case "/version":
				ctx.Reply(r.Version)
			default:
				completionResponse(ctx)
			}
		}
	})

	// Reconnect loop with a maximum retry limit
	maxRetries := 5
	for retries := 0; retries < maxRetries; retries++ {
		log.Printf("connecting to server:%s, port:%d, tls:%t, sasl:%t, api:%s", irc.Config.Server, irc.Config.Port, irc.Config.SSL, irc.Config.SASL != nil, Config.OpenAIConfig.BaseURL)
		if err := irc.Connect(); err != nil {
			log.Println("connection error:", err)
			log.Println("reconnecting in 5 seconds...")
			time.Sleep(5 * time.Second)
			continue
		}
		return
	}
	log.Println("maximum retry limit reached, exiting...")
}
