# SoulShack: Because Conversations with Real People are Overrated

SoulShack lets you load any personality you want

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
- `--channel`: Channel to join (e.g., "#channel1 #channel2")
- `--ssl`: Enable SSL for the IRC connection (default: false)
- `--preamble`: Text prepended to prompt (default: "provide a short reply of no more than 3 lines:")
- `--model`: Model to be used for responses (e.g., "gpt-4") (default: openai.GPT4)
- `--maxtokens`:  Maximum number of tokens to generate with the OpenAI model (default: 64)
- `--openaikey`: Api key for OpenAI 
- `--config`: config filename

## Environment Variables

- `SOULSHACK_OPENAI_API_KEY`: Api key for OpenAI 

## Configuring with flags
```bash
export SOULSHACK_OPENAI_API_KEY="<KEY>"
./soulshack --host localhost --port 6667 --nick soulshack --channel "#soulshack"  
```

## Using a configuration file to define a personality

`personalities/obamabot.yml`
```
nick: obamabot
preamble: provide a short reply of no more than 3 lines as president obama talking to the group chat. type like you are using a blackberry phone...
greeting: give a rousing politically astute greeting to the group chat
goodbye: discuss the things you have to do with your wife, or that you have to polish your nobel award, or another type of common obama boast as you sign off from the chat
```

```bash
./soulshack --config obamabot --host localhost --port 6667 --channel "#yeswecan"
```

## Commands

Users can send the following commands to the bot:

- `/set preamble <value>`: Set the preamble used before each prompt.
- `/set model <value>`: Set the OpenAI model used for generating responses.
- `/set nick <value>`: Set the bot's ircnick
- `/set greeting <value>` Set the greeting prompt
- `/set goodbye <value>` Set the goodbye prompt

## Contributing

If you would like to contribute, please open an issue or submit a pull request on the project's GitHub repository.


## named as tribute to my old friend dayv, sp0t, who i think of often