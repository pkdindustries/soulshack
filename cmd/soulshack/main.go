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
	"os"
	"strings"
	"time"

	"github.com/lrstanley/girc"
	"github.com/urfave/cli/v3"
	"go.uber.org/zap"

	"pkdindustries/soulshack/internal/bot"
	"pkdindustries/soulshack/internal/config"
	"pkdindustries/soulshack/internal/core"
)

const version = "0.91"

func main() {
	fmt.Printf("%s\n", bot.GetBanner(version))

	cmd := &cli.Command{
		Name:    "soulshack",
		Usage:   "because real people are overrated",
		Version: version + " - http://github.com/pkdindustries/soulshack",
		Flags:   config.GetFlags(),
		Action:  runBot,
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		// Print to stderr first in case logger isn't initialized
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		zap.S().Fatal(err)
	}
}

func runBot(ctx context.Context, c *cli.Command) error {

	cfg := config.NewConfiguration(c)
	core.InitLogger(cfg.Bot.Verbose)
	defer zap.L().Sync() // Flushes buffer, if any

	sys := bot.NewSystem(cfg)

	ircClient := girc.New(girc.Config{
		Server:    cfg.Server.Server,
		Port:      cfg.Server.Port,
		Nick:      cfg.Server.Nick,
		User:      "soulshack",
		Name:      "soulshack",
		SSL:       cfg.Server.SSL,
		TLSConfig: &tls.Config{InsecureSkipVerify: cfg.Server.TLSInsecure},
	})

	if cfg.Server.SASLNick != "" && cfg.Server.SASLPass != "" {
		ircClient.Config.SASL = &girc.SASLPlain{
			User: cfg.Server.SASLNick,
			Pass: cfg.Server.SASLPass,
		}
	}

	ircClient.Handlers.AddBg(girc.CONNECTED, func(client *girc.Client, e girc.Event) {
		zap.S().Infof("Joining channel: %s", cfg.Server.Channel)
		client.Cmd.Join(cfg.Server.Channel)
	})

	ircClient.Handlers.AddBg(girc.JOIN, func(client *girc.Client, e girc.Event) {
		if e.Source.Name == cfg.Server.Nick {
			ctx, cancel := core.NewChatContext(context.Background(), cfg, sys, client, &e)
			defer cancel()
			bot.Greeting(ctx)
		}
	})

	ircClient.Handlers.AddBg(girc.PRIVMSG, func(client *girc.Client, e girc.Event) {
		ctx, cancel := core.NewChatContext(context.Background(), cfg, sys, client, &e)
		defer cancel()

		// Check for URL trigger
		urlTriggered := bot.CheckURLTrigger(ctx, e.Last())

		if ctx.Valid() || urlTriggered {
			// Get lock for this channel to serialize message processing
			channelKey := e.Params[0]
			if !girc.IsValidChannel(channelKey) {
				// For private messages, use the sender's name as the key
				channelKey = e.Source.Name
			}
			lock := core.GetRequestLock(channelKey)

			// Try to acquire lock with context timeout
			ctx.GetLogger().Debugf("Acquiring lock for channel '%s'", channelKey)
			if !lock.LockWithContext(ctx) {
				ctx.GetLogger().Warnf("Failed to acquire lock for channel '%s' (timeout)", channelKey)
				ctx.Reply("Request timed out waiting for previous operation to complete")
				return
			}
			ctx.GetLogger().Debugf("Lock acquired for channel '%s'", channelKey)
			defer func() {
				ctx.GetLogger().Debugf("Releasing lock for channel '%s'", channelKey)
				lock.Unlock()
			}()

			ctx.GetLogger().Infof(">> %s", strings.Join(e.Params[1:], " "))
			switch ctx.GetCommand() {
			case "/set":
				bot.SlashSet(ctx)
			case "/get":
				bot.SlashGet(ctx)
			case "/leave":
				bot.SlashLeave(ctx)
			case "/help":
				fallthrough
			case "/?":
				ctx.Reply("Supported commands: /set, /get, /leave, /help, /version")
			case "/version":
				ctx.Reply(c.Root().Version)
			default:
				bot.CompletionResponse(ctx)
			}
		}
	})

	// Reconnect loop with a maximum retry limit
	maxRetries := 5
	for range maxRetries {
		zap.S().Infow("Connecting to server",
			"server", ircClient.Config.Server,
			"port", ircClient.Config.Port,
			"tls", ircClient.Config.SSL,
			"sasl", ircClient.Config.SASL != nil,
		)
		if err := ircClient.Connect(); err != nil {
			zap.S().Errorw("Connection failed", "error", err)
			zap.S().Info("Reconnecting in 5 seconds")
			time.Sleep(5 * time.Second)
			continue
		}
		return nil
	}
	zap.S().Info("Maximum retry limit reached; exiting")
	return nil
}
