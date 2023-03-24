package main

import (
	"context"
	"crypto/tls"
	"flag"
	"log"
	"net"
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
	OPENAIKEY   = flag.String("openaikey", "", "OpenAI API key")
	USESSL      = flag.Bool("ssl", false, "Enable SSL for the IRC connection")
	PREAMBLE    = flag.String("preamble", "provide a short reply of no more than 3 lines:", "Prepended to prompt")
)

func main() {
	flag.Parse()

	host, _ := net.LookupIP(*IRCHOST)
	log.Println(*IRCHOST, host, *IRCPORT)
	client := girc.New(girc.Config{
		Server:    *IRCHOST,
		Port:      *IRCPORT,
		Nick:      *IRCNICK,
		User:      "irc chatbot by a-lex",
		Name:      "irc chatbot by a-lex",
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
			tokens := strings.Fields(e.Last())[1:]
			if len(tokens) == 0 {
				return
			}
			switch tokens[0] {
			case "/set":
				if len(tokens) != 3 {
					c.Cmd.Reply(e, "Usage: /set preamble <message>")
					return
				}
				if tokens[1] == "preamble" {
					PREAMBLE = &tokens[2]
					c.Cmd.Reply(e, "preamble set to: "+*PREAMBLE)
				}
			default:
				if reply, err := getReply(*PREAMBLE, strings.Join(tokens, " ")); err != nil {
					c.Cmd.Reply(e, err.Error())
				} else {
					c.Cmd.Reply(e, *reply)
				}
			}
		}
	})

	for {
		if err := client.Connect(); err != nil {
			log.Println("reconnecting in 5 seconds...")
			time.Sleep(5 * time.Second)
		} else {
			return
		}
	}
}

func getReply(preamble string, prompt string) (*string, error) {
	client := openai.NewClient(*OPENAIKEY)
	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			MaxTokens: 64,
			Model:     openai.GPT4, // Replace with the appropriate model, as GPT-4 is hypothetical
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
