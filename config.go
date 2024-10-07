package main

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"strings"
	"time"

	"github.com/sashabaranov/go-openai"
	"github.com/spf13/cobra"
	vip "github.com/spf13/viper"
)

type Config struct {
	Nick          string
	Server        string
	Port          int
	Channel       string
	SSL           bool
	TLSInsecure   bool
	SASLNick      string
	SASLPass      string
	Admins        []string
	Directory     string
	Verbose       bool
	Addressed     bool
	Chunkdelay    time.Duration
	Chunkmax      int
	Chunkquoted   bool
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
	Goodbye       string
	OpenAi        openai.ClientConfig
}

func LoadConfig() *Config {
	config := &Config{
		Nick:          vip.GetString("nick"),
		Server:        vip.GetString("server"),
		Port:          vip.GetInt("port"),
		Channel:       vip.GetString("channel"),
		SSL:           vip.GetBool("tls"),
		TLSInsecure:   vip.GetBool("tlsinsecure"),
		SASLNick:      vip.GetString("saslnick"),
		SASLPass:      vip.GetString("saslpass"),
		Admins:        vip.GetStringSlice("admins"),
		Directory:     vip.GetString("directory"),
		Verbose:       vip.GetBool("verbose"),
		Addressed:     vip.GetBool("addressed"),
		Chunkdelay:    vip.GetDuration("chunkdelay"),
		Chunkmax:      vip.GetInt("chunkmax"),
		Chunkquoted:   vip.GetBool("chunkquoted"),
		ClientTimeout: vip.GetDuration("apitimeout"),
		MaxHistory:    vip.GetInt("sessionhistory"),
		MaxTokens:     vip.GetInt("maxtokens"),
		ReactMode:     vip.GetBool("reactmode"),
		TTL:           vip.GetDuration("sessionduration"),
		APIKey:        vip.GetString("openaikey"),
		Model:         vip.GetString("model"),
		URL:           vip.GetString("openaiurl"),
		Prompt:        vip.GetString("prompt"),
		Greeting:      vip.GetString("greeting"),
		Goodbye:       vip.GetString("goodbye"),
		OpenAi:        openai.DefaultConfig(vip.GetString("openaikey")),
	}

	if vip.GetString("openaiurl") != "" {
		log.Println("using alternate OpenAI API URL:", vip.GetString("openaiurl"))
		config.OpenAi.BaseURL = vip.GetString("openaiurl")
	}

	return config
}

func init() {

	cobra.OnInitialize(func() {
		fmt.Println(getBanner())

		if _, err := os.Stat(vip.GetString("directory")); errors.Is(err, fs.ErrNotExist) {
			log.Printf("? configuration directory %s does not exist", vip.GetString("directory"))
		}

		vip.AddConfigPath(vip.GetString("directory"))
		vip.SetConfigName(vip.GetString("config"))

		if err := vip.ReadInConfig(); err != nil {
			log.Println("? no config file found:", vip.GetString("config"))
		} else {
			log.Println("using config file:", vip.ConfigFileUsed())
		}
	})

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
	root.PersistentFlags().StringP("config", "b", "chatbot", "become the named configuration")
	root.PersistentFlags().StringP("directory", "d", "./config", "configuration directory")
	root.PersistentFlags().StringSliceP("admins", "A", []string{}, "comma-separated list of allowed users to administrate the bot (e.g., user1,user2,user3)")

	// informational
	root.PersistentFlags().BoolP("verbose", "v", false, "enable verbose logging of sessions and configuration")

	// openai configuration
	root.PersistentFlags().String("openaikey", "", "openai api key")
	root.PersistentFlags().Int("maxtokens", 512, "maximum number of tokens to generate")
	root.PersistentFlags().String("model", openai.GPT4o, "model to be used for responses (e.g., gpt-4)")
	root.PersistentFlags().String("openaiurl", "", "alternative base url to use instead of openai")
	root.PersistentFlags().DurationP("apitimeout", "t", time.Second*60, "timeout for each completion request to openai")
	root.PersistentFlags().Float32("tempurature", 0.7, "temperature for the completion")

	// timeouts and behavior
	root.PersistentFlags().BoolP("addressed", "a", true, "require bot be addressed by nick for response")
	root.PersistentFlags().DurationP("sessionduration", "S", time.Minute*3, "duration for the chat session; message context will be cleared after this time")
	root.PersistentFlags().IntP("sessionhistory", "H", 15, "maximum number of lines of context to keep per session")
	root.PersistentFlags().DurationP("chunkdelay", "C", time.Second*15, "after this delay, bot will look to split the incoming buffer on sentence boundaries")
	root.PersistentFlags().IntP("chunkmax", "m", 350, "maximum number of characters to send as a single message")

	// personality / prompting
	root.PersistentFlags().String("goodbye", "goodbye.", "prompt to be used when the bot leaves the channel")
	root.PersistentFlags().String("greeting", "hello.", "prompt to be used when the bot joins the channel")
	root.PersistentFlags().String("prompt", "respond in a short text:", "initial system prompt for the ai")

	vip.BindPFlags(root.PersistentFlags())

	vip.SetEnvPrefix("SOULSHACK")
	vip.AutomaticEnv()
}

func (c *Config) VerifyConfig() error {
	for _, varName := range vip.AllKeys() {
		if varName == "admins" || varName == "openaiurl" || varName == "saslnick" || varName == "saslpass" {
			continue
		}
		value := vip.GetString(varName)
		if value == "" {
			return fmt.Errorf("! %s unset. use --%s flag, personality config, or SOULSHACK_%s env", varName, varName, strings.ToUpper(varName))
		}

		if c.Verbose {
			if varName == "openaikey" {
				value = strings.Repeat("*", len(value))
			}
			log.Printf("\t%s: '%s'", varName, value)
		}
	}
	return nil
}
