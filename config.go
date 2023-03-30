package main

import (
	"fmt"
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
	root.PersistentFlags().BoolP("ssl", "e", false, "Enable SSL for the IRC connection")
	root.PersistentFlags().Int("maxtokens", 512, "Maximum number of tokens to generate with the OpenAI model")
	root.PersistentFlags().IntP("port", "p", 6667, "IRC server port")
	root.PersistentFlags().String("goodbye", "goodbye.", "Response to channel on part")
	root.PersistentFlags().String("greeting", "hello.", "Response to the channel on join")
	root.PersistentFlags().String("model", ai.GPT4, "Model to be used for responses (e.g., gpt-4")
	root.PersistentFlags().String("openaikey", "", "OpenAI API key")
	root.PersistentFlags().String("prompt", "", "Initial character prompt for the AI")
	root.PersistentFlags().StringP("become", "b", "chatbot", "become the named personality")
	root.PersistentFlags().StringP("channel", "c", "", "irc channel to join")
	root.PersistentFlags().StringP("nick", "n", "", "Bot's nickname on the IRC server")
	root.PersistentFlags().StringP("server", "s", "localhost", "IRC server address")
	root.PersistentFlags().StringP("answer", "a", "", "prompt for answering a question")
	root.PersistentFlags().StringSliceP("admins", "A", []string{}, "Comma-separated list of allowed users to administrate the bot (e.g., user1,user2,user3)")
	root.PersistentFlags().DurationP("session", "S", time.Minute*3, "dureation for the chat session; message context will be cleared after this time")
	root.PersistentFlags().DurationP("timeout", "t", time.Second*15, "timeout for each completion request to openai")
	root.PersistentFlags().BoolP("list", "l", false, "list configured personalities")
	root.PersistentFlags().StringP("directory", "d", "./personalities", "personalities configuration directory")

	vip.BindPFlag("become", root.PersistentFlags().Lookup("become"))
	vip.BindPFlag("channel", root.PersistentFlags().Lookup("channel"))
	vip.BindPFlag("goodbye", root.PersistentFlags().Lookup("goodbye"))
	vip.BindPFlag("greeting", root.PersistentFlags().Lookup("greeting"))
	vip.BindPFlag("maxtokens", root.PersistentFlags().Lookup("maxtokens"))
	vip.BindPFlag("model", root.PersistentFlags().Lookup("model"))
	vip.BindPFlag("nick", root.PersistentFlags().Lookup("nick"))
	vip.BindPFlag("openaikey", root.PersistentFlags().Lookup("openaikey"))
	vip.BindPFlag("port", root.PersistentFlags().Lookup("port"))
	vip.BindPFlag("prompt", root.PersistentFlags().Lookup("prompt"))
	vip.BindPFlag("server", root.PersistentFlags().Lookup("server"))
	vip.BindPFlag("ssl", root.PersistentFlags().Lookup("ssl"))
	vip.BindPFlag("answer", root.PersistentFlags().Lookup("answer"))
	vip.BindPFlag("admins", root.PersistentFlags().Lookup("admins"))
	vip.BindPFlag("session", root.PersistentFlags().Lookup("session"))
	vip.BindPFlag("timeout", root.PersistentFlags().Lookup("timeout"))
	vip.BindPFlag("list", root.PersistentFlags().Lookup("list"))
	vip.BindPFlag("directory", root.PersistentFlags().Lookup("directory"))

	vip.SetEnvPrefix("SOULSHACK")
	vip.AutomaticEnv()
}

func initConfig() {

	fmt.Println(getBanner())
	log.Printf("configuration directory %s", vip.GetString("directory"))

	if vip.GetBool("list") {
		personalities := listPersonalities()
		log.Printf("Available personalities: %s", strings.Join(personalities, ", "))
		os.Exit(0)
	}

	log.Println("initializing personality", vip.GetString("become"))

	vip.AddConfigPath(vip.GetString("directory"))
	vip.SetConfigName(vip.GetString("become"))

	if err := vip.ReadInConfig(); err != nil {
		log.Fatalln("! no personality found:", vip.GetString("become"))
	}
	log.Println("using personality file:", vip.ConfigFileUsed())

}
func verifyConfig() error {
	log.Print("verifying configuration...", vip.AllKeys())
	for _, varName := range vip.AllKeys() {
		if varName == "answer" || varName == "admins" {
			continue
		}
		value := vip.GetString(varName)
		if value == "" {
			return fmt.Errorf("! %s unset. use --%s flag, personality config, or SOULSHACK_%s env", varName, varName, strings.ToUpper(varName))
		}
		if varName == "openaikey" {
			value = strings.Repeat("*", len(value))
		}

		log.Printf("\t%s: '%s'", varName, value)
	}
	log.Println("configuration ok")
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

func loadPersonality(p string) error {
	log.Println("loading personality", p)
	personalityViper := vip.New()
	personalityViper.SetConfigFile(vip.GetString("directory") + "/" + p + ".yml")

	err := personalityViper.ReadInConfig()
	if err != nil {
		log.Println("Error reading personality config:", err)
		return err
	}

	originalSettings := vip.AllSettings()
	vip.MergeConfigMap(personalityViper.AllSettings())
	vip.Set("become", p)

	if err := verifyConfig(); err != nil {
		log.Println("Error verifying personality config:", err)
		vip.MergeConfigMap(originalSettings)
		return err
	}

	log.Println("personality loaded:", vip.GetString("become"))
	return nil
}
