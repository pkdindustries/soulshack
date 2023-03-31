package main

//  ____                    _   ____    _                      _
// / ___|    ___    _   _  | | / ___|  | |__     __ _    ___  | | __
// \___ \   / _ \  | | | | | | \___ \  | '_ \   / _` |  / __| | |/ /
//  ___) | | (_) | | |_| | | |  ___) | | | | | | (_| | | (__  |   <
// |____/   \___/   \__,_| |_| |____/  |_| |_|  \__,_|  \___| |_|\_\
//  .  .  .  because  real  people  are  overrated

import (
	"crypto/tls"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/common-nighthawk/go-figure"
	"github.com/spf13/cobra"
	vip "github.com/spf13/viper"

	"github.com/lrstanley/girc"
	ai "github.com/sashabaranov/go-openai"
)

func getBanner() string {
	return fmt.Sprintf("%s\n%s",
		figure.NewColorFigure("SoulShack", "", "green", true).ColorString(),
		figure.NewColorFigure(" . . . because real people are overrated", "term", "green", true).ColorString())
}

func main() {
	if err := root.Execute(); err != nil {
		log.Fatal(err)
	}
}

var root = &cobra.Command{
	Use:     "soulshack",
	Example: "soulshack --server irc.freenode.net --port 6697 --channel '#soulshack' --ssl --openaikey ****************",
	Short:   getBanner(),
	Run:     run,
	Version: "0.42 - http://github.com/pkdindustries/soulshack",
}

func run(r *cobra.Command, _ []string) {

	aiClient = ai.NewClient(vip.GetString("openaikey"))

	if vip.GetBool("filter") {
		handleFilter()
		os.Exit(0)
	}

	if err := verifyConfig(); err != nil {
		log.Fatal(err)
	}

	irc := girc.New(girc.Config{
		Server:    vip.GetString("server"),
		Port:      vip.GetInt("port"),
		Nick:      vip.GetString("nick"),
		User:      "soulshack",
		Name:      "soulshack",
		SSL:       vip.GetBool("ssl"),
		TLSConfig: &tls.Config{InsecureSkipVerify: true},
	})

	irc.Handlers.Add(girc.CONNECTED, func(c *girc.Client, e girc.Event) {
		ctx, cancel := createChatContext(c, &e)
		defer cancel()

		channel := vip.GetString("channel")
		log.Println("joining channel:", channel)
		c.Cmd.Join(channel)

		time.Sleep(1 * time.Second)
		sendGreeting(ctx)
	})

	irc.Handlers.Add(girc.PRIVMSG, func(c *girc.Client, e girc.Event) {

		ctx, cancel := createChatContext(c, &e)
		defer cancel()

		if ctx.isValid() {
			log.Println(">>", strings.Join(e.Params[1:], " "))
			switch ctx.getCommand() {
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
				ctx.Reply("Supported commands: /set, /say, /get, /list, /become, /leave, /help, /version")
			case "/version":
				ctx.Reply(r.Version)
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
