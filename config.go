package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/sashabaranov/go-openai"
	vip "github.com/spf13/viper"
)

var ModifiableConfigKeys = []string{
	"model",
	"addressed",
	"prompt",
	"maxtokens",
	"temperature",
	"top_p",
	"admins",
	"tools",
	"openaiurl",
	"ollamaurl",
}

type ModelConfig struct {
	Model       string
	MaxTokens   int
	Temperature float32
	TopP        float32
}

type BotConfig struct {
	Admins       []string
	Verbose      bool
	Addressed    bool
	Prompt       string
	Greeting     string
	ToolsDir     string
	ToolsEnabled bool
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
	OpenAIKey     string
	OpenAIURL     string
	AnthropicKey  string
	GeminiKey     string
	OllamaURL     string
	Timeout       time.Duration
}

type Configuration struct {
	Server  *ServerConfig
	Bot     *BotConfig
	Model   *ModelConfig
	Session *SessionConfig
	API     *APIConfig
}

type SystemImpl struct {
	Store SessionStore
	LLM   LLM
	Tools *ToolRegistry
}

func (s *SystemImpl) GetLLM() LLM {
	return s.LLM
}

func (s *SystemImpl) GetToolRegistry() *ToolRegistry {
	return s.Tools
}

func (s *SystemImpl) SetToolRegistry(reg *ToolRegistry) {
	s.Tools = reg
}

func (s *SystemImpl) GetSessionStore() SessionStore {
	return s.Store
}

func NewSystem(c *Configuration) System {
	s := SystemImpl{}
	// initialize tools
	if c.Bot.ToolsEnabled {
		toolsDir := vip.GetString("toolsdir")
		registry, err := NewToolRegistry(toolsDir)
		if err != nil {
			log.Println("config: failed to initialize tools:", err)
			c.Bot.ToolsEnabled = false
		} else {
			RegisterIrcTools(registry)
			s.Tools = registry
		}
	} else {
		log.Println("config: tools are disabled")
		s.Tools = &ToolRegistry{}
	}

	// initialize the api for completions using MultiPass
	s.LLM = NewMultiPass(*c.API)

	// initialize sessions
	s.Store = NewSessionStore(c)
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
	fmt.Printf("tools: %t\n", c.Bot.ToolsEnabled)
	fmt.Printf("toolsdir: %s\n", c.Bot.ToolsDir)

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
	fmt.Printf("prompt: %s\n", c.Bot.Prompt)
	fmt.Printf("greeting: %s\n", c.Bot.Greeting)
}

func NewConfiguration() *Configuration {
	configfile := vip.GetString("config")
	if configfile != "" {
		vip.SetConfigFile(configfile)
		if err := vip.ReadInConfig(); err != nil {
			log.Fatal("config: config file not found", configfile)
		} else {
			log.Println("config: using config file:", vip.ConfigFileUsed())
		}
	}

	config := &Configuration{
		Server: &ServerConfig{
			Nick:        vip.GetString("nick"),
			Server:      vip.GetString("server"),
			Port:        vip.GetInt("port"),
			Channel:     vip.GetString("channel"),
			SSL:         vip.GetBool("tls"),
			TLSInsecure: vip.GetBool("tlsinsecure"),
			SASLNick:    vip.GetString("saslnick"),
			SASLPass:    vip.GetString("saslpass"),
		},
		Bot: &BotConfig{
			Admins:       vip.GetStringSlice("admins"),
			Verbose:      vip.GetBool("verbose"),
			Addressed:    vip.GetBool("addressed"),
			Prompt:       vip.GetString("prompt"),
			Greeting:     vip.GetString("greeting"),
			ToolsEnabled: vip.GetBool("tools"),
			ToolsDir:     vip.GetString("toolsdir"),
		},
		Model: &ModelConfig{
			Model:       vip.GetString("model"),
			MaxTokens:   vip.GetInt("maxtokens"),
			Temperature: float32(vip.GetFloat64("temperature")),
			TopP:        float32(vip.GetFloat64("top_p")),
		},

		Session: &SessionConfig{
			ChunkMax:   vip.GetInt("chunkmax"),
			MaxHistory: vip.GetInt("sessionhistory"),
			TTL:        vip.GetDuration("sessionduration"),
		},

		API: &APIConfig{
			Timeout:      vip.GetDuration("apitimeout"),
			OpenAIKey:    vip.GetString("openaikey"),
			OpenAIURL:    vip.GetString("openaiurl"),
			AnthropicKey: vip.GetString("anthropickey"),
			GeminiKey:    vip.GetString("geminikey"),
			OllamaURL:    vip.GetString("ollamaurl"),
		},
	}

	return config
}

func initializeConfig() {
	cmd := root
	// irc client configuration
	cmd.PersistentFlags().StringP("nick", "n", "soulshack", "bot's nickname on the irc server")
	cmd.PersistentFlags().StringP("server", "s", "localhost", "irc server address")
	cmd.PersistentFlags().BoolP("tls", "e", false, "enable TLS for the IRC connection")
	cmd.PersistentFlags().BoolP("tlsinsecure", "", false, "skip TLS certificate verification")
	cmd.PersistentFlags().IntP("port", "p", 6667, "irc server port")
	cmd.PersistentFlags().StringP("channel", "c", "", "irc channel to join")
	cmd.PersistentFlags().StringP("saslnick", "", "", "nick used for SASL")
	cmd.PersistentFlags().StringP("saslpass", "", "", "password for SASL plain")

	// bot configuration
	cmd.PersistentFlags().StringP("config", "b", "", "use the named configuration file")
	cmd.PersistentFlags().StringSliceP("admins", "A", []string{}, "comma-separated list of allowed hostmasks to administrate the bot (e.g. alex!~alex@localhost, josh!~josh@localhost)")

	// informational
	cmd.PersistentFlags().BoolP("verbose", "v", false, "enable verbose logging of sessions and configuration")

	// API configuration
	cmd.PersistentFlags().String("openaikey", "", "OpenAI API key")
	cmd.PersistentFlags().String("openaiurl", "", "OpenAI API URL (for custom endpoints)")
	cmd.PersistentFlags().String("anthropickey", "", "Anthropic API key")
	cmd.PersistentFlags().String("geminikey", "", "Google Gemini API key")
	cmd.PersistentFlags().String("ollamaurl", "", "Ollama API URL (default: http://localhost:11434)")
	cmd.PersistentFlags().Int("maxtokens", 512, "maximum number of tokens to generate")
	cmd.PersistentFlags().String("model", openai.GPT4o, "model to be used for responses")
	cmd.PersistentFlags().DurationP("apitimeout", "t", time.Minute*5, "timeout for each completion request")
	cmd.PersistentFlags().Float32("temperature", 0.7, "temperature for the completion")
	cmd.PersistentFlags().Float32("top_p", 1, "top P value for the completion")
	cmd.PersistentFlags().Bool("tools", false, "enable tool use")
	cmd.PersistentFlags().String("toolsdir", "examples/tools", "directory to load tools from")

	// timeouts and behavior
	cmd.PersistentFlags().BoolP("addressed", "a", true, "require bot be addressed by nick for response")
	cmd.PersistentFlags().DurationP("sessionduration", "S", time.Minute*3, "message context will be cleared after it is unused for this duration")
	cmd.PersistentFlags().IntP("sessionhistory", "H", 250, "maximum number of lines of context to keep per session")
	cmd.PersistentFlags().IntP("chunkmax", "m", 350, "maximum number of characters to send as a single message")

	// personality / prompting
	cmd.PersistentFlags().String("greeting", "hello.", "prompt to be used when the bot joins the channel")
	cmd.PersistentFlags().String("prompt", "you are a helpful chatbot. do not use caps. do not use emoji.", "initial system prompt")

	vip.BindPFlags(cmd.PersistentFlags())

	vip.SetEnvPrefix("SOULSHACK")
	vip.AutomaticEnv()
}
