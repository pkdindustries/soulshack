package bot

import (
	"context"
	"crypto/tls"
	"time"

	"fmt"

	"github.com/lrstanley/girc"
	"go.uber.org/zap"

	"pkdindustries/soulshack/internal/commands"
	"pkdindustries/soulshack/internal/config"
	"pkdindustries/soulshack/internal/core"
	"pkdindustries/soulshack/internal/irc"
	"pkdindustries/soulshack/internal/triggers"
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
	cmdRegistry.Register(&commands.StatsCommand{})

	// Initialize trigger registry (order matters: passive watchers first, addressed last as fallback)
	triggerRegistry := triggers.NewRegistry()
	// Lifecycle triggers
	triggerRegistry.Register(&triggers.ConnectedTrigger{})
	triggerRegistry.Register(&triggers.NickErrorTrigger{})
	triggerRegistry.Register(&triggers.ChannelErrorTrigger{})
	// Behavior triggers
	triggerRegistry.Register(&triggers.URLTrigger{})
	triggerRegistry.Register(&triggers.OpTrigger{BotNick: cfg.Server.Nick})
	triggerRegistry.Register(&triggers.JoinTrigger{BotNick: cfg.Server.Nick})
	triggerRegistry.Register(&triggers.AddressedTrigger{CmdRegistry: cmdRegistry})
	triggerRegistry.Register(&triggers.NonAddressedTrigger{CmdRegistry: cmdRegistry})

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
		zap.S().Infow("irc_client_closed")
	}()

	// Single global handler routes all events through the trigger registry
	ircClient.Handlers.AddBg(girc.ALL_EVENTS, func(client *girc.Client, e girc.Event) {
		chatCtx, cancel := irc.NewChatContext(ctx, cfg, sys, client, &e, fatalErr)
		defer cancel()

		key := getLockKey(&e, cfg.Server.Channel)
		var onTimeout func()
		if e.Command == girc.PRIVMSG {
			onTimeout = func() {
				chatCtx.Reply("Request timed out waiting for previous operation to complete")
			}
		}
		core.WithRequestLock(chatCtx, key, e.Command, func() {
			triggerRegistry.Process(chatCtx, &e)
		}, onTimeout)
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

			// Check for fatal IRC errors (nick taken, channel join failures)
			select {
			case fErr := <-fatalErr:
				return fErr
			default:
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

// getLockKey returns the lock key for serializing event processing per conversation
func getLockKey(e *girc.Event, channel string) string {
	if len(e.Params) > 0 && girc.IsValidChannel(e.Params[0]) {
		return channel
	}
	if e.Source != nil {
		return e.Source.Name
	}
	return channel
}
