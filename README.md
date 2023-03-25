# GPTBOT for IRC

This is a simple chatbot IRC client that uses OpenAI's GPT model to generate responses to user messages. The chatbot connects to an IRC server and specified channels, listens for messages, and responds using the OpenAI API.

## Requirements

- Go (tested with version 1.21)
- [girc](https://github.com/lrstanley/girc) (IRC library)
- [go-openai](https://github.com/sashabaranov/go-openai) (OpenAI client library)

## Installation

1. Clone the repository.
2. Run `go build` in the repository folder.
3. Run the generated executable with the appropriate command line options.

## Configure the bot by setting up a config.yaml file or by using environment variables and command-line flags. 

- `--host`: IRC server address (default: "localhost")
- `--port`: IRC server port (default: 6667)
- `--nick`: Bot's nickname on the IRC server (default: "chatbot")
- `--channels`: Space-separated list of channels to join (e.g., "#channel1 #channel2")
- `--ssl`: Enable SSL for the IRC connection (default: false)
- `--preamble`: Text prepended to prompt (default: "provide a short reply of no more than 3 lines:")
- `--model`: Model to be used for responses (e.g., "gpt-4") (default: openai.GPT4)
- `--maxtokens`:  Maximum number of tokens to generate with the OpenAI model (default: 64)
- `--openaikey`: Api key for OpenAI 
- `--configfile`: config filename

## Environment Variables

- `CHATBOT_OPENAI_API_KEY`: Api key for OpenAI 

## Configuring with flags
```bash
export CHATBOT_OPENAI_API_KEY="<KEY>"
./gptbot --host irc.example.com --port 6667 --nick gptbot --channels "#general #gptbot"  
```

## Using a configuration file to define a personality

`obamabot.yml`
```
host: irc.example.com
nick: obamabot
channels: "#general #gptbot #yeswecan"
preamble: "provide a short reply of no more than 3 lines as president obama talking to the group chat. type like you are using a blackberry phone:"
openaikey: "your_openai_api_key"
```

```bash
./gptbot --config obamabot.yml
```

## Commands

Users can send the following commands to the bot:

- `/set preamble <value>`: Set the preamble used before each prompt.
- `/set model <value>`: Set the OpenAI model used for generating responses.

## Contributing

If you would like to contribute, please open an issue or submit a pull request on the project's GitHub repository.
