package main

// ./chatbot -irchost <IRC_SERVER_ADDRESS> -ircport <IRC_SERVER_PORT> -ircnick <BOT_NICKNAME> -ircchannels '#<CHANNEL>'
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
	USESSL      = flag.Bool("ssl", false, "Enable SSL for the IRC connection")
	PREAMBLE    = flag.String("preamble", "provide a short reply of no more than 3 lines:", "Prepended to prompt")
	MODEL       = flag.String("model", openai.GPT4, "Model to be used for responses (e.g., gpt-4")
	OPENAIKEY   = os.Getenv("OPENAI_API_KEY")
)

// parses the command line arguments, looks up the IP of the server, and sets up the girc client configuration.
func main() {
	flag.Parse()
	if OPENAIKEY == "" {
		log.Fatal("OPENAI_API_KEY environment variable not set")
	}
	host, _ := net.LookupIP(*IRCHOST)
	log.Println(*IRCHOST, host, *IRCPORT)
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

	// The girc.CONNECTED event is used to join the specified channels when the bot connects to the server.
	client.Handlers.Add(girc.CONNECTED, func(c *girc.Client, _ girc.Event) {
		channels := strings.Fields(*IRCCHANNELS)
		log.Println("connecting to channels:", channels)
		for _, channel := range channels {
			c.Cmd.Join(channel)
		}
	})

	// A background event handler is added for girc.PRIVMSG to handle incoming messages.
	client.Handlers.AddBg(girc.PRIVMSG, func(c *girc.Client, e girc.Event) {
		// If the message starts with the bot's nickname, the message is parsed and the appropriate action is taken.
		if strings.HasPrefix(e.Last(), *IRCNICK) {
			tokens := strings.Fields(e.Last())[1:]
			if len(tokens) == 0 {
				return
			}
			switch tokens[0] {
			// If the message contains a /set command, the preamble is updated.
			case "/set":
				if len(tokens) < 3 {
					c.Cmd.Reply(e, "Usage: /set preamble,model <value>")
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
			default:
				// Otherwise, the getReply function is called to generate a response using OpenAI's GPT model
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
			log.Println(err)
			log.Println("reconnecting in 5 seconds...")
			time.Sleep(5 * time.Second)
		} else {
			return
		}
	}
}

// takes a preamble and prompt, creates an OpenAI API client, and sends a chat completion request
func getReply(preamble string, prompt string) (*string, error) {
	client := openai.NewClient(OPENAIKEY)
	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			MaxTokens: 64,
			Model:     openai.GPT4,
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
