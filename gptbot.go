package main

import (
	"context"
	"crypto/tls"
	"flag"
	"log"
	"os"
	"strings"
	"time"

	"github.com/lrstanley/girc"
	openai "github.com/sashabaranov/go-openai"
)

var (
	IRCHOST     = flag.String("irchost", "localhost", "IRC server address")
	IRCPORT     = flag.Int("ircport", 6667, "IRC server port")
	IRCNICK     = flag.String("ircnick", "chatbot", "Bot's nickname on the IRC server")
	IRCCHANNELS = flag.String("ircchannels", "", "Space-separated list of channels to join")
	USESSL      = flag.Bool("ssl", false, "Enable SSL for the IRC connection")
	PREAMBLE    = flag.String("preamble", "provide a short reply of no more than 3 lines:", "Prepended to prompt")
	MODEL       = flag.String("model", openai.GPT4, "Model to be used for responses (e.g., gpt-4")
	MAXTOKENS   = flag.Int("maxtokens", 64, "Maximum number of tokens to generate with the OpenAI model")
	OPENAIKEY   = os.Getenv("CHATBOT_OPENAI_API_KEY")
)

var openaiClient *openai.Client

func main() {
	flag.Parse()
	if OPENAIKEY == "" {
		log.Fatal("CHATBOT_OPENAI_API_KEY environment variable not set")
	}
	if *IRCCHANNELS == "" {
		log.Fatal("ircchannels flag must be provided")
	}

	openaiClient = openai.NewClient(OPENAIKEY)
	client := girc.New(girc.Config{
		Server:    *IRCHOST,
		Port:      *IRCPORT,
		Nick:      *IRCNICK,
		User:      *IRCNICK,
		Name:      *IRCNICK,
		Debug:     os.Stdout,
		SSL:       *USESSL,
		TLSConfig: &tls.Config{InsecureSkipVerify: true},
	})

	client.Handlers.Add(girc.CONNECTED, func(c *girc.Client, _ girc.Event) {
		channels := strings.Fields(*IRCCHANNELS)
		log.Println("connecting to channels:", channels)
		for _, channel := range channels {
			c.Cmd.Join(channel)
		}
	})

	client.Handlers.AddBg(girc.PRIVMSG, func(c *girc.Client, e girc.Event) {
		if strings.HasPrefix(e.Last(), *IRCNICK) {
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
		p := strings.Join(tokens[2:], " ")
		PREAMBLE = &p
		c.Cmd.Reply(e, "preamble set to: "+*PREAMBLE)
	case "model":
		MODEL = &tokens[2]
		c.Cmd.Reply(e, "model set to: "+*MODEL)
	default:
		c.Cmd.Reply(e, "Unknown parameter. Supported parameters: preamble, model")
	}
}

func handleDefault(c *girc.Client, e girc.Event, tokens []string) {
	if reply, err := getChatCompletion(*PREAMBLE, strings.Join(tokens, " "), *MODEL, *MAXTOKENS); err != nil {
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
