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
		zap.S().Info("IRC client closed")
	}()

	ircClient.Handlers.AddBg(girc.CONNECTED, func(client *girc.Client, e girc.Event) {
		zap.S().Infof("Joining channel: %s", cfg.Server.Channel)
		client.Cmd.Join(cfg.Server.Channel)
	})

	ircClient.Handlers.AddBg(girc.JOIN, func(client *girc.Client, e girc.Event) {
		if e.Source.Name == cfg.Server.Nick {
			ctx, cancel := irc.NewChatContext(ctx, cfg, sys, client, &e)
			defer cancel()
			Greeting(ctx)
		}
	})

	ircClient.Handlers.AddBg(girc.PRIVMSG, func(client *girc.Client, e girc.Event) {
		ctx, cancel := irc.NewChatContext(ctx, cfg, sys, client, &e)
		defer cancel()

		urlTriggered := CheckURLTrigger(ctx, e.Last())

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
			cmdRegistry.Dispatch(ctx)
		}
	})

	// Reconnect loop
	const maxRetries = 5
	for i := range maxRetries {
		if ctx.Err() != nil {
			return nil
		}

		zap.S().Infow("Connecting to server",
			"server", ircClient.Config.Server,
			"port", ircClient.Config.Port,
			"tls", ircClient.Config.SSL,
			"sasl", ircClient.Config.SASL != nil,
		)

		if err := ircClient.Connect(); err != nil {
			if ctx.Err() != nil {
				return nil
			}

			zap.S().Errorw("Connection failed", "error", err)
			zap.S().Infof("Reconnecting in 5 seconds (attempt %d/%d)", i+1, maxRetries)

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
