package main

//  ____                    _   ____    _                      _
// / ___|    ___    _   _  | | / ___|  | |__     __ _    ___  | | __
// \___ \   / _ \  | | | | | | \___ \  | '_ \   / _` |  / __| | |/ /
//  ___) | | (_) | | |_| | | |  ___) | | | | | | (_| | | (__  |   <
// |____/   \___/   \__,_| |_| |____/  |_| |_|  \__,_|  \___| |_|\_\
//  .  .  .  because  real  people  are  overrated

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"pkdindustries/soulshack/config"
	"pkdindustries/soulshack/discord"
	"pkdindustries/soulshack/irc"
	"strings"
	"time"

	"github.com/common-nighthawk/go-figure"
	ai "github.com/sashabaranov/go-openai"
	"github.com/spf13/cobra"
	vip "github.com/spf13/viper"
)

var RootCmd = &cobra.Command{
	Use:     "soulshack",
	Example: "soulshack [irc|discord]",
	Short:   GetBanner(),
	Version: "0.50 - http://github.com/pkdindustries/soulshack",
}

var IrcCmd = &cobra.Command{
	Use:     "irc",
	Example: "soulshack irc",
	Short:   "run soulshack as an irc bot",
	Run:     runIrc,
}

var DiscordCmd = &cobra.Command{
	Use:     "discord",
	Example: "soulshack discord",
	Short:   "run soulshack as a discord bot",
	Run:     runDiscord,
}

func main() {
	cobra.OnInitialize(func() {
		fmt.Println(GetBanner())
		if _, err := os.Stat(vip.GetString("directory")); errors.Is(err, fs.ErrNotExist) {
			log.Printf("? configuration directory %s does not exist", vip.GetString("directory"))
		}
		if vip.GetBool("list") {
			personalities := config.List()
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
	})
	// irc configuration
	RootCmd.PersistentFlags().StringP("nick", "n", "soulshack", "bot's nickname on the irc server")
	RootCmd.PersistentFlags().StringP("server", "s", "localhost", "irc server address")
	RootCmd.PersistentFlags().BoolP("ssl", "e", false, "enable SSL for the IRC connection")
	RootCmd.PersistentFlags().IntP("port", "p", 6667, "irc server port")
	RootCmd.PersistentFlags().StringP("channel", "c", "soulshack", "irc channel to join")

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
	RootCmd.PersistentFlags().String("discordtoken", "", "discord bot token")

	vip.BindPFlags(RootCmd.PersistentFlags())
	// vip.BindPFlags(IrcCmd.PersistentFlags())
	// vip.BindPFlags(DiscordCmd.PersistentFlags())

	vip.SetEnvPrefix("SOULSHACK")
	vip.AutomaticEnv()
	RootCmd.AddCommand(IrcCmd)
	RootCmd.AddCommand(DiscordCmd)
	if err := RootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func runIrc(_ *cobra.Command, _ []string) {
	log.Println("running soulshack as an irc bot")

	if err := config.Verify(vip.GetViper()); err != nil {
		log.Fatal(err)
	}

	go irc.Irc()
	select {}
}

func runDiscord(_ *cobra.Command, _ []string) {
	log.Println("running soulshack as a discord bot")

	if err := config.Verify(vip.GetViper()); err != nil {
		log.Fatal(err)
	}

	go discord.Discord()
	select {}
}

func GetBanner() string {
	return fmt.Sprintf("%s\n%s",
		figure.NewColorFigure("SoulShack", "", "green", true).ColorString(),
		figure.NewColorFigure(" . . . because real people are overrated", "term", "green", true).ColorString())
}
