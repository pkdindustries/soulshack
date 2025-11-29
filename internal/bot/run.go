package bot

import (
	"context"
	"crypto/tls"
	"strings"
	"time"

	"github.com/lrstanley/girc"
	"go.uber.org/zap"

	"pkdindustries/soulshack/internal/commands"
	"pkdindustries/soulshack/internal/config"
	"pkdindustries/soulshack/internal/core"
	"pkdindustries/soulshack/internal/irc"
)

// Run starts the IRC bot with the given configuration
func Run(ctx context.Context, cfg *config.Configuration) error {
	core.InitLogger(cfg.Bot.Verbose)
	defer zap.L().Sync()

	sys := NewSystem(cfg)

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

	// Handle graceful shutdown
	go func() {
		<-ctx.Done()
		zap.S().Info("Context cancelled, shutting down IRC client...")
		ircClient.Close()
	}()

	ircClient.Handlers.AddBg(girc.CONNECTED, func(client *girc.Client, e girc.Event) {
		zap.S().Infof("Joining channel: %s", cfg.Server.Channel)
		client.Cmd.Join(cfg.Server.Channel)
	})

	ircClient.Handlers.AddBg(girc.JOIN, func(client *girc.Client, e girc.Event) {
		if e.Source.Name == cfg.Server.Nick {
			ctx, cancel := irc.NewChatContext(context.Background(), cfg, sys, client, &e)
			defer cancel()
			commands.Greeting(ctx)
		}
	})

	ircClient.Handlers.AddBg(girc.PRIVMSG, func(client *girc.Client, e girc.Event) {
		ctx, cancel := irc.NewChatContext(context.Background(), cfg, sys, client, &e)
		defer cancel()

		urlTriggered := commands.CheckURLTrigger(ctx, e.Last())

		if ctx.Valid() || urlTriggered {
			channelKey := e.Params[0]
			if !girc.IsValidChannel(channelKey) {
				channelKey = e.Source.Name
			}
			lock := core.GetRequestLock(channelKey)

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
				commands.SlashSet(ctx)
			case "/get":
				commands.SlashGet(ctx)
			case "/leave":
				commands.SlashLeave(ctx)
			case "/help", "/?":
				ctx.Reply("Supported commands: /set, /get, /leave, /help, /version")
			case "/version":
				ctx.Reply(cfg.Server.Nick + " v0.91")
			default:
				commands.CompletionResponse(ctx)
			}
		}
	})

	// Reconnect loop
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
