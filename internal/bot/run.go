package bot

import (
	"context"
	"crypto/tls"
	"strings"
	"time"

	"fmt"

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

	// Initialize command registry
	cmdRegistry := commands.NewRegistry()
	cmdRegistry.Register(&commands.SetCommand{})
	cmdRegistry.Register(&commands.GetCommand{})
	cmdRegistry.Register(commands.NewHelpCommand(cmdRegistry))
	cmdRegistry.Register(&commands.VersionCommand{Version: "v" + Version})
	cmdRegistry.Register(&commands.CompletionCommand{})
	cmdRegistry.Register(&commands.ToolsCommand{})
	cmdRegistry.Register(&commands.AdminCommand{})

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

	go func() {
		<-ctx.Done()
		ircClient.Quit("Shutting down...")
		zap.S().Infow("irc_client_closed")
	}()

	ircClient.Handlers.AddBg(girc.CONNECTED, func(client *girc.Client, e girc.Event) {
		zap.S().Infow("channel_joining", "channel", cfg.Server.Channel)
		client.Cmd.Join(cfg.Server.Channel)
	})

	ircClient.Handlers.AddBg(girc.JOIN, func(client *girc.Client, e girc.Event) {
		if e.Source.Name == cfg.Server.Nick {
			ctx, cancel := irc.NewChatContext(ctx, cfg, sys, client, &e)
			defer cancel()

			channelKey := e.Params[0]
			core.WithRequestLock(ctx, channelKey, "join", func() {
				Greeting(ctx)
			}, nil)
		}
	})

	ircClient.Handlers.AddBg(girc.PRIVMSG, func(client *girc.Client, e girc.Event) {
		ctx, cancel := irc.NewChatContext(ctx, cfg, sys, client, &e)
		defer cancel()

		urlTriggered := CheckURLTrigger(ctx, e.Last())
		ctx.SetURLTriggered(urlTriggered)

		if ctx.Valid() || urlTriggered {
			channelKey := e.Params[0]
			if !girc.IsValidChannel(channelKey) {
				channelKey = e.Source.Name
			}

			core.WithRequestLock(ctx, channelKey, "privmsg", func() {
				ctx.GetLogger().Infow("privmsg_received", "text", strings.Join(e.Params[1:], " "))
				cmdRegistry.Dispatch(ctx)
			}, func() {
				ctx.Reply("Request timed out waiting for previous operation to complete")
			})
		}
	})

	// Reconnect loop
	const maxRetries = 5
	for i := range maxRetries {
		if ctx.Err() != nil {
			return nil
		}

		zap.S().Infow("server_connecting",
			"server", ircClient.Config.Server,
			"port", ircClient.Config.Port,
			"tls", ircClient.Config.SSL,
			"sasl", ircClient.Config.SASL != nil,
		)

		if err := ircClient.Connect(); err != nil {
			if ctx.Err() != nil {
				return nil
			}

			zap.S().Errorw("connection_failed", "error", err)
			zap.S().Infow("connection_retry", "delay_sec", 5, "attempt", i+1, "max_attempts", maxRetries)

			select {
			case <-time.After(5 * time.Second):
				continue
			case <-ctx.Done():
				return nil
			}
		}
		return nil
	}

	return fmt.Errorf("failed to connect after %d attempts", maxRetries)
}
