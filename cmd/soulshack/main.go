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
	"regexp"
	"strings"
	"time"

	"github.com/lrstanley/girc"
	"github.com/mazznoer/colorgrad"
	"github.com/urfave/cli/v2"
	"github.com/urfave/cli/v2/altsrc"
	"go.uber.org/zap"

	"pkdindustries/soulshack/internal/bot"
	"pkdindustries/soulshack/internal/config"
	"pkdindustries/soulshack/internal/core"
)

var urlPattern = regexp.MustCompile(`^https?://[^\s]+`)

const version = "0.91"

func main() {
	fmt.Printf("%s\n", getBanner())

	flags := []cli.Flag{
		// Config file
		&cli.StringFlag{Name: "config", Aliases: []string{"b"}, Usage: "use the named configuration file", EnvVars: []string{"SOULSHACK_CONFIG"}},

		// IRC Client Configuration
		altsrc.NewStringFlag(&cli.StringFlag{Name: "nick", Aliases: []string{"n"}, Value: "soulshack", Usage: "bot's nickname on the irc server", EnvVars: []string{"SOULSHACK_NICK"}}),
		altsrc.NewStringFlag(&cli.StringFlag{Name: "server", Aliases: []string{"s"}, Value: "localhost", Usage: "irc server address", EnvVars: []string{"SOULSHACK_SERVER"}}),
		altsrc.NewBoolFlag(&cli.BoolFlag{Name: "tls", Aliases: []string{"e"}, Usage: "enable TLS for the IRC connection", EnvVars: []string{"SOULSHACK_TLS"}}),
		altsrc.NewBoolFlag(&cli.BoolFlag{Name: "tlsinsecure", Usage: "skip TLS certificate verification", EnvVars: []string{"SOULSHACK_TLSINSECURE"}}),
		altsrc.NewIntFlag(&cli.IntFlag{Name: "port", Aliases: []string{"p"}, Value: 6667, Usage: "irc server port", EnvVars: []string{"SOULSHACK_PORT"}}),
		altsrc.NewStringFlag(&cli.StringFlag{Name: "channel", Aliases: []string{"c"}, Usage: "irc channel to join", EnvVars: []string{"SOULSHACK_CHANNEL"}}),
		altsrc.NewStringFlag(&cli.StringFlag{Name: "saslnick", Usage: "nick used for SASL", EnvVars: []string{"SOULSHACK_SASLNICK"}}),
		altsrc.NewStringFlag(&cli.StringFlag{Name: "saslpass", Usage: "password for SASL plain", EnvVars: []string{"SOULSHACK_SASLPASS"}}),

		// Bot Configuration
		altsrc.NewStringSliceFlag(&cli.StringSliceFlag{Name: "admins", Aliases: []string{"A"}, Usage: "comma-separated list of allowed hostmasks to administrate the bot", EnvVars: []string{"SOULSHACK_ADMINS"}}),
		altsrc.NewBoolFlag(&cli.BoolFlag{Name: "verbose", Aliases: []string{"V"}, Usage: "enable verbose logging of sessions and configuration", EnvVars: []string{"SOULSHACK_VERBOSE"}}),

		// API Configuration
		altsrc.NewStringFlag(&cli.StringFlag{Name: "openaikey", Usage: "OpenAI API key", EnvVars: []string{"SOULSHACK_OPENAIKEY"}}),
		altsrc.NewStringFlag(&cli.StringFlag{Name: "openaiurl", Usage: "OpenAI API URL (for custom endpoints)", EnvVars: []string{"SOULSHACK_OPENAIURL"}}),
		altsrc.NewStringFlag(&cli.StringFlag{Name: "anthropickey", Usage: "Anthropic API key", EnvVars: []string{"SOULSHACK_ANTHROPICKEY"}}),
		altsrc.NewStringFlag(&cli.StringFlag{Name: "geminikey", Usage: "Google Gemini API key", EnvVars: []string{"SOULSHACK_GEMINIKEY"}}),
		altsrc.NewStringFlag(&cli.StringFlag{Name: "ollamaurl", Value: "http://localhost:11434", Usage: "Ollama API URL", EnvVars: []string{"SOULSHACK_OLLAMAURL"}}),
		altsrc.NewStringFlag(&cli.StringFlag{Name: "ollamakey", Usage: "Ollama API key (Bearer token for authentication)", EnvVars: []string{"SOULSHACK_OLLAMAKEY"}}),
		altsrc.NewIntFlag(&cli.IntFlag{Name: "maxtokens", Value: 4096, Usage: "maximum number of tokens to generate", EnvVars: []string{"SOULSHACK_MAXTOKENS"}}),
		altsrc.NewStringFlag(&cli.StringFlag{Name: "model", Value: "ollama/llama3.2", Usage: "model to be used for responses", EnvVars: []string{"SOULSHACK_MODEL"}}),
		altsrc.NewDurationFlag(&cli.DurationFlag{Name: "apitimeout", Aliases: []string{"t"}, Value: time.Minute * 5, Usage: "timeout for each completion request", EnvVars: []string{"SOULSHACK_APITIMEOUT"}}),
		altsrc.NewFloat64Flag(&cli.Float64Flag{Name: "temperature", Value: 0.7, Usage: "temperature for the completion", EnvVars: []string{"SOULSHACK_TEMPERATURE"}}),
		altsrc.NewFloat64Flag(&cli.Float64Flag{Name: "top_p", Value: 1.0, Usage: "top P value for the completion", EnvVars: []string{"SOULSHACK_TOP_P"}}),
		altsrc.NewBoolFlag(&cli.BoolFlag{Name: "thinking", Usage: "enable thinking/reasoning for models that support it", EnvVars: []string{"SOULSHACK_THINKING"}}),
		altsrc.NewStringSliceFlag(&cli.StringSliceFlag{Name: "tool", Usage: "tools to load (shell scripts, MCP server JSON files, or native tools like irc_op)", EnvVars: []string{"SOULSHACK_TOOL"}}),
		altsrc.NewBoolFlag(&cli.BoolFlag{Name: "showthinkingaction", Value: true, Usage: "show '[thinking]' IRC action when bot is reasoning", EnvVars: []string{"SOULSHACK_SHOWTHINKINGACTION"}}),
		altsrc.NewBoolFlag(&cli.BoolFlag{Name: "showtoolactions", Value: true, Usage: "show '[calling toolname]' IRC actions when executing tools", EnvVars: []string{"SOULSHACK_SHOWTOOLACTIONS"}}),
		altsrc.NewBoolFlag(&cli.BoolFlag{Name: "urlwatcher", Usage: "enable passive URL watching and analysis", EnvVars: []string{"SOULSHACK_URLWATCHER"}}),

		// Timeouts and Behavior
		altsrc.NewBoolFlag(&cli.BoolFlag{Name: "addressed", Aliases: []string{"a"}, Value: true, Usage: "require bot be addressed by nick for response", EnvVars: []string{"SOULSHACK_ADDRESSED"}}),
		altsrc.NewDurationFlag(&cli.DurationFlag{Name: "sessionduration", Aliases: []string{"S"}, Value: time.Minute * 10, Usage: "message context will be cleared after it is unused for this duration", EnvVars: []string{"SOULSHACK_SESSIONDURATION"}}),
		altsrc.NewIntFlag(&cli.IntFlag{Name: "sessionhistory", Aliases: []string{"H"}, Value: 250, Usage: "maximum number of lines of context to keep per session", EnvVars: []string{"SOULSHACK_SESSIONHISTORY"}}),
		altsrc.NewIntFlag(&cli.IntFlag{Name: "chunkmax", Aliases: []string{"m"}, Value: 350, Usage: "maximum number of characters to send as a single message", EnvVars: []string{"SOULSHACK_CHUNKMAX"}}),

		// Personality / Prompting
		altsrc.NewStringFlag(&cli.StringFlag{Name: "greeting", Value: "hello.", Usage: "prompt to be used when the bot joins the channel", EnvVars: []string{"SOULSHACK_GREETING"}}),
		altsrc.NewStringFlag(&cli.StringFlag{Name: "prompt", Value: "you are a helpful chatbot. do not use caps. do not use emoji.", Usage: "initial system prompt", EnvVars: []string{"SOULSHACK_PROMPT"}}),
	}

	app := &cli.App{
		Name:    "soulshack",
		Usage:   "because real people are overrated",
		Version: version + " - http://github.com/pkdindustries/soulshack",
		Flags:   flags,
		Before:  altsrc.InitInputSourceWithContext(flags, altsrc.NewYamlSourceFromFlagFunc("config")),
		Action:  runBot,
	}

	if err := app.Run(os.Args); err != nil {
		// Print to stderr first in case logger isn't initialized
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		zap.S().Fatal(err)
	}
}

func getBanner() string {
	banner := `
 ____                    _   ____    _                      _
/ ___|    ___    _   _  | | / ___|  | |__     __ _    ___  | | __
\___ \   / _ \  | | | | | | \___ \  | '_ \   / _' |  / __| | |/ /
 ___) | | (_) | | |_| | | |  ___) | | | | | | (_| | | (__  |   <
|____/   \___/   \__,_| |_| |____/  |_| |_|  \__,_|  \___| |_|\_\
 .  .  .  because  real  people  are  overrated  [v` + version + `]
`
	grad, _ := colorgrad.NewGradient().
		HtmlColors("#1115f0ff", "#fdfdfdff").
		Build()

	lines := strings.Split(banner, "\n")

	// Find max line length for gradient spread
	maxLen := 0
	for _, line := range lines {
		if len(line) > maxLen {
			maxLen = len(line)
		}
	}

	colors := grad.Colors(uint(maxLen))
	var coloredBanner strings.Builder

	for _, line := range lines {
		for i, ch := range line {
			r, g, b, _ := colors[i].RGBA255()
			coloredBanner.WriteString(fmt.Sprintf("\x1b[38;2;%d;%d;%dm%c", r, g, b, ch))
		}
		coloredBanner.WriteString("\x1b[0m\n")
	}

	return coloredBanner.String()
}

func runBot(c *cli.Context) error {

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
		urlTriggered := false
		if cfg.Bot.URLWatcher && !ctx.IsAddressed() {
			if urlPattern.MatchString(e.Last()) {
				urlTriggered = true
				ctx.GetLogger().Info("URL detected, triggering response")
			}
		}

		if ctx.Valid() || urlTriggered {
			// Get lock for this channel to serialize message processing
			channelKey := e.Params[0]
			if !girc.IsValidChannel(channelKey) {
				// For private messages, use the sender's name as the key
				channelKey = e.Source.Name
			}
			lock := core.GetChannelLock(channelKey)

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
				ctx.Reply(c.App.Version)
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
