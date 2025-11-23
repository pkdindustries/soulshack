package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/alexschlessinger/pollytool/sessions"
	"github.com/alexschlessinger/pollytool/tools"
	"github.com/urfave/cli/v2"
	"go.uber.org/zap"
)

var ModifiableConfigKeys = []string{
	"model",
	"addressed",
	"prompt",
	"maxtokens",
	"temperature",
	"top_p",
	"thinking",
	"admins",
	"openaiurl",
	"ollamaurl",
	"ollamakey",
	"openaikey",
	"anthropickey",
	"geminikey",
	"tools",
	"showthinkingaction",
	"showtoolactions",
	"sessionduration",
	"apitimeout",
	"sessionhistory",
	"chunkmax",
}

type ModelConfig struct {
	Model       string
	MaxTokens   int
	Temperature float32
	TopP        float32
	Thinking    bool
}

type BotConfig struct {
	Admins             []string
	Verbose            bool
	Addressed          bool
	Prompt             string
	Greeting           string
	Tools              []string // Unified list of tools (shell, MCP, and IRC tools)
	ShowThinkingAction bool     // Whether to show "[thinking]" IRC action
	ShowToolActions    bool     // Whether to show "[calling toolname]" IRC actions
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

type SessionConfig struct {
	MaxHistory int
	TTL        time.Duration
	ChunkMax   int
}

type APIConfig struct {
	OpenAIKey    string
	OpenAIURL    string
	AnthropicKey string
	GeminiKey    string
	OllamaURL    string
	OllamaKey    string
	Timeout      time.Duration
}

type Configuration struct {
	Server  *ServerConfig
	Bot     *BotConfig
	Model   *ModelConfig
	Session *SessionConfig
	API     *APIConfig
}

type System interface {
	GetToolRegistry() *tools.ToolRegistry
	GetSessionStore() sessions.SessionStore
	GetHistory() HistoryStore
}

type SystemImpl struct {
	Store   sessions.SessionStore
	Tools   *tools.ToolRegistry
	History HistoryStore
}

func (s *SystemImpl) GetToolRegistry() *tools.ToolRegistry {
	return s.Tools
}

func (s *SystemImpl) SetToolRegistry(reg *tools.ToolRegistry) {
	s.Tools = reg
}

func (s *SystemImpl) GetSessionStore() sessions.SessionStore {
	return s.Store
}

func (s *SystemImpl) GetHistory() HistoryStore {
	return s.History
}

func NewSystem(c *Configuration) System {
	s := SystemImpl{}
	// Initialize empty tool registry
	s.Tools = tools.NewToolRegistry([]tools.Tool{})

	// Register native IRC tools with polly's registry
	RegisterIRCTools(s.Tools)

	// Load all tools from configuration (polly now handles native, shell, and MCP tools)
	if len(c.Bot.Tools) > 0 {
		for _, toolSpec := range c.Bot.Tools {
			if _, err := s.Tools.LoadToolAuto(toolSpec); err != nil {
				zap.S().Warnw("Warning loading tool", "tool", toolSpec, "error", err)
				continue
			}
		}
	}
	zap.S().Infow("Loaded tools", "count", len(s.Tools.All()))

	// initialize sessions with pollytool's SyncMapSessionStore
	zap.S().Info("Initialized session store: syncmap")

	s.Store = sessions.NewSyncMapSessionStore(&sessions.Metadata{
		MaxHistory:   c.Session.MaxHistory,
		TTL:          c.Session.TTL,
		SystemPrompt: c.Bot.Prompt,
	})

	// Initialize history store
	history, err := NewFileHistory(".history") // Assuming NewFileHistory is the correct function, not NewHistory
	if err != nil {
		zap.S().Warnw("Failed to initialize history store", "error", err)
	} else {
		s.History = history
	}

	return &s
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

func NewConfiguration(c *cli.Context) *Configuration {
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
		},
		Model: &ModelConfig{
			Model:       c.String("model"),
			MaxTokens:   c.Int("maxtokens"),
			Temperature: float32(c.Float64("temperature")),
			TopP:        float32(c.Float64("top_p")),
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
