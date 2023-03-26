package main

//  ____                    _   ____    _                      _
// / ___|    ___    _   _  | | / ___|  | |__     __ _    ___  | | __
// \___ \   / _ \  | | | | | | \___ \  | '_ \   / _` |  / __| | |/ /
//  ___) | | (_) | | |_| | | |  ___) | | | | | | (_| | | (__  |   <
// |____/   \___/   \__,_| |_| |____/  |_| |_|  \__,_|  \___| |_|\_\
//  .  .  .  because  real  people  are  overrated

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/common-nighthawk/go-figure"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/lrstanley/girc"
	ai "github.com/sashabaranov/go-openai"
)

var aiClient *ai.Client

func getBanner() string {
	return fmt.Sprintf("%s\n%s",
		figure.NewColorFigure("SoulShack", "", "green", true).ColorString(),
		figure.NewColorFigure(" . . . because real people are overrated", "term", "green", true).ColorString())
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

var rootCmd = &cobra.Command{
	Use:     "soulshack --server irc.freenode.net --port 6697 --channel '#soulshack' --ssl --openaikey <your openai api key> --become <your personality>",
	Example: "soulshack --server irc.freenode.net --port 6697 --channel '#soulshack' --ssl --openaikey ****************",
	Short:   getBanner(),
	Run:     run,
}

func init() {

	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().BoolP("ssl", "e", false, "Enable SSL for the IRC connection")
	rootCmd.PersistentFlags().Int("maxtokens", 512, "Maximum number of tokens to generate with the OpenAI model")
	rootCmd.PersistentFlags().IntP("port", "p", 6667, "IRC server port")
	rootCmd.PersistentFlags().String("goodbye", "", "Response to channel on part")
	rootCmd.PersistentFlags().String("greeting", "", "Response to the channel on join")
	rootCmd.PersistentFlags().String("model", ai.GPT4, "Model to be used for responses (e.g., gpt-4")
	rootCmd.PersistentFlags().String("openaikey", "", "OpenAI API key")
	rootCmd.PersistentFlags().String("prompt", "", "Initial character prompt for the AI")
	rootCmd.PersistentFlags().StringP("become", "b", "chatbot", "become the named personality")
	rootCmd.PersistentFlags().StringP("channel", "c", "", "irc channel to join")
	rootCmd.PersistentFlags().StringP("nick", "n", "", "Bot's nickname on the IRC server")
	rootCmd.PersistentFlags().StringP("server", "s", "localhost", "IRC server address")
	rootCmd.PersistentFlags().StringP("answer", "a", "", "prompt for answering a question")

	viper.BindPFlag("become", rootCmd.PersistentFlags().Lookup("become"))
	viper.BindPFlag("channel", rootCmd.PersistentFlags().Lookup("channel"))
	viper.BindPFlag("goodbye", rootCmd.PersistentFlags().Lookup("goodbye"))
	viper.BindPFlag("greeting", rootCmd.PersistentFlags().Lookup("greeting"))
	viper.BindPFlag("maxtokens", rootCmd.PersistentFlags().Lookup("maxtokens"))
	viper.BindPFlag("model", rootCmd.PersistentFlags().Lookup("model"))
	viper.BindPFlag("nick", rootCmd.PersistentFlags().Lookup("nick"))
	viper.BindPFlag("openaikey", rootCmd.PersistentFlags().Lookup("openaikey"))
	viper.BindPFlag("port", rootCmd.PersistentFlags().Lookup("port"))
	viper.BindPFlag("prompt", rootCmd.PersistentFlags().Lookup("prompt"))
	viper.BindPFlag("server", rootCmd.PersistentFlags().Lookup("server"))
	viper.BindPFlag("ssl", rootCmd.PersistentFlags().Lookup("ssl"))
	viper.BindPFlag("answer", rootCmd.PersistentFlags().Lookup("answer"))
	viper.SetEnvPrefix("SOULSHACK")
	viper.AutomaticEnv()
}

func initConfig() {

	fmt.Println(getBanner())
	log.Println("initializing personality", viper.GetString("become"))

	viper.AddConfigPath("personalities")
	viper.SetConfigName(viper.GetString("become"))

	if err := viper.ReadInConfig(); err != nil {
		log.Fatalln("no personality found:", viper.GetString("become"))
	}
	log.Println("using personality file:", viper.ConfigFileUsed())

}
func verifyConfig() {
	for _, varName := range viper.AllKeys() {
		if varName == "answer" {
			continue
		}
		value := viper.GetString(varName)
		if value == "" {
			log.Fatalf("%s unset. use --%s flag, personality config, or %s env.", varName, varName, strings.ToUpper(varName))
		}
		if varName == "openaikey" {
			value = strings.Repeat("*", len(value))
		}

		log.Printf("%s: %s", varName, value)
	}
}

func run(_ *cobra.Command, _ []string) {

	verifyConfig()

	aiClient = ai.NewClient(viper.GetString("openaikey"))

	irc := girc.New(girc.Config{
		Server:    viper.GetString("server"),
		Port:      viper.GetInt("port"),
		Nick:      viper.GetString("nick"),
		User:      "soulshack",
		Name:      "soulshack",
		SSL:       viper.GetBool("ssl"),
		TLSConfig: &tls.Config{InsecureSkipVerify: true},
	})

	irc.Handlers.Add(girc.CONNECTED, func(c *girc.Client, e girc.Event) {
		channel := viper.GetString("channel")
		log.Println("joining channel:", channel)
		c.Cmd.Join(channel)
		time.Sleep(1 * time.Second)
		sendMessage(c, &e, getChatCompletionString(viper.GetString("prompt")+viper.GetString("greeting")))
	})

	irc.Handlers.Add(girc.PRIVMSG, func(c *girc.Client, e girc.Event) {
		log.Printf("%s: %s", viper.GetString("nick"), e.Last())
		if strings.HasPrefix(e.Last(), viper.GetString("nick")) {
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
			case "/become":
				handleBecome(c, e, tokens)
			case "/leave":
				handleLeave(c)
			case "/help":
				fallthrough
			case "/?":
				c.Cmd.Reply(e, "Supported commands: /set, /get, /save, /load, /leave, /help")
			default:
				handleDefault(c, e, tokens)
			}
		}
	})

	for {
		log.Println("connecting to irc server", viper.GetString("server"), "on port", viper.GetInt("port"), "using ssl:", viper.GetBool("ssl"))

		if err := irc.Connect(); err != nil {
			log.Println(err)
			log.Println("reconnecting in 5 seconds...")
			time.Sleep(5 * time.Second)
		} else {
			return
		}
	}
}

func sendMessage(c *girc.Client, e *girc.Event, message string) {
	log.Println("sendMessage()", c.ChannelList(), e, message)

	target := viper.GetString("channel")

	sendMessageChunks(c, target, &message)
}

func getChannelContext(channel *girc.Channel) string {
	context := "you are in the group chat channel " + channel.Name + " with the following users:"
	for _, u := range channel.UserList {
		if u == viper.GetString("nick") {
			continue
		}
		context += u + ", "
	}
	return context + "..."
}

func sendMessageChunks(c *girc.Client, target string, message *string) {
	chunks := splitResponse(*message, 400)
	for _, msg := range chunks {
		time.Sleep(500 * time.Millisecond)
		c.Cmd.Message(target, msg)
	}
}

var configParams = map[string]string{
	"prompt":   "",
	"model":    "",
	"nick":     "",
	"greeting": "",
	"goodbye":  "",
	"answer":   "",
}

func handleSet(c *girc.Client, e girc.Event, tokens []string) {
	if len(tokens) < 3 {
		c.Cmd.Reply(e, fmt.Sprintf("Usage: /set %s <value>", keysAsString(configParams)))
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
		c.Cmd.Reply(e, fmt.Sprintf("Usage: /get %s", keysAsString(configParams)))
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
		c.Cmd.Reply(e, "Usage: /save <name>")
		return
	}

	filename := tokens[1]

	v := viper.New()
	v.SetConfigName(filename)
	v.SetConfigType("yml")
	v.AddConfigPath("personalities")

	v.Set("nick", viper.GetString("nick"))
	v.Set("prompt", viper.GetString("prompt"))
	v.Set("model", viper.GetString("model"))
	v.Set("maxtokens", viper.GetInt("maxtokens"))
	v.Set("greeting", viper.GetString("greeting"))
	v.Set("goodbye", viper.GetString("goodbye"))
	v.Set("answer", viper.GetString("answer"))

	if err := v.WriteConfigAs("personalities/" + filename + ".yml"); err != nil {
		c.Cmd.Reply(e, fmt.Sprintf("Error saving configuration: %s", err.Error()))
		return
	}

	c.Cmd.Reply(e, fmt.Sprintf("Configuration saved to: %s", filename))
}

func handleBecome(c *girc.Client, e girc.Event, tokens []string) {
	if len(tokens) < 2 {
		c.Cmd.Reply(e, "Usage: /become <any person>")
		return
	}

	fullName := strings.Join(tokens[1:], " ")
	nick := strings.ToLower(strings.ReplaceAll(fullName, " ", "")) + "bot"

	// nick is at most 9 chars
	if len(nick) > 9 {
		nick = nick[:6] + "bot"
	}

	viper.Set("prompt", fmt.Sprintf("compose a short reply of no more than 3 lines in characteristic %s fashion...", fullName))
	viper.Set("greeting", "greeting the group chat")
	viper.Set("goodbye", "leaving the group chat ")

	c.Cmd.Nick(nick)
	viper.Set("nick", nick)

	sendMessage(c, &e, getChatCompletionString(viper.GetString("greeting")))
}

func handleLeave(c *girc.Client) {
	sendMessage(c, nil, getChatCompletionString(viper.GetString("goodbye")))
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
	if reply, err := getChatCompletion(viper.GetString("prompt") + viper.GetString("answer") + strings.Join(tokens, " ")); err != nil {
		c.Cmd.Reply(e, err.Error())
	} else {
		sendMessage(c, &e, *reply)
	}
}

func getChatCompletionString(query string) string {
	if reply, err := getChatCompletion(query); err != nil {
		return err.Error()
	} else {
		return *reply
	}
}
func getChatCompletion(query string) (*string, error) {

	log.Println("getChatCompletion() prompt:", query, "tokens:", viper.GetInt("maxtokens"), "model:", viper.GetString("model"))

	resp, err := aiClient.CreateChatCompletion(
		context.Background(),
		ai.ChatCompletionRequest{
			MaxTokens: viper.GetInt("maxtokens"),
			Model:     viper.GetString("model"),
			Messages: []ai.ChatCompletionMessage{
				{
					Role:    ai.ChatMessageRoleUser,
					Content: query,
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

func splitResponse(response string, maxLineLength int) []string {
	words := strings.Fields(response)
	messages := []string{}
	currentLine := ""

	for _, word := range words {
		if len(currentLine)+len(word)+1 > maxLineLength {
			messages = append(messages, currentLine)
			currentLine = ""
		}
		if len(currentLine) > 0 {
			currentLine += " "
		}
		currentLine += word
	}

	if currentLine != "" {
		messages = append(messages, currentLine)
	}

	return messages
}
