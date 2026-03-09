package bot

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"time"

	"github.com/lrstanley/girc"

	"pkdindustries/soulshack/internal/behaviors"
	"pkdindustries/soulshack/internal/commands"
	"pkdindustries/soulshack/internal/config"
	"pkdindustries/soulshack/internal/core"
	"pkdindustries/soulshack/internal/irc"
)

// Run starts the IRC bot with the given configuration
func Run(ctx context.Context, cfg *config.Configuration) error {
	level := cfg.Bot.LogLevel
	if cfg.Bot.Verbose {
		level = "debug"
	}
	core.InitLogger(level, cfg.Bot.LogFormat)

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
	cmdRegistry.Register(&commands.StatsCommand{})

	// Initialize behavior registry (order matters: passive watchers first, addressed last as fallback)
	behaviorRegistry := behaviors.NewRegistry()
	// Lifecycle behaviors
	behaviorRegistry.Register(&behaviors.ConnectedBehavior{})
	behaviorRegistry.Register(&behaviors.NickErrorBehavior{})
	behaviorRegistry.Register(&behaviors.ChannelErrorBehavior{})
	// Reactive behaviors
	behaviorRegistry.Register(&behaviors.URLBehavior{})
	behaviorRegistry.Register(&behaviors.OpBehavior{BotNick: cfg.Server.Nick})
	behaviorRegistry.Register(&behaviors.JoinBehavior{BotNick: cfg.Server.Nick})
	behaviorRegistry.Register(&behaviors.AddressedBehavior{CmdRegistry: cmdRegistry})
	behaviorRegistry.Register(&behaviors.NonAddressedBehavior{CmdRegistry: cmdRegistry})

	// Channel for fatal IRC errors (nick taken, channel join failures)
	fatalErr := make(chan error, 1)

	ircClient := girc.New(girc.Config{
		Server:    cfg.Server.Server,
		Port:      cfg.Server.Port,
		Nick:      cfg.Server.Nick,
		User:      "soulshack",
		Name:      "soulshack",
		SSL:       cfg.Server.SSL,
		TLSConfig: &tls.Config{InsecureSkipVerify: cfg.Server.TLSInsecure},
		HandleNickCollide: func(oldNick string) string {
			return "" // Don't auto-retry, we handle it via ERR_NICKNAMEINUSE
		},
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
		slog.Info("irc_client_closed")
	}()

	// Single global handler routes all events through the behavior registry
	ircClient.Handlers.AddBg(girc.ALL_EVENTS, func(client *girc.Client, e girc.Event) {
		if !behaviorRegistry.Handles(e.Command) {
			return
		}
		chatCtx, cancel := irc.NewChatContext(ctx, cfg, sys, client, &e, fatalErr)
		defer cancel()
		behaviorRegistry.Process(chatCtx, &e)
	})

	// Reconnect loop
	const maxRetries = 5
	for i := range maxRetries {
		if ctx.Err() != nil {
			return nil
		}

		slog.Info("server_connecting",
			"server", ircClient.Config.Server,
			"port", ircClient.Config.Port,
			"tls", ircClient.Config.SSL,
			"sasl", ircClient.Config.SASL != nil,
		)

		if err := ircClient.Connect(); err != nil {
			if ctx.Err() != nil {
				return nil
			}

			// Check for fatal IRC errors (nick taken, channel join failures)
			select {
			case fErr := <-fatalErr:
				return fErr
			default:
			}

			slog.Error("connection_failed", "error", err)
			slog.Info("connection_retry", "delay_sec", 5, "attempt", i+1, "max_attempts", maxRetries)

			select {
			case <-time.After(5 * time.Second):
				continue
			case <-ctx.Done():
				return nil
			}
		}

		// Check for fatal IRC errors after successful connection closed
		select {
		case fErr := <-fatalErr:
			return fErr
		default:
		}

		return nil
	}

	return fmt.Errorf("failed to connect after %d attempts", maxRetries)
}
