package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/urfave/cli/v3"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

type Configuration struct {
	Server  *ServerConfig
	Bot     *BotConfig
	Model   *ModelConfig
	Session *SessionConfig
	API     *APIConfig
}

type ServerConfig struct {
	Nick        string
	Server      string
	Port        int
	Channel     string
	SSL         bool
	TLSInsecure bool
	SASLNick    string
	SASLPass    string
}

type BotConfig struct {
	Admins             []string
	Verbose            bool
	Addressed          bool
	Prompt             string
	Greeting           string
	Tools              []string
	ShowThinkingAction bool
	ShowToolActions    bool
	URLWatcher         bool
}

type ModelConfig struct {
	Model       string
	MaxTokens   int
	Temperature float32
	TopP        float32
	Thinking    bool
}

type SessionConfig struct {
	ChunkMax   int
	MaxHistory int
	TTL        time.Duration
}

type APIConfig struct {
	Timeout      time.Duration
	OpenAIKey    string
	OpenAIURL    string
	AnthropicKey string
	GeminiKey    string
	OllamaURL    string
	OllamaKey    string
}

// YamlSource implements cli.ValueSource for a map loaded from YAML
type YamlSource struct {
	data map[string]any
	key  string
}

func (y *YamlSource) Lookup() (string, bool) {
	if v, ok := y.data[y.key]; ok {
		// Handle slices by joining with comma
		if slice, ok := v.([]any); ok {
			var strs []string
			for _, item := range slice {
				strs = append(strs, fmt.Sprintf("%v", item))
			}
			return strings.Join(strs, ","), true
		}
		return fmt.Sprintf("%v", v), true
	}
	return "", false
}

func (y *YamlSource) String() string   { return "yaml" }
func (y *YamlSource) GoString() string { return "yaml" }

func GetFlags() []cli.Flag {
	// Pre-parse config path
	configPath := getConfigPath()
	var configData map[string]any
	if configPath != "" {
		data, err := os.ReadFile(configPath)
		if err == nil {
			_ = yaml.Unmarshal(data, &configData)
		} else {
			fmt.Fprintf(os.Stderr, "Warning: failed to read config file %s: %v\n", configPath, err)
		}
	}

	// Helper to create sources: EnvVar > YAML > Default
	src := func(key string, env ...string) cli.ValueSourceChain {
		chain := cli.ValueSourceChain{}
		for _, e := range env {
			chain.Chain = append(chain.Chain, cli.EnvVar(e))
		}
		if configData != nil {
			chain.Chain = append(chain.Chain, &YamlSource{data: configData, key: key})
		}
		return chain
	}

	return []cli.Flag{
		// Config file
		&cli.StringFlag{Name: "config", Aliases: []string{"b"}, Usage: "use the named configuration file", Sources: cli.EnvVars("SOULSHACK_CONFIG")},

		// IRC Client Configuration
		&cli.StringFlag{Name: "nick", Aliases: []string{"n"}, Value: "soulshack", Usage: "bot's nickname on the irc server", Sources: src("nick", "SOULSHACK_NICK")},
		&cli.StringFlag{Name: "server", Aliases: []string{"s"}, Value: "localhost", Usage: "irc server address", Sources: src("server", "SOULSHACK_SERVER")},
		&cli.BoolFlag{Name: "tls", Aliases: []string{"e"}, Usage: "enable TLS for the IRC connection", Sources: src("tls", "SOULSHACK_TLS")},
		&cli.BoolFlag{Name: "tlsinsecure", Usage: "skip TLS certificate verification", Sources: src("tlsinsecure", "SOULSHACK_TLSINSECURE")},
		&cli.IntFlag{Name: "port", Aliases: []string{"p"}, Value: 6667, Usage: "irc server port", Sources: src("port", "SOULSHACK_PORT")},
		&cli.StringFlag{Name: "channel", Aliases: []string{"c"}, Usage: "irc channel to join", Sources: src("channel", "SOULSHACK_CHANNEL")},
		&cli.StringFlag{Name: "saslnick", Usage: "nick used for SASL", Sources: src("saslnick", "SOULSHACK_SASLNICK")},
		&cli.StringFlag{Name: "saslpass", Usage: "password for SASL plain", Sources: src("saslpass", "SOULSHACK_SASLPASS")},

		// Bot Configuration
		&cli.StringSliceFlag{Name: "admins", Aliases: []string{"A"}, Usage: "comma-separated list of allowed hostmasks to administrate the bot", Sources: src("admins", "SOULSHACK_ADMINS")},
		&cli.BoolFlag{Name: "verbose", Aliases: []string{"V"}, Usage: "enable verbose logging of sessions and configuration", Sources: src("verbose", "SOULSHACK_VERBOSE")},

		// API Configuration
		&cli.StringFlag{Name: "openaikey", Usage: "OpenAI API key", Sources: src("openaikey", "SOULSHACK_OPENAIKEY")},
		&cli.StringFlag{Name: "openaiurl", Usage: "OpenAI API URL (for custom endpoints)", Sources: src("openaiurl", "SOULSHACK_OPENAIURL")},
		&cli.StringFlag{Name: "anthropickey", Usage: "Anthropic API key", Sources: src("anthropickey", "SOULSHACK_ANTHROPICKEY")},
		&cli.StringFlag{Name: "geminikey", Usage: "Google Gemini API key", Sources: src("geminikey", "SOULSHACK_GEMINIKEY")},
		&cli.StringFlag{Name: "ollamaurl", Value: "http://localhost:11434", Usage: "Ollama API URL", Sources: src("ollamaurl", "SOULSHACK_OLLAMAURL")},
		&cli.StringFlag{Name: "ollamakey", Usage: "Ollama API key (Bearer token for authentication)", Sources: src("ollamakey", "SOULSHACK_OLLAMAKEY")},
		&cli.IntFlag{Name: "maxtokens", Value: 4096, Usage: "maximum number of tokens to generate", Sources: src("maxtokens", "SOULSHACK_MAXTOKENS")},
		&cli.StringFlag{Name: "model", Value: "ollama/llama3.2", Usage: "model to be used for responses", Sources: src("model", "SOULSHACK_MODEL")},
		&cli.DurationFlag{Name: "apitimeout", Aliases: []string{"t"}, Value: time.Minute * 5, Usage: "timeout for each completion request", Sources: src("apitimeout", "SOULSHACK_APITIMEOUT")},
		&cli.FloatFlag{Name: "temperature", Value: 0.7, Usage: "temperature for the completion", Sources: src("temperature", "SOULSHACK_TEMPERATURE")},
		&cli.FloatFlag{Name: "top_p", Value: 1.0, Usage: "top P value for the completion", Sources: src("top_p", "SOULSHACK_TOP_P")},
		&cli.BoolFlag{Name: "thinking", Usage: "enable thinking/reasoning for models that support it", Sources: src("thinking", "SOULSHACK_THINKING")},
		&cli.StringSliceFlag{Name: "tool", Usage: "tools to load (shell scripts, MCP server JSON files, or native tools like irc_op)", Sources: src("tool", "SOULSHACK_TOOL")},
		&cli.BoolFlag{Name: "showthinkingaction", Value: true, Usage: "show '[thinking]' IRC action when bot is reasoning", Sources: src("showthinkingaction", "SOULSHACK_SHOWTHINKINGACTION")},
		&cli.BoolFlag{Name: "showtoolactions", Value: true, Usage: "show '[calling toolname]' IRC actions when executing tools", Sources: src("showtoolactions", "SOULSHACK_SHOWTOOLACTIONS")},
		&cli.BoolFlag{Name: "urlwatcher", Usage: "enable passive URL watching and analysis", Sources: src("urlwatcher", "SOULSHACK_URLWATCHER")},

		// Timeouts and Behavior
		&cli.BoolFlag{Name: "addressed", Aliases: []string{"a"}, Value: true, Usage: "require bot be addressed by nick for response", Sources: src("addressed", "SOULSHACK_ADDRESSED")},
		&cli.DurationFlag{Name: "sessionduration", Aliases: []string{"S"}, Value: time.Minute * 10, Usage: "message context will be cleared after it is unused for this duration", Sources: src("sessionduration", "SOULSHACK_SESSIONDURATION")},
		&cli.IntFlag{Name: "sessionhistory", Aliases: []string{"H"}, Value: 250, Usage: "maximum number of lines of context to keep per session", Sources: src("sessionhistory", "SOULSHACK_SESSIONHISTORY")},
		&cli.IntFlag{Name: "chunkmax", Aliases: []string{"m"}, Value: 350, Usage: "maximum number of characters to send as a single message", Sources: src("chunkmax", "SOULSHACK_CHUNKMAX")},

		// Personality / Prompting
		&cli.StringFlag{Name: "greeting", Value: "hello.", Usage: "prompt to be used when the bot joins the channel", Sources: src("greeting", "SOULSHACK_GREETING")},
		&cli.StringFlag{Name: "prompt", Value: "you are a helpful chatbot. do not use caps. do not use emoji.", Usage: "initial system prompt", Sources: src("prompt", "SOULSHACK_PROMPT")},
	}
}

func getConfigPath() string {
	// Check env first
	if v := os.Getenv("SOULSHACK_CONFIG"); v != "" {
		return v
	}
	// Check args
	for i, arg := range os.Args {
		if arg == "--config" || arg == "-b" {
			if i+1 < len(os.Args) {
				return os.Args[i+1]
			}
		}
		if strings.HasPrefix(arg, "--config=") {
			return strings.TrimPrefix(arg, "--config=")
		}
		// Handle -b=... if needed, though standard is space
	}
	return ""
}

func (c *Configuration) PrintConfig() {
	fmt.Printf("nick: %s\n", c.Server.Nick)
	fmt.Printf("server: %s\n", c.Server.Server)
	fmt.Printf("port: %d\n", c.Server.Port)
	fmt.Printf("channel: %s\n", c.Server.Channel)
	fmt.Printf("tls: %t\n", c.Server.SSL)
	fmt.Printf("tlsinsecure: %t\n", c.Server.TLSInsecure)
	fmt.Printf("saslnick: %s\n", c.Server.SASLNick)
	fmt.Printf("saslpass: %s\n", c.Server.SASLPass)
	fmt.Printf("admins: %v\n", c.Bot.Admins)
	fmt.Printf("verbose: %t\n", c.Bot.Verbose)
	fmt.Printf("addressed: %t\n", c.Bot.Addressed)
	fmt.Printf("chunkmax: %d\n", c.Session.ChunkMax)
	fmt.Printf("clienttimeout: %s\n", c.API.Timeout)
	fmt.Printf("maxhistory: %d\n", c.Session.MaxHistory)
	fmt.Printf("maxtokens: %d\n", c.Model.MaxTokens)
	fmt.Printf("tool: %v\n", c.Bot.Tools)
	fmt.Printf("showthinkingaction: %t\n", c.Bot.ShowThinkingAction)
	fmt.Printf("showtoolactions: %t\n", c.Bot.ShowToolActions)
	fmt.Printf("urlwatcher: %t\n", c.Bot.URLWatcher)

	fmt.Printf("sessionduration: %s\n", c.Session.TTL)
	if len(c.API.OpenAIKey) > 3 && c.API.OpenAIKey != "" {
		fmt.Printf("openaikey: %s\n", strings.Repeat("*", len(c.API.OpenAIKey)-3)+c.API.OpenAIKey[len(c.API.OpenAIKey)-3:])
	} else {
		fmt.Printf("openaikey: %s\n", c.API.OpenAIKey)
	}
	if len(c.API.AnthropicKey) > 3 && c.API.AnthropicKey != "" {
		fmt.Printf("anthropickey: %s\n", strings.Repeat("*", len(c.API.AnthropicKey)-3)+c.API.AnthropicKey[len(c.API.AnthropicKey)-3:])
	} else {
		fmt.Printf("anthropickey: %s\n", c.API.AnthropicKey)
	}
	if len(c.API.GeminiKey) > 3 && c.API.GeminiKey != "" {
		fmt.Printf("geminikey: %s\n", strings.Repeat("*", len(c.API.GeminiKey)-3)+c.API.GeminiKey[len(c.API.GeminiKey)-3:])
	} else {
		fmt.Printf("geminikey: %s\n", c.API.GeminiKey)
	}
	fmt.Printf("openaiurl: %s\n", c.API.OpenAIURL)
	fmt.Printf("ollamaurl: %s\n", c.API.OllamaURL)
	fmt.Printf("model: %s\n", c.Model.Model)
	fmt.Printf("temperature: %f\n", c.Model.Temperature)
	fmt.Printf("topp: %f\n", c.Model.TopP)
	fmt.Printf("thinking: %t\n", c.Model.Thinking)
	fmt.Printf("prompt: %s\n", c.Bot.Prompt)
	fmt.Printf("greeting: %s\n", c.Bot.Greeting)
}

func NewConfiguration(c *cli.Command) *Configuration {
	if c.IsSet("config") {
		zap.S().Info("Using config file", "path", c.String("config"))
	}

	config := &Configuration{
		Server: &ServerConfig{
			Nick:        c.String("nick"),
			Server:      c.String("server"),
			Port:        c.Int("port"),
			Channel:     c.String("channel"),
			SSL:         c.Bool("tls"),
			TLSInsecure: c.Bool("tlsinsecure"),
			SASLNick:    c.String("saslnick"),
			SASLPass:    c.String("saslpass"),
		},
		Bot: &BotConfig{
			Admins:             c.StringSlice("admins"),
			Verbose:            c.Bool("verbose"),
			Addressed:          c.Bool("addressed"),
			Prompt:             c.String("prompt"),
			Greeting:           c.String("greeting"),
			Tools:              c.StringSlice("tool"),
			ShowThinkingAction: c.Bool("showthinkingaction"),
			ShowToolActions:    c.Bool("showtoolactions"),
			URLWatcher:         c.Bool("urlwatcher"),
		},
		Model: &ModelConfig{
			Model:       c.String("model"),
			MaxTokens:   c.Int("maxtokens"),
			Temperature: float32(c.Float("temperature")),
			TopP:        float32(c.Float("top_p")),
			Thinking:    c.Bool("thinking"),
		},

		Session: &SessionConfig{
			ChunkMax:   c.Int("chunkmax"),
			MaxHistory: c.Int("sessionhistory"),
			TTL:        c.Duration("sessionduration"),
		},

		API: &APIConfig{
			Timeout:      c.Duration("apitimeout"),
			OpenAIKey:    c.String("openaikey"),
			OpenAIURL:    c.String("openaiurl"),
			AnthropicKey: c.String("anthropickey"),
			GeminiKey:    c.String("geminikey"),
			OllamaURL:    c.String("ollamaurl"),
			OllamaKey:    c.String("ollamakey"),
		},
	}

	return config
}
