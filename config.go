package main

import (
	"fmt"
	"log"
	"time"

	"github.com/sashabaranov/go-openai"
	"github.com/spf13/cobra"
	vip "github.com/spf13/viper"
)

var ModifiableConfigKeys = []string{"nick", "channel", "model", "addressed", "prompt", "maxtokens", "tempurature", "admins"}
var BotConfig *Configuration

type Configuration struct {
	Nick          string
	Server        string
	Port          int
	Channel       string
	SSL           bool
	TLSInsecure   bool
	SASLNick      string
	SASLPass      string
	Admins        []string
	Verbose       bool
	Addressed     bool
	ChunkDelay    time.Duration
	ChunkMax      int
	ChunkQuoted   bool
	ClientTimeout time.Duration
	MaxHistory    int
	MaxTokens     int
	ReactMode     bool
	TTL           time.Duration
	APIKey        string
	Model         string
	Tempurature   float32
	URL           string
	Prompt        string
	Greeting      string
	OpenAI        openai.ClientConfig
}

func loadConfig() {
	configfile := vip.GetString("config")
	if configfile != "" {
		vip.SetConfigFile(configfile)
		if err := vip.ReadInConfig(); err != nil {
			log.Printf("config file %s not found", configfile)
		} else {
			log.Println("using config file:", vip.ConfigFileUsed())
		}
	}

	BotConfig = &Configuration{
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
		ReactMode:     vip.GetBool("reactmode"),
		TTL:           vip.GetDuration("sessionduration"),
		APIKey:        vip.GetString("openaikey"),
		Model:         vip.GetString("model"),
		Tempurature:   float32(vip.GetFloat64("tempurature")),
		URL:           vip.GetString("openaiurl"),
		Prompt:        vip.GetString("prompt"),
		Greeting:      vip.GetString("greeting"),
		OpenAI:        openai.DefaultConfig(vip.GetString("openaikey")),
	}

	if vip.GetString("openaiurl") != "" {
		log.Println("using alternate OpenAI API URL:", vip.GetString("openaiurl"))
		BotConfig.OpenAI.BaseURL = vip.GetString("openaiurl")
	}

	// Verify required configuration settings
	if err := verifyConfig(); err != nil {
		log.Fatalf("invalid configuration: %v", err)
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
	root.PersistentFlags().StringSliceP("admins", "A", []string{}, "comma-separated list of allowed users to administrate the bot (e.g., user1,user2,user3)")

	// informational
	root.PersistentFlags().BoolP("verbose", "v", false, "enable verbose logging of sessions and configuration")

	// openai configuration
	root.PersistentFlags().String("openaikey", "", "openai api key")
	root.PersistentFlags().Int("maxtokens", 512, "maximum number of tokens to generate")
	root.PersistentFlags().String("model", openai.GPT4o, "model to be used for responses")
	root.PersistentFlags().String("openaiurl", "", "alternative base url to use instead of openai")
	root.PersistentFlags().DurationP("apitimeout", "t", time.Minute*5, "timeout for each completion request to openai")
	root.PersistentFlags().Float32("tempurature", 0.7, "temperature for the completion")

	// timeouts and behavior
	root.PersistentFlags().BoolP("addressed", "a", true, "require bot be addressed by nick for response")
	root.PersistentFlags().DurationP("sessionduration", "S", time.Minute*3, "duration for the chat session; message context will be cleared after this time")
	root.PersistentFlags().IntP("sessionhistory", "H", 15, "maximum number of lines of context to keep per session")
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
	required := []string{"nick", "server", "channel", "openaikey"}
	for _, key := range required {
		if vip.GetString(key) == "" {
			return fmt.Errorf("missing required config: %s", key)
		}
	}
	return nil
}
