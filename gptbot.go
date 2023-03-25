package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"os"
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
	rootCmd.PersistentFlags().String("preamble", "provide a short reply of no more than 3 lines...", "Prepended to prompt, use to customize the bot")
	rootCmd.PersistentFlags().String("model", openai.GPT4, "Model to be used for responses (e.g., gpt-4")
	rootCmd.PersistentFlags().Int("maxtokens", 64, "Maximum number of tokens to generate with the OpenAI model")
	rootCmd.PersistentFlags().String("greeting", "greet the group chat and introduce yourself as a GPT-4 based irc chatbot", "Response to the channel on join")
	rootCmd.PersistentFlags().String("goodbye", "say goodbye to the group chat and sign off as a GPT-4 based irc chatbot", "Response to channel on part")

	rootCmd.PersistentFlags().String("openaikey", "", "OpenAI API key")
	rootCmd.PersistentFlags().String("config", "", "path to configuration file")

	viper.BindPFlag("host", rootCmd.PersistentFlags().Lookup("host"))
	viper.BindPFlag("port", rootCmd.PersistentFlags().Lookup("port"))
	viper.BindPFlag("nick", rootCmd.PersistentFlags().Lookup("nick"))
	viper.BindPFlag("channels", rootCmd.PersistentFlags().Lookup("channels"))
	viper.BindPFlag("ssl", rootCmd.PersistentFlags().Lookup("ssl"))
	viper.BindPFlag("preamble", rootCmd.PersistentFlags().Lookup("preamble"))
	viper.BindPFlag("model", rootCmd.PersistentFlags().Lookup("model"))
	viper.BindPFlag("maxtokens", rootCmd.PersistentFlags().Lookup("maxtokens"))
	viper.BindPFlag("openaikey", rootCmd.PersistentFlags().Lookup("openaikey"))
	viper.BindPFlag("config", rootCmd.PersistentFlags().Lookup("config"))
	viper.BindPFlag("greeting", rootCmd.PersistentFlags().Lookup("greeting"))
	viper.BindPFlag("goodbye", rootCmd.PersistentFlags().Lookup("goodbye"))

	viper.SetEnvPrefix("CHATBOT")
	viper.BindEnv("openaikey", "OPENAI_API_KEY")
	viper.AutomaticEnv()
}

func initConfig() {
	viper.SetConfigFile(viper.GetString("config"))
	viper.AddConfigPath(".")

	if err := viper.ReadInConfig(); err == nil {
		log.Println("using config file:", viper.ConfigFileUsed())
	}
}

func run(_ *cobra.Command, _ []string) {

	log.Println("Starting chatbot...")
	if viper.GetString("openaikey") == "" {
		log.Fatal("openai api key unset. use --openaikey flag, config, or CHATBOT_OPENAI_API_KEY env.")
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
		log.Println("joining channels:", channels)
		c.Cmd.Join(channels...)
		sendGreeting(c, channels)
	})

	client.Handlers.AddBg(girc.PRIVMSG, func(c *girc.Client, e girc.Event) {
		if strings.HasPrefix(e.Last(), viper.GetString("nick")) {
			handleMessage(c, e)
		}
	})

	for {
		log.Println("connecting to irc server", viper.GetString("host"), "on port", viper.GetInt("port"))

		if err := client.Connect(); err != nil {
			log.Println(err)
			log.Println("reconnecting in 5 seconds...")
			time.Sleep(5 * time.Second)
		} else {
			return
		}
	}
}

func sendGoodbye(c *girc.Client, channels []string) {
	go func() {
		time.Sleep(1 * time.Second)
		if msg, err := getChatCompletion(viper.GetString("goodbye")); err != nil {
			c.Cmd.Message(channels[0], err.Error())
		} else {
			c.Cmd.Message(channels[0], *msg)
		}
	}()
}

func sendGreeting(c *girc.Client, channels []string) {
	go func() {
		time.Sleep(1 * time.Second)
		if msg, err := getChatCompletion(viper.GetString("greeting")); err != nil {
			c.Cmd.Message(channels[0], err.Error())
		} else {
			c.Cmd.Message(channels[0], *msg)
		}
	}()
}

var configParams = map[string]string{
	"preamble": "Usage: /set preamble <value>",
	"model":    "Usage: /set model <value>",
	"nick":     "Usage: /set nick <value>",
	"greeting": "Usage: /set greeting <value>",
	"goodbye":  "Usage: /set goodbye <value>",
}

func handleMessage(c *girc.Client, e girc.Event) {
	tokens := strings.Fields(e.Last())[1:]
	if len(tokens) == 0 {
		return
	}
	switch tokens[0] {
	case "/set":
		handleSet(c, e, tokens)
	case "/get":
		handleGet(c, e, tokens)
	case "/save":
		handleSave(c, e, tokens)
	case "/load":
		handleLoad(c, e, tokens)
	case "/leave":
		handleLeave(c, e)
	case "/help":
		fallthrough
	case "/?":
		c.Cmd.Reply(e, "Supported commands: /set, /get, /save, /load, /leave, /help")
	default:
		handleDefault(c, e, tokens)
	}
}

func handleSet(c *girc.Client, e girc.Event, tokens []string) {
	if len(tokens) < 3 {
		for _, desc := range configParams {
			c.Cmd.Reply(e, desc)
		}
		return
	}

	param, v := tokens[1], tokens[2:]
	value := strings.Join(v, " ")
	if _, ok := configParams[param]; !ok {
		c.Cmd.Reply(e, fmt.Sprintf("Unknown parameter. Supported parameters: %v", keysAsString(configParams)))
		return
	}

	viper.Set(param, value)
	c.Cmd.Reply(e, fmt.Sprintf("%s set to: %s", param, viper.GetString(param)))

	if param == "nick" {
		c.Cmd.Nick(value)
	}
}

func handleGet(c *girc.Client, e girc.Event, tokens []string) {
	if len(tokens) < 2 {
		for _, desc := range configParams {
			c.Cmd.Reply(e, desc)
		}
		return
	}

	param := tokens[1]
	if _, ok := configParams[param]; !ok {
		c.Cmd.Reply(e, fmt.Sprintf("Unknown parameter. Supported parameters: %v", keysAsString(configParams)))
		return
	}

	value := viper.GetString(param)
	c.Cmd.Reply(e, fmt.Sprintf("%s: %s", param, value))
}

func handleSave(c *girc.Client, e girc.Event, tokens []string) {
	if len(tokens) < 2 {
		c.Cmd.Reply(e, "Usage: /save <filename>")
		return
	}

	filename := tokens[1]

	if err := viper.WriteConfigAs(filename); err != nil {
		c.Cmd.Reply(e, fmt.Sprintf("Error saving configuration: %s", err.Error()))
		return
	}

	c.Cmd.Reply(e, fmt.Sprintf("Configuration saved to: %s", filename))
}

func handleLoad(c *girc.Client, e girc.Event, tokens []string) {
	if len(tokens) < 2 {
		c.Cmd.Reply(e, "Usage: /load <filename>")
		return
	}

	file := tokens[1]

	viper.SetConfigFile(file)
	if err := viper.ReadInConfig(); err != nil {
		c.Cmd.Reply(e, fmt.Sprintf("Error loading configuration: %s", err.Error()))
		return
	}

	c.Cmd.Reply(e, fmt.Sprintf("loaded %s, %s, name: %s", file, viper.GetString("model"), viper.GetString("nick")))

	c.Cmd.Nick(viper.GetString("nick"))
	sendGreeting(c, e.Params)
}

func handleLeave(c *girc.Client, e girc.Event) {
	sendGoodbye(c, e.Params)
	log.Println("exiting...")
	go func() {
		time.Sleep(1 * time.Second)
		os.Exit(0)
	}()
}

func keysAsString(m map[string]string) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return strings.Join(keys, ", ")
}

func handleDefault(c *girc.Client, e girc.Event, tokens []string) {
	if reply, err := getChatCompletion(strings.Join(tokens, " ")); err != nil {
		c.Cmd.Reply(e, err.Error())
	} else {
		c.Cmd.Reply(e, *reply)
	}
}

func getChatCompletion(query string) (*string, error) {

	prompt := viper.GetString("preamble") + query
	log.Println("prompt: ", prompt)

	resp, err := openaiClient.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			MaxTokens: viper.GetInt("maxtokens"),
			Model:     viper.GetString("model"),
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: prompt,
				},
			},
		},
	)

	if err != nil {
		return nil, err
	}

	log.Println("response: ", resp.Choices[0].Message.Content)
	return &resp.Choices[0].Message.Content, nil
}
