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
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/common-nighthawk/go-figure"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/lrstanley/girc"
	ai "github.com/sashabaranov/go-openai"
)

var aiClient *ai.Client

// user <-> bot transcript in memory
var chatHistory []ai.ChatCompletionMessage
var lastMessageTime = time.Now()

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
	Version: "0.42 . . . because real people are overrated . . . http://github.com/pkdindustries/soulshack",
}

func init() {

	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().BoolP("ssl", "e", false, "Enable SSL for the IRC connection")
	rootCmd.PersistentFlags().Int("maxtokens", 512, "Maximum number of tokens to generate with the OpenAI model")
	rootCmd.PersistentFlags().IntP("port", "p", 6667, "IRC server port")
	rootCmd.PersistentFlags().String("goodbye", "goodbye.", "Response to channel on part")
	rootCmd.PersistentFlags().String("greeting", "hello.", "Response to the channel on join")
	rootCmd.PersistentFlags().String("model", ai.GPT4, "Model to be used for responses (e.g., gpt-4")
	rootCmd.PersistentFlags().String("openaikey", "", "OpenAI API key")
	rootCmd.PersistentFlags().String("prompt", "", "Initial character prompt for the AI")
	rootCmd.PersistentFlags().StringP("become", "b", "chatbot", "become the named personality")
	rootCmd.PersistentFlags().StringP("channel", "c", "", "irc channel to join")
	rootCmd.PersistentFlags().StringP("nick", "n", "", "Bot's nickname on the IRC server")
	rootCmd.PersistentFlags().StringP("server", "s", "localhost", "IRC server address")
	rootCmd.PersistentFlags().StringP("answer", "a", "", "prompt for answering a question")
	rootCmd.PersistentFlags().StringSliceP("admins", "A", []string{}, "Comma-separated list of allowed users to administrate the bot (e.g., user1,user2,user3)")
	rootCmd.PersistentFlags().DurationP("session", "S", time.Minute*3, "dureation for the chat session; message context will be cleared after this time")
	rootCmd.PersistentFlags().DurationP("timeout", "t", time.Second*15, "timeout for each completion request to openai")
	rootCmd.PersistentFlags().BoolP("list", "l", false, "list configured personalities")
	rootCmd.PersistentFlags().StringP("directory", "d", "./personalities", "personalities configuration directory")

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
	viper.BindPFlag("admins", rootCmd.PersistentFlags().Lookup("admins"))
	viper.BindPFlag("session", rootCmd.PersistentFlags().Lookup("session"))
	viper.BindPFlag("timeout", rootCmd.PersistentFlags().Lookup("timeout"))
	viper.BindPFlag("list", rootCmd.PersistentFlags().Lookup("list"))
	viper.BindPFlag("directory", rootCmd.PersistentFlags().Lookup("directory"))

	viper.SetEnvPrefix("SOULSHACK")
	viper.AutomaticEnv()
}

func initConfig() {

	fmt.Println(getBanner())
	log.Printf("configuration directory %s", viper.GetString("directory"))

	if viper.GetBool("list") {
		personalities := listPersonalities()
		log.Printf("Available personalities: %s", strings.Join(personalities, ", "))
		os.Exit(0)
	}

	log.Println("initializing personality", viper.GetString("become"))

	viper.AddConfigPath(viper.GetString("directory"))
	viper.SetConfigName(viper.GetString("become"))

	if err := viper.ReadInConfig(); err != nil {
		log.Fatalln("! no personality found:", viper.GetString("become"))
	}
	log.Println("using personality file:", viper.ConfigFileUsed())

}
func verifyConfig() error {
	log.Print("verifying configuration...", viper.AllKeys())
	for _, varName := range viper.AllKeys() {
		if varName == "answer" || varName == "admins" {
			continue
		}
		value := viper.GetString(varName)
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

func run(_ *cobra.Command, _ []string) {

	if err := verifyConfig(); err != nil {
		log.Fatal(err)
	}

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
		sendGreeting(c, &e)
	})

	irc.Handlers.Add(girc.PRIVMSG, func(c *girc.Client, e girc.Event) {
		if time.Since(lastMessageTime) >= viper.GetDuration("session") {
			resetChatHistory()
		}

		privmsg := !strings.HasPrefix(e.Params[0], "#")
		addressed := strings.HasPrefix(e.Last(), c.GetNick())
		if privmsg || addressed {

			log.Println("<", e)

			tokens := strings.Fields(e.Last())
			if addressed {
				tokens = strings.Fields(e.Last())[1:]
			}

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
			case "/list":
				handleList(c, e)
			case "/become":
				handleBecome(c, e, tokens)
			case "/leave":
				handleLeave(c, e)
			case "/help":
				fallthrough
			case "/?":
				c.Cmd.Reply(e, "Supported commands: /set, /get, /list, /become, /leave, /help, /version")
			case "/version":
				//handleVersion(c, e, rootCmd.Version)
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

func resetChatHistory() {
	if len(chatHistory) > 1 {
		log.Println("chat history reset...")
	}
	chatHistory = []ai.ChatCompletionMessage{}
	chatHistory = append(chatHistory, ai.ChatCompletionMessage{
		Role:    ai.ChatMessageRoleAssistant,
		Content: viper.GetString("prompt"),
	})
}

func sendGreeting(c *girc.Client, e *girc.Event) {

	resetChatHistory()

	log.Println("sending greeting...")
	chatHistory = append(chatHistory, ai.ChatCompletionMessage{
		Role:    ai.ChatMessageRoleAssistant,
		Content: viper.GetString("greeting"),
	})

	reply := getChatCompletionString(chatHistory)

	if e.Source == nil {
		e.Source = &girc.Source{
			Name: viper.GetString("channel"),
		}
	}
	sendMessage(c, e, reply)

	chatHistory = append(chatHistory, ai.ChatCompletionMessage{
		Role:    ai.ChatMessageRoleAssistant,
		Content: reply,
	})
}

func sendMessage(c *girc.Client, e *girc.Event, message string) {
	log.Println(">", e, message)
	lastMessageTime = time.Now()
	sendMessageChunks(c, e, &message)
}

func sendMessageChunks(c *girc.Client, e *girc.Event, message *string) {
	chunks := splitResponse(*message, 400)
	for _, msg := range chunks {
		time.Sleep(500 * time.Millisecond)
		c.Cmd.ReplyTo(*e, msg)
	}
}

var configParams = map[string]string{"prompt": "", "model": "", "nick": "", "greeting": "", "goodbye": "", "answer": "", "directory": "", "session": ""}

func handleSet(c *girc.Client, e girc.Event, tokens []string) {

	if !isAdmin(e.Source.Name) {
		c.Cmd.Reply(e, "You don't have permission to perform this action.")
		return
	}

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

	resetChatHistory()
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

	if !isAdmin(e.Source.Name) {
		c.Cmd.Reply(e, "You don't have permission to perform this action.")
		return
	}

	if len(tokens) < 2 {
		c.Cmd.Reply(e, "Usage: /save <name>")
		return
	}

	filename := tokens[1]

	v := viper.New()

	v.Set("nick", viper.GetString("nick"))
	v.Set("prompt", viper.GetString("prompt"))
	v.Set("model", viper.GetString("model"))
	v.Set("maxtokens", viper.GetInt("maxtokens"))
	v.Set("greeting", viper.GetString("greeting"))
	v.Set("goodbye", viper.GetString("goodbye"))
	v.Set("answer", viper.GetString("answer"))

	if err := v.WriteConfigAs(viper.GetString("directory") + "/" + filename + ".yml"); err != nil {
		c.Cmd.Reply(e, fmt.Sprintf("Error saving configuration: %s", err.Error()))
		return
	}

	c.Cmd.Reply(e, fmt.Sprintf("Configuration saved to: %s", filename))
}

func loadPersonality(p string) error {
	log.Println("loading personality", p)
	personalityViper := viper.New()
	personalityViper.SetConfigFile(viper.GetString("directory") + "/" + p + ".yml")

	err := personalityViper.ReadInConfig()
	if err != nil {
		log.Println("Error reading personality config:", err)
		return err
	}

	originalSettings := viper.AllSettings()
	viper.MergeConfigMap(personalityViper.AllSettings())
	viper.Set("become", p)

	if err := verifyConfig(); err != nil {
		log.Println("Error verifying personality config:", err)
		viper.MergeConfigMap(originalSettings)
		return err
	}

	log.Println("personality loaded:", viper.GetString("become"))
	return nil
}

func handleBecome(c *girc.Client, e girc.Event, tokens []string) {

	if !isAdmin(e.Source.Name) {
		c.Cmd.Reply(e, "You don't have permission to perform this action.")
		return
	}

	if len(tokens) < 2 {
		c.Cmd.Reply(e, "Usage: /become <personality>")
		return
	}

	personality := tokens[1]
	if err := loadPersonality(personality); err != nil {
		log.Println("Error loading personality:", err)
		c.Cmd.Reply(e, fmt.Sprintf("Error loading personality: %s", err.Error()))
		return
	}

	log.Printf("changing nick to %s as personality %s", viper.GetString("nick"), personality)

	c.Cmd.Nick(viper.GetString("nick"))
	time.Sleep(2 * time.Second)
	sendGreeting(c, &e)
}

func handleList(c *girc.Client, e girc.Event) {
	personalities := listPersonalities()
	c.Cmd.Reply(e, fmt.Sprintf("Available personalities: %s", strings.Join(personalities, ", ")))
}

func handleLeave(c *girc.Client, e girc.Event) {
	if !isAdmin(e.Source.Name) {
		c.Cmd.Reply(e, "You don't have permission to perform this action.")
		return
	}

	sendMessage(c, &e, getChatCompletionString(
		[]ai.ChatCompletionMessage{
			{
				Role:    ai.ChatMessageRoleAssistant,
				Content: viper.GetString("prompt") + viper.GetString("goodbye"),
			},
		},
	))

	log.Println("exiting...")
	go func() {
		time.Sleep(1 * time.Second)
		os.Exit(0)
	}()
}

func handleDefault(c *girc.Client, e girc.Event, tokens []string) {
	msg := strings.Join(tokens, " ")

	chatHistory = append(chatHistory, ai.ChatCompletionMessage{
		Role:    ai.ChatMessageRoleUser,
		Content: msg,
		Name:    e.Source.Name,
	})

	if reply, err := getChatCompletion(chatHistory); err != nil {
		c.Cmd.Reply(e, err.Error())
		// never happened
		if len(chatHistory) > 1 {
			chatHistory = chatHistory[:len(chatHistory)-1]
		}
	} else {
		chatHistory = append(chatHistory, ai.ChatCompletionMessage{
			Role:    ai.ChatMessageRoleAssistant,
			Content: *reply,
		})
		sendMessage(c, &e, *reply)
	}
}

func getChatCompletionString(messages []ai.ChatCompletionMessage) string {
	if reply, err := getChatCompletion(messages); err != nil {
		return err.Error()
	} else {
		if reply != nil {
			return *reply
		} else {
			log.Println("getchatcompletionstring: ", err)
			return err.Error()
		}
	}
}

func getChatCompletion(msgs []ai.ChatCompletionMessage) (*string, error) {

	log.Printf("completing: messages %d, characters %d, maxtokens %d, model %s",
		len(msgs),
		sumMessageLengths(msgs),
		viper.GetInt("maxtokens"),
		viper.GetString("model"),
	)

	now := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), viper.GetDuration("timeout"))
	defer cancel()

	resp, err := aiClient.CreateChatCompletion(
		ctx,
		ai.ChatCompletionRequest{
			MaxTokens: viper.GetInt("maxtokens"),
			Model:     viper.GetString("model"),
			Messages:  msgs,
		},
	)

	if err != nil {
		return nil, err
	}

	if len(resp.Choices) == 0 {
		return nil, errors.New("no response")
	}

	log.Printf("completed: %s, %d tokens", time.Since(now), resp.Usage.TotalTokens)
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

func isAdmin(nick string) bool {
	admins := viper.GetStringSlice("admins")
	if len(admins) == 0 {
		return true
	}
	for _, user := range admins {
		if user == nick {
			return true
		}
	}
	return false
}

func sumMessageLengths(messages []ai.ChatCompletionMessage) int {
	sum := 0
	for _, m := range messages {
		sum += len(m.Content)
	}
	return sum
}

func keysAsString(m map[string]string) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return strings.Join(keys, ", ")
}

func listPersonalities() []string {
	files, err := os.ReadDir(viper.GetString("directory"))
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
