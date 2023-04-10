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

	cobra.OnInitialize(initConfig)
	root.PersistentFlags().BoolP("ssl", "e", false, "enable SSL for the IRC connection")
	root.PersistentFlags().Int("maxtokens", 512, "maximum number of tokens to generate")
	root.PersistentFlags().IntP("port", "p", 6667, "irc server port")
	root.PersistentFlags().String("goodbye", "goodbye.", "prompt to be used when the bot leaves the channel")
	root.PersistentFlags().String("greeting", "hello.", "prompt to be used when the bot joins the channel")
	root.PersistentFlags().String("model", ai.GPT4, "model to be used for responses (e.g., gpt-4")
	root.PersistentFlags().String("openaikey", "", "openai api key")
	root.PersistentFlags().String("prompt", "", "initial system prompt for the ai")
	root.PersistentFlags().StringP("become", "b", "chatbot", "become the named personality")
	root.PersistentFlags().StringP("channel", "c", "", "irc channel to join")
	root.PersistentFlags().StringP("nick", "n", "", "bot's nickname on the irc server")
	root.PersistentFlags().StringP("server", "s", "localhost", "irc server address")
	root.PersistentFlags().StringSliceP("admins", "A", []string{}, "comma-separated list of allowed users to administrate the bot (e.g., user1,user2,user3)")
	root.PersistentFlags().DurationP("session", "S", time.Minute*3, "duration for the chat session; message context will be cleared after this time")
	root.PersistentFlags().IntP("history", "H", 15, "maximum number of lines of context to keep per session")
	root.PersistentFlags().DurationP("timeout", "t", time.Second*30, "timeout for each completion request to openai")
	root.PersistentFlags().BoolP("list", "l", false, "list configured personalities")
	root.PersistentFlags().StringP("directory", "d", "./personalities", "personalities configuration directory")
	root.PersistentFlags().BoolP("verbose", "v", false, "enable verbose logging of sessions and configuration")

	vip.BindPFlags(root.PersistentFlags())

	vip.SetEnvPrefix("SOULSHACK")
	vip.AutomaticEnv()
}

func initConfig() {

	fmt.Println(getBanner())

	if _, err := os.Stat(vip.GetString("directory")); errors.Is(err, fs.ErrNotExist) {
		log.Printf("! configuration directory %s does not exist", vip.GetString("directory"))
	}

	if vip.GetBool("list") {
		personalities := listPersonalities()
		log.Printf("Available personalities: %s", strings.Join(personalities, ", "))
		os.Exit(0)
	}

	vip.AddConfigPath(vip.GetString("directory"))
	vip.SetConfigName(vip.GetString("become"))

	if err := vip.ReadInConfig(); err != nil {
		log.Println(err)
		log.Fatalln("! no personality found:", vip.GetString("become"))
	}
	log.Println("using personality file:", vip.ConfigFileUsed())
}

func verifyConfig(v *vip.Viper) error {
	for _, varName := range v.AllKeys() {
		if varName == "admins" {
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

func listPersonalities() []string {
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

func loadPersonality(p string) (*vip.Viper, error) {
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
