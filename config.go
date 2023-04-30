package main

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	ai "github.com/sashabaranov/go-openai"
	"github.com/spf13/cobra"
	vip "github.com/spf13/viper"
)

func init() {

	cobra.OnInitialize(startup)

	// irc configuration
	IrcCmd.PersistentFlags().StringP("nick", "n", "soulshack", "bot's nickname on the irc server")
	IrcCmd.PersistentFlags().StringP("server", "s", "localhost", "irc server address")
	IrcCmd.PersistentFlags().BoolP("ssl", "e", false, "enable SSL for the IRC connection")
	IrcCmd.PersistentFlags().IntP("port", "p", 6667, "irc server port")
	IrcCmd.PersistentFlags().StringP("channel", "c", "soulshack", "irc channel to join")

	// bot meta configuration
	RootCmd.PersistentFlags().StringP("become", "b", "chatbot", "become the named personality")
	RootCmd.PersistentFlags().StringP("directory", "d", "./personalities", "personalities configuration directory")
	RootCmd.PersistentFlags().StringSliceP("admins", "A", []string{}, "comma-separated list of allowed users to administrate the bot (e.g., user1,user2,user3)")

	// informational
	RootCmd.PersistentFlags().BoolP("list", "l", false, "list configured personalities")
	RootCmd.PersistentFlags().BoolP("verbose", "v", false, "enable verbose logging of sessions and configuration")

	// openai configuration
	RootCmd.PersistentFlags().String("openaikey", "", "openai api key")
	RootCmd.PersistentFlags().Int("maxtokens", 512, "maximum number of tokens to generate")
	RootCmd.PersistentFlags().String("model", ai.GPT4, "model to be used for responses (e.g., gpt-4)")
	RootCmd.PersistentFlags().Float64("temperature", 0.3, "temperature for the completion response")

	// timeouts and behavior
	RootCmd.PersistentFlags().BoolP("addressed", "a", true, "require bot be addressed by nick for response")
	RootCmd.PersistentFlags().DurationP("session", "S", time.Minute*3, "duration for the chat session; message context will be cleared after this time")
	RootCmd.PersistentFlags().DurationP("timeout", "t", time.Second*120, "timeout for each message from the bot")
	RootCmd.PersistentFlags().IntP("history", "H", 15, "maximum number of lines of context to keep per session")
	RootCmd.PersistentFlags().DurationP("chunkdelay", "C", time.Second*15, "after this delay, bot will look to split the incoming buffer on sentence boundaries")
	RootCmd.PersistentFlags().IntP("chunkmax", "m", 350, "maximum number of characters to send as a single message")

	// personality / prompting
	RootCmd.PersistentFlags().String("greeting", "hello.", "prompt to be used when the bot joins the channel")
	RootCmd.PersistentFlags().String("prompt", "respond in a short text:", "initial system prompt for the ai")

	// discord??
	DiscordCmd.PersistentFlags().String("discordtoken", "", "discord bot token")

	vip.BindPFlags(RootCmd.PersistentFlags())
	vip.BindPFlags(IrcCmd.PersistentFlags())
	vip.BindPFlags(DiscordCmd.PersistentFlags())

	vip.SetEnvPrefix("SOULSHACK")
	vip.AutomaticEnv()
}

func startup() {

	fmt.Println(GetBanner())

	if _, err := os.Stat(vip.GetString("directory")); errors.Is(err, fs.ErrNotExist) {
		log.Printf("? configuration directory %s does not exist", vip.GetString("directory"))
	}

	if vip.GetBool("list") {
		personalities := ListConfigs()
		log.Printf("Available personalities: %s", strings.Join(personalities, ", "))
		os.Exit(0)
	}

	vip.AddConfigPath(vip.GetString("directory"))
	vip.SetConfigName(vip.GetString("become"))

	if err := vip.ReadInConfig(); err != nil {
		log.Println("? no personality file found:", vip.GetString("become"))
	} else {
		log.Println("using personality file:", vip.ConfigFileUsed())
	}
}

func VerifyConfig(v *vip.Viper) error {
	for _, varName := range v.AllKeys() {
		if varName == "admins" || varName == "discordtoken" {
			continue
		}
		value := v.GetString(varName)
		if value == "" {
			return fmt.Errorf("! %s unset. use --%s flag, personality config, or SOULSHACK_%s env", varName, varName, strings.ToUpper(varName))
		}

		if v.GetBool("verbose") {
			if varName == "openaikey" {
				value = strings.Repeat("*", len(value))
			}
			log.Printf("\t%s: '%s'", varName, value)
		}
	}
	return nil
}

func ListConfigs() []string {
	files, err := os.ReadDir(vip.GetString("directory"))
	if err != nil {
		log.Fatal(err)
	}
	var personalities []string
	for _, file := range files {
		if filepath.Ext(file.Name()) == ".yml" {
			personalities = append(personalities, strings.TrimSuffix(file.Name(), filepath.Ext(file.Name())))
		}
	}
	return personalities
}

func LoadConfig(p string) (*vip.Viper, error) {
	log.Println("loading personality:", p)
	conf := vip.New()
	conf.SetConfigFile(vip.GetString("directory") + "/" + p + ".yml")

	err := conf.ReadInConfig()
	if err != nil {
		log.Println("Error reading personality config:", err)
		return nil, err
	}
	return conf, nil
}
