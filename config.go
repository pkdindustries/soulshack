package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/neurosnap/sentences"
	"github.com/neurosnap/sentences/english"
	"github.com/sashabaranov/go-openai"
	"github.com/spf13/cobra"
	vip "github.com/spf13/viper"
)

var ModifiableConfigKeys = []string{
	"nick",
	"channel",
	"model",
	"addressed",
	"prompt",
	"maxtokens",
	"temperature",
	"top_p",
	"admins",
	"tools",
}

var Config *Configuration

type Configuration struct {
	// bot
	Nick        string
	Server      string
	Port        int
	Channel     string
	SSL         bool
	TLSInsecure bool
	SASLNick    string
	SASLPass    string
	Admins      []string
	Verbose     bool
	Addressed   bool
	Prompt      string
	Greeting    string

	// ai
	OpenAIConfig  openai.ClientConfig
	OpenAiClient  *openai.Client
	ClientTimeout time.Duration
	MaxTokens     int
	APIKey        string
	Model         string
	Temperature   float32
	TopP          float32

	// tools
	ToolRegistry *ToolRegistry
	ToolsDir     string
	Tools        bool
	//ToolTag      string
	//ToolsInline  bool

	// sessions
	Sessions        *Sessions
	ChunkDelay      time.Duration
	ChunkMax        int
	ChunkQuoted     bool
	MaxHistory      int
	SessionDuration time.Duration

	// tokenizer
	Tokenizer *sentences.DefaultSentenceTokenizer
}

func (c *Configuration) PrintConfig() {
	fmt.Printf("[configuration]\n")
	fmt.Printf("nick: %s\n", c.Nick)
	fmt.Printf("server: %s\n", c.Server)
	fmt.Printf("port: %d\n", c.Port)
	fmt.Printf("channel: %s\n", c.Channel)
	fmt.Printf("tls: %t\n", c.SSL)
	fmt.Printf("tlsinsecure: %t\n", c.TLSInsecure)
	fmt.Printf("saslnick: %s\n", c.SASLNick)
	fmt.Printf("saslpass: %s\n", c.SASLPass)
	fmt.Printf("admins: %v\n", c.Admins)
	fmt.Printf("verbose: %t\n", c.Verbose)
	fmt.Printf("addressed: %t\n", c.Addressed)
	fmt.Printf("chunkdelay: %s\n", c.ChunkDelay)
	fmt.Printf("chunkmax: %d\n", c.ChunkMax)
	fmt.Printf("chunkquoted: %t\n", c.ChunkQuoted)
	fmt.Printf("clienttimeout: %s\n", c.ClientTimeout)
	fmt.Printf("maxhistory: %d\n", c.MaxHistory)
	fmt.Printf("maxtokens: %d\n", c.MaxTokens)
	fmt.Printf("tools: %t\n", c.Tools)
	fmt.Printf("toolsdir: %s\n", c.ToolsDir)
	//fmt.Printf("tooltag: %s\n", c.ToolTag)
	//fmt.Printf("toolsinline: %t\n", c.ToolsInline)
	fmt.Printf("sessionduration: %s\n", c.SessionDuration)
	if len(c.APIKey) > 3 && c.APIKey != "" {
		fmt.Printf("openapikey: %s\n", strings.Repeat("*", len(c.APIKey)-3)+c.APIKey[len(c.APIKey)-3:])
	} else {
		fmt.Printf("openapikey: %s\n", c.APIKey)
	}
	fmt.Printf("openaiurl: %s\n", c.OpenAIConfig.BaseURL)
	fmt.Printf("model: %s\n", c.Model)
	fmt.Printf("temperature: %f\n", c.Temperature)
	fmt.Printf("topp: %f\n", c.TopP)
	fmt.Printf("prompt: %s\n", c.Prompt)
	fmt.Printf("greeting: %s\n", c.Greeting)
}

func loadConfig() {
	configfile := vip.GetString("config")
	if configfile != "" {
		vip.SetConfigFile(configfile)
		if err := vip.ReadInConfig(); err != nil {
			log.Fatalf("config file %s not found", configfile)
		} else {
			log.Println("using config file:", vip.ConfigFileUsed())
		}
	}

	Config = &Configuration{
		Nick:          vip.GetString("nick"),
		Server:        vip.GetString("server"),
		Port:          vip.GetInt("port"),
		Channel:       vip.GetString("channel"),
		SSL:           vip.GetBool("tls"),
		TLSInsecure:   vip.GetBool("tlsinsecure"),
		SASLNick:      vip.GetString("saslnick"),
		SASLPass:      vip.GetString("saslpass"),
		Admins:        vip.GetStringSlice("admins"),
		Verbose:       vip.GetBool("verbose"),
		Addressed:     vip.GetBool("addressed"),
		ChunkDelay:    vip.GetDuration("chunkdelay"),
		ChunkMax:      vip.GetInt("chunkmax"),
		ChunkQuoted:   vip.GetBool("chunkquoted"),
		ClientTimeout: vip.GetDuration("apitimeout"),
		MaxHistory:    vip.GetInt("sessionhistory"),
		MaxTokens:     vip.GetInt("maxtokens"),
		Tools:         vip.GetBool("tools"),
		ToolsDir:      vip.GetString("toolsdir"),
		//ToolTag:       vip.GetString("tooltag"),
		//ToolsInline:     vip.GetBool("toolsinline"),
		SessionDuration: vip.GetDuration("sessionduration"),
		APIKey:          vip.GetString("openaikey"),
		Model:           vip.GetString("model"),
		Temperature:     float32(vip.GetFloat64("temperature")),
		TopP:            float32(vip.GetFloat64("top_p")),
		Prompt:          vip.GetString("prompt"),
		Greeting:        vip.GetString("greeting"),
		OpenAIConfig:    openai.DefaultConfig(vip.GetString("openaikey")),
	}

	// initialize the ai client
	baseurl := vip.GetString("openaiurl")
	if baseurl != "" {
		log.Println("using alternate OpenAI API URL:", baseurl)
		Config.OpenAIConfig.BaseURL = baseurl
	}

	Config.OpenAiClient = openai.NewClientWithConfig(Config.OpenAIConfig)

	// initialize tools
	if Config.Tools {
		toolsDir := vip.GetString("toolsdir")
		registry, err := NewToolRegistry(toolsDir)
		if err != nil {
			log.Println("failed to initialize tools:", err)
			Config.Tools = false
		} else {
			RegisterIrcTools(registry)
			Config.ToolRegistry = registry
		}
	}

	// initialize tokenizer
	tokenizer, err := english.NewSentenceTokenizer(nil)
	if err != nil {
		log.Fatal("Error creating tokenizer:", err)
	}
	Config.Tokenizer = tokenizer

	// initialize sessions
	Config.Sessions = &Sessions{}

	if err := verifyConfig(); err != nil {
		fmt.Println("")
		fmt.Println("invalid configuration,", err)
		fmt.Println("use soulshack --help for more information")
		os.Exit(-1)
	}
}

func InitializeConfig() {

	cobra.OnInitialize(loadConfig)

	// irc client configuration
	root.PersistentFlags().StringP("nick", "n", "soulshack", "bot's nickname on the irc server")
	root.PersistentFlags().StringP("server", "s", "localhost", "irc server address")
	root.PersistentFlags().BoolP("tls", "e", false, "enable TLS for the IRC connection")
	root.PersistentFlags().BoolP("tlsinsecure", "", false, "skip TLS certificate verification")
	root.PersistentFlags().IntP("port", "p", 6667, "irc server port")
	root.PersistentFlags().StringP("channel", "c", "", "irc channel to join")
	root.PersistentFlags().StringP("saslnick", "", "", "nick used for SASL")
	root.PersistentFlags().StringP("saslpass", "", "", "password for SASL plain")

	// bot configuration
	root.PersistentFlags().StringP("config", "b", "", "use the named configuration file")
	root.PersistentFlags().StringSliceP("admins", "A", []string{}, "comma-separated list of allowed hostmasks to administrate the bot (e.g. alex!~alex@localhost, josh!~josh@localhost)")

	// informational
	root.PersistentFlags().BoolP("verbose", "v", false, "enable verbose logging of sessions and configuration")

	// openai configuration
	root.PersistentFlags().String("openaikey", "", "openai api key")
	root.PersistentFlags().Int("maxtokens", 512, "maximum number of tokens to generate")
	root.PersistentFlags().String("model", openai.GPT4o, "model to be used for responses")
	root.PersistentFlags().String("openaiurl", "", "alternative base url to use instead of openai")
	root.PersistentFlags().DurationP("apitimeout", "t", time.Minute*5, "timeout for each completion request")
	root.PersistentFlags().Float32("temperature", 0.7, "temperature for the completion")
	root.PersistentFlags().Float32("top_p", 1, "top P value for the completion")
	root.PersistentFlags().Bool("tools", false, "enable tool use")
	//root.PersistentFlags().Bool("toolsinline", false, "enable inline tool use")
	root.PersistentFlags().String("toolsdir", "examples/tools", "directory to load tools from")
	root.PersistentFlags().String("tooltag", "tool_call", "tag llm uses for inline tool commands")

	// timeouts and behavior
	root.PersistentFlags().BoolP("addressed", "a", true, "require bot be addressed by nick for response")
	root.PersistentFlags().DurationP("sessionduration", "S", time.Minute*3, "duration for the chat session; message context will be cleared after this time")
	root.PersistentFlags().IntP("sessionhistory", "H", 250, "maximum number of lines of context to keep per session")
	root.PersistentFlags().DurationP("chunkdelay", "C", time.Second*15, "after this delay, bot will look to split the incoming buffer on sentence boundaries")
	root.PersistentFlags().IntP("chunkmax", "m", 350, "maximum number of characters to send as a single message")

	// personality / prompting
	root.PersistentFlags().String("greeting", "hello.", "prompt to be used when the bot joins the channel")
	root.PersistentFlags().String("prompt", "you are a helpful chatbot. do not use caps. do not use emoji.", "initial system prompt")

	vip.BindPFlags(root.PersistentFlags())

	vip.SetEnvPrefix("SOULSHACK")
	vip.AutomaticEnv()

}

func verifyConfig() error {
	if Config.Verbose {
		Config.PrintConfig()
	}

	if Config.OpenAIConfig.BaseURL == "https://api.openai.com/v1" {
		if vip.GetString("openaikey") == "" {
			return fmt.Errorf("missing required configuration key: %s", "openaikey")
		}
	}

	if Config.Channel == "" {
		return fmt.Errorf("missing required config: %s", "channel")
	}

	return nil
}
