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
	Use:     "soulshack --channel <channel> [--nick <nickname>] [--server <server>] [--port <port>] [--tls] [--apikey <key>]",
	Example: "soulshack --nick chatbot --server irc.freenode.net --port 6697 --channel '#soulshack' --tls --apikey ****************",
	Run:     runBot,
	Version: "0.7 - http://github.com/pkdindustries/soulshack",
}

func main() {
	fmt.Printf("%s\n", getBanner())
	initializeConfig()

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

	config := NewConfiguration()
	sys := NewSystem(config)

	irc := girc.New(girc.Config{
		Server:    config.Server.Server,
		Port:      config.Server.Port,
		Nick:      config.Server.Nick,
		User:      "soulshack",
		Name:      "soulshack",
		SSL:       config.Server.SSL,
		TLSConfig: &tls.Config{InsecureSkipVerify: config.Server.TLSInsecure},
	})

	if config.Server.SASLNick != "" && config.Server.SASLPass != "" {
		irc.Config.SASL = &girc.SASLPlain{
			User: config.Server.SASLNick,
			Pass: config.Server.SASLPass,
		}
	}

	irc.Handlers.AddBg(girc.CONNECTED, func(irc *girc.Client, e girc.Event) {
		log.Println("joining channel:", config.Server.Channel)
		irc.Cmd.Join(config.Server.Channel)
	})

	irc.Handlers.AddBg(girc.JOIN, func(irc *girc.Client, e girc.Event) {
		if e.Source.Name == config.Server.Nick {
			ctx, cancel := NewChatContext(context.Background(), config, sys, irc, &e)
			defer cancel()
			greeting(ctx)
		}
	})

	irc.Handlers.AddBg(girc.PRIVMSG, func(irc *girc.Client, e girc.Event) {
		ctx, cancel := NewChatContext(context.Background(), config, sys, irc, &e)
		defer cancel()
		if ctx.Valid() {
			// Get lock for this channel to serialize message processing
			channelKey := e.Params[0]
			if !girc.IsValidChannel(channelKey) {
				// For private messages, use the sender's name as the key
				channelKey = e.Source.Name
			}
			lock := getChannelLock(channelKey)

			// Try to acquire lock with context timeout
			log.Printf("Acquiring lock for channel '%s'", channelKey)
			if !lock.LockWithContext(ctx) {
				log.Printf("Failed to acquire lock for channel '%s' (timeout)", channelKey)
				ctx.Reply("Request timed out waiting for previous operation to complete")
				return
			}
			log.Printf("Lock acquired for channel '%s'", channelKey)
			defer func() {
				log.Printf("Releasing lock for channel '%s'", channelKey)
				lock.Unlock()
			}()

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
	for range maxRetries {
		log.Printf("connecting to server:%s, port:%d, tls:%t, sasl:%t", irc.Config.Server, irc.Config.Port, irc.Config.SSL, irc.Config.SASL != nil)
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
