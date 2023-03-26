    //  ____                    _   ____    _                      _
    // / ___|    ___    _   _  | | / ___|  | |__     __ _    ___  | | __
    // \___ \   / _ \  | | | | | | \___ \  | '_ \   / _` |  / __| | |/ /
    //  ___) | | (_) | | |_| | | |  ___) | | | | | | (_| | | (__  |   <
    // |____/   \___/   \__,_| |_| |____/  |_| |_|  \__,_|  \___| |_|\_\
    //  .  .  .  because  real  people  are  overrated


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

- `--server`: IRC server address (default: "localhost")
- `--port`: IRC server port (default: 6667)
- `--nick`: Bot's nickname on the IRC server (default: "chatbot")
- `--channel`: Channel to join (e.g., "#channel1 #channel2")
- `--ssl`: Enable SSL for the IRC connection (default: false)
- `--prompt`: Text prepended to prompt (default: "provide a short reply of no more than 3 lines:")
- `--model`: Model to be used for responses (e.g., "gpt-4") (default: openai.GPT4)
- `--maxtokens`:  Maximum number of tokens to generate with the OpenAI model (default: 64)
- `--openaikey`: Api key for OpenAI 
- `--become`: personality module
- `--greeting`: greeting prompt
- `--goodbye`: goodbye prompt


## Environment Variables

- `SOULSHACK_OPENAIKEY`: Api key for OpenAI 

## Configuring with flags
```bash
export SOULSHACK_OPENAIKEY="<KEY>"
./soulshack --server localhost --port 6667 --nick soulshack --channel "#chatbot" --model gpt-4 --prompt 'repond like a angry dog' --greeting 'bark' --goodbye 'whimper'
```

## Using a configuration file to define a personality

`personalities/obama.yml`
```
nick: obamabot
prompt: provide a short reply of no more than 3 lines as president obama talking to the group chat. type like you are using a blackberry phone...
greeting: give a rousing politically astute greeting to the group chat
goodbye: discuss the things you have to do with your wife, or that you have to polish your nobel award, or another type of common obama boast as you sign off from the chat
```

```bash
./soulshack --become obamabot --server localhost --port 6667 --channel "#yeswecan"
```

## Commands

Users can send the following commands to the bot:

- `/set prompt <value>`: Set the prompt used before each prompt.
- `/set model <value>`: Set the OpenAI model used for generating responses.
- `/set nick <value>`: Set the bot's ircnick
- `/set greeting <value>` Set the greeting prompt
- `/set goodbye <value>` Set the goodbye prompt

## Contributing

If you would like to contribute, please open an issue or submit a pull request on the project's GitHub repository.


## named as tribute to my old friend dayv, sp0t, who i think of often