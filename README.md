    //  ____                    _   ____    _                      _
    // / ___|    ___    _   _  | | / ___|  | |__     __ _    ___  | | __
    // \___ \   / _ \  | | | | | | \___ \  | '_ \   / _` |  / __| | |/ /
    //  ___) | | (_) | | |_| | | |  ___) | | | | | | (_| | | (__  |   <
    // |____/   \___/   \__,_| |_| |____/  |_| |_|  \__,_|  \___| |_|\_\
    //  .  .  .  because  real  people  are  overrated

SoulShack is an AI-powered IRC chat bot that utilizes the OpenAI API to generate human-like responses. 


## Dependencies

- Go (tested with version 1.21)
- Cobra: https://github.com/spf13/cobra
- GIRC: https://github.com/lrstanley/girc
- Go Figure: https://github.com/common-nighthawk/go-figure
- OpenAI Go Client: https://github.com/sashabaranov/go-openai
- Viper: https://github.com/spf13/viper

## Features

- Connects to an IRC server and joins a specified channel
- Utilizes the OpenAI GPT-4 model to generate realistic and human-like responses
- Allows dynamic configuration of bot settings through commands
- Supports SSL connections for secure communication
- Can adopt various personalities by changing configuration files


## Building

```bash
go build .
```

```
docker build . -t soulshack:dev
```

## Usage

```bash
soulshack --nick chatbot --server irc.freenode.net --port 6697 --channel '#soulshack' --ssl --openaikey ****************
```

## Configuration

SoulShack can be configured using command line flags, environment variables, or personality configuration files. It uses Viper to manage configuration settings.

### Flags
```
Usage:
  soulshack [flags]

Examples:
soulshack --nick chatbot --server irc.freenode.net --port 6697 --channel '#soulshack' --ssl --openaikey ****************

Flags:
  -A, --admins strings     comma-separated list of allowed users to administrate the bot (e.g., user1,user2,user3)
  -b, --become string      become the named personality (default "chatbot")
  -c, --channel string     irc channel to join
  -d, --directory string   personalities configuration directory (default "./personalities")
      --goodbye string     prompt to be used when the bot leaves the channel (default "goodbye.")
      --greeting string    prompt to be used when the bot joins the channel (default "hello.")
  -h, --help               help for soulshack
  -H, --history int        maximum number of lines of context to keep per session (default 15)
  -l, --list               list configured personalities
      --maxtokens int      maximum number of tokens to generate (default 512)
      --model string       model to be used for responses (e.g., gpt-4 (default "gpt-4")
  -n, --nick string        bot's nickname on the irc server
      --openaikey string   openai api key
  -p, --port int           irc server port (default 6667)
      --prompt string      initial system prompt for the ai
  -s, --server string      irc server address (default "localhost")
  -S, --session duration   duration for the chat session; message context will be cleared after this time (default 3m0s)
  -e, --ssl                enable SSL for the IRC connection
  -t, --timeout duration   timeout for each completion request to openai (default 30s)
  -v, --verbose            enable verbose logging of sessions and configuration
      --version            version for soulshack
```

### Environment Variables

All flags can also be set via environment variables with the prefix `SOULSHACK_`. For example, `SOULSHACK_SERVER` for the `--server` flag.

### Personality Configuration Files

Personality configuration files are stored in the `personalities` directory and use the YAML format. They can be loaded using the `--become` flag. A personality file can contain any of the settings that can be set via flags.

# Adding a Personality

To add a new personality to SoulShack, follow these steps:

1. Create a new YML file in the `personalities` directory with the desired name (e.g., `marvin.yml`).
2. Add your desired settings to the YML file. For example:

```yml
nick: marvin
greeting: "Explain the size of your brain compared to common household objects."
goodbye: "goodbye."
prompt: "you are marvin the paranoid android. respond with a short text message: "
```

```bash
soulshack --server localhost --channel '#marvinshouse' --become marvin 
```

## Commands

- `/set`: Set a configuration parameter (e.g., `/set nick NewNick`)
- `/get`: Get the current value of a configuration parameter (e.g., `/get nick`)
- `/save`: Save the current configuration as a personality (e.g., `/save mypersonality`)
- `/become`: Adopt a new personality (e.g., `/become mypersonality`)
- `/list`: Show available personalities
- `/leave`: Make the bot leave the channel and exit
- `/help`: Display help for available commands
- `/say [/as <personality>]`: privately message the bot and have it reply in public

[jacob, high five me](https://i.redd.it/8y2blwiyvira1.png)

## named as tribute to my old friend dayv, sp0t, who i think of often
