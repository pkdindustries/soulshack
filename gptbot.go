package main

import (
	"context"
	"crypto/tls"
	"log"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/lrstanley/girc"
	openai "github.com/sashabaranov/go-openai"
)

var openaiClient *openai.Client

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

var rootCmd = &cobra.Command{
	Use:   "chatbot",
	Short: "A chatbot that connects to IRC and uses OpenAI to generate responses",
	Run:   run,
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().String("host", "localhost", "IRC server address")
	rootCmd.PersistentFlags().Int("port", 6667, "IRC server port")
	rootCmd.PersistentFlags().String("nick", "chatbot", "Bot's nickname on the IRC server")
	rootCmd.PersistentFlags().String("channels", "#chatbot", "Space-separated list of channels to join")
	rootCmd.PersistentFlags().Bool("ssl", false, "Enable SSL for the IRC connection")
	rootCmd.PersistentFlags().String("preamble", "provide a short reply of no more than 3 lines:", "Prepended to prompt")
	rootCmd.PersistentFlags().String("model", openai.GPT4, "Model to be used for responses (e.g., gpt-4")
	rootCmd.PersistentFlags().Int("maxtokens", 64, "Maximum number of tokens to generate with the OpenAI model")
	rootCmd.PersistentFlags().String("openaikey", "", "OpenAI API key")
	rootCmd.PersistentFlags().String("configfile", "config.yaml", "config file (default is ./config.yaml)")

	viper.BindPFlag("host", rootCmd.PersistentFlags().Lookup("host"))
	viper.BindPFlag("port", rootCmd.PersistentFlags().Lookup("port"))
	viper.BindPFlag("nick", rootCmd.PersistentFlags().Lookup("nick"))
	viper.BindPFlag("channels", rootCmd.PersistentFlags().Lookup("channels"))
	viper.BindPFlag("ssl", rootCmd.PersistentFlags().Lookup("ssl"))
	viper.BindPFlag("preamble", rootCmd.PersistentFlags().Lookup("preamble"))
	viper.BindPFlag("model", rootCmd.PersistentFlags().Lookup("model"))
	viper.BindPFlag("maxtokens", rootCmd.PersistentFlags().Lookup("maxtokens"))
	viper.BindPFlag("openaikey", rootCmd.PersistentFlags().Lookup("openaikey"))
	viper.BindPFlag("configfile", rootCmd.PersistentFlags().Lookup("configfile"))
	viper.BindEnv("openaikey", "OPENAI_API_KEY")
	viper.SetEnvPrefix("CHATBOT")
	viper.AutomaticEnv()
}

func initConfig() {
	viper.SetConfigName(viper.GetString("configfile"))
	viper.AddConfigPath(".")
	if err := viper.ReadInConfig(); err == nil {
		log.Println("using config file:", viper.ConfigFileUsed())
	}
}

func run(_ *cobra.Command, _ []string) {

	if viper.GetString("openaikey") == "" {
		log.Fatal("OpenAI API key not set via --openaikey or CHATBOT_OPENAI_API_KEY environment variable")
	}

	openaiClient = openai.NewClient(viper.GetString("openaikey"))
	client := girc.New(girc.Config{
		Server:    viper.GetString("host"),
		Port:      viper.GetInt("port"),
		Nick:      viper.GetString("nick"),
		User:      viper.GetString("nick"),
		Name:      viper.GetString("nick"),
		SSL:       viper.GetBool("ssl"),
		TLSConfig: &tls.Config{InsecureSkipVerify: true},
	})

	client.Handlers.Add(girc.CONNECTED, func(c *girc.Client, _ girc.Event) {
		channels := strings.Fields(viper.GetString("channels"))
		log.Println("connecting to channels:", channels)
		for _, channel := range channels {
			c.Cmd.Join(channel)
		}
	})

	client.Handlers.AddBg(girc.PRIVMSG, func(c *girc.Client, e girc.Event) {
		if strings.HasPrefix(e.Last(), viper.GetString("nick")) {
			handleMessage(c, e)
		}
	})

	for {
		if err := client.Connect(); err != nil {
			log.Println(err)
			log.Println("reconnecting in 5 seconds...")
			time.Sleep(5 * time.Second)
		} else {
			return
		}
	}
}

func handleMessage(c *girc.Client, e girc.Event) {
	tokens := strings.Fields(e.Last())[1:]
	if len(tokens) == 0 {
		return
	}
	switch tokens[0] {
	case "/set":
		handleSet(c, e, tokens)
	default:
		handleDefault(c, e, tokens)
	}
}

func handleSet(c *girc.Client, e girc.Event, tokens []string) {
	if len(tokens) < 3 {
		c.Cmd.Reply(e, "Usage: /set preamble <value>")
		c.Cmd.Reply(e, "Usage: /set model <value>")
		return
	}
	switch tokens[1] {
	case "preamble":
		viper.Set("preamble", strings.Join(tokens[2:], " "))
		c.Cmd.Reply(e, "preamble set to: "+viper.GetString("preamble"))
	case "model":
		viper.Set("model", tokens[2])
		c.Cmd.Reply(e, "model set to: "+viper.GetString("model"))
	default:
		c.Cmd.Reply(e, "Unknown parameter. Supported parameters: preamble, model")
	}
}

func handleDefault(c *girc.Client, e girc.Event, tokens []string) {
	if reply, err := getChatCompletion(viper.GetString("preamble"), strings.Join(tokens, " "), viper.GetString("model"), viper.GetInt("maxtokens")); err != nil {
		c.Cmd.Reply(e, err.Error())
	} else {
		c.Cmd.Reply(e, *reply)
	}
}

func getChatCompletion(preamble string, prompt string, model string, maxtokens int) (*string, error) {
	resp, err := openaiClient.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			MaxTokens: maxtokens,
			Model:     model,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: preamble + prompt,
				},
			},
		},
	)

	if err != nil {
		return nil, err
	}

	return &resp.Choices[0].Message.Content, nil
}
