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
	Use:     "soulshack",
	Example: "soulshack --nick chatbot --server irc.freenode.net --port 6697 --channel '#soulshack' --tls --openaikey ****************",
	Short:   getBanner(),
	Run:     run,
	Version: "0.6 - http://github.com/pkdindustries/soulshack",
}

func main() {
	if err := root.Execute(); err != nil {
		log.Fatal(err)
	}
}

func getBanner() string {
	return fmt.Sprintf("%s\n%s",
		figure.NewColorFigure("SoulShack", "", "green", true).ColorString(),
		figure.NewColorFigure(" . . . because real people are overrated", "term", "green", true).ColorString())
}

func run(r *cobra.Command, _ []string) {

	config := LoadConfig()

	if err := config.VerifyConfig(); err != nil {
		log.Fatal(err)
	}

	irc := girc.New(girc.Config{
		Server:    config.Server,
		Port:      config.Port,
		Nick:      config.Nick,
		User:      "soulshack",
		Name:      "soulshack",
		SSL:       true,
		TLSConfig: &tls.Config{InsecureSkipVerify: config.TLSInsecure},
	})

	if config.SASLNick != "" && config.SASLPass != "" {
		irc.Config.SASL = &girc.SASLPlain{
			User: config.SASLNick,
			Pass: config.SASLPass,
		}
	}

	irc.Handlers.AddBg(girc.CONNECTED, func(irc *girc.Client, e girc.Event) {
		ctx, cancel := NewChatContext(context.Background(), config, irc, &e)
		defer cancel()

		log.Println("joining channel:", ctx.Session.Config.Channel)
		irc.Cmd.Join(ctx.Session.Config.Channel)

		time.Sleep(1 * time.Second)
		greeting(ctx)
	})

	irc.Handlers.AddBg(girc.PRIVMSG, func(irc *girc.Client, e girc.Event) {

		ctx, cancel := NewChatContext(context.Background(), config, irc, &e)
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

	for {
		log.Println("connecting to server:", irc.Config.Server, "port:", irc.Config.Port, "tls:", irc.Config.SSL)
		if err := irc.Connect(); err != nil {
			log.Println(err)
			log.Println("reconnecting in 5 seconds...")
			time.Sleep(5 * time.Second)
		} else {
			return
		}
	}
}
