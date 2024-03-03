    //  ____                    _   ____    _                      _
    // / ___|    ___    _   _  | | / ___|  | |__     __ _    ___  | | __
    // \___ \   / _ \  | | | | | | \___ \  | '_ \   / _` |  / __| | |/ /
    //  ___) | | (_) | | |_| | | |  ___) | | | | | | (_| | | (__  |   <
    // |____/   \___/   \__,_| |_| |____/  |_| |_|  \__,_|  \___| |_|\_\
    //  .  .  .  because  real  people  are  overrated

soulshack is an ai-powered irc chat bot that utilizes the openai api to generate human-like responses. 

## features

- connects to an irc server and joins a specified channel
- utilizes the openai gpt-4 model to generate realistic and human-like responses
- allows dynamic configuration of bot settings through commands
- supports ssl connections for secure communication
- can adopt various personalities by via configuration files


## building

```bash
go build .
```

```
docker build . -t soulshack:dev
```

## usage

```bash
soulshack --nick chatbot --server irc.freenode.net --port 6697 --channel '#soulshack' --ssl --openaikey ****************
```

## configuration

soulshack can be configured using command line flags, environment variables, or personality configuration files. it uses viper to manage configuration settings.

### flags
```
Usage:
  soulshack [flags]

Examples:
soulshack --nick chatbot --server irc.freenode.net --port 6697 --channel '#soulshack' --ssl --openaikey ****************

Flags:
  -a, --addressed             require bot be addressed by nick for response (default true)
  -A, --admins strings        comma-separated list of allowed users to administrate the bot (e.g., user1,user2,user3)
  -b, --become string         become the named personality (default "chatbot")
  -c, --channel string        irc channel to join
  -C, --chunkdelay duration   after this delay, bot will look to split the incoming buffer on sentence boundaries (default 7s)
  -m, --chunkmax int          maximum number of characters to send as a single message (default 350)
  -d, --directory string      personalities configuration directory (default "./personalities")
      --goodbye string        prompt to be used when the bot leaves the channel (default "goodbye.")
      --greeting string       prompt to be used when the bot joins the channel (default "hello.")
  -h, --help                  help for soulshack
  -H, --history int           maximum number of lines of context to keep per session (default 15)
  -l, --list                  list configured personalities
      --maxtokens int         maximum number of tokens to generate (default 512)
      --model string          model to be used for responses e.g., gpt-4 (default "gpt-4")
  -n, --nick string           bot's nickname on the irc server (default "soulshack")
      --openaikey string      openai api key
      --openaiurl string      alternative base url to use instead of openai
  -p, --port int              irc server port (default 6667)
      --prompt string         initial system prompt for the ai (default "respond in a short text:")
  -s, --server string         irc server address (default "localhost")
  -S, --session duration      duration for the chat session; message context will be cleared after this time (default 3m0s)
  -e, --ssl                   enable SSL for the IRC connection
  -t, --timeout duration      timeout for each completion request to openai (default 1m0s)
  -v, --verbose               enable verbose logging of sessions and configuration
      --version               version for soulshack
```

### environment variables

all flags can also be set via environment variables with the prefix `soulshack_`. for example, `soulshack_server` for the `--server` flag.

### personality configuration files

personality configuration files are stored in the `personalities` directory and use the yaml format. they can be loaded using the `--become` flag. a personality file can contain any of the settings that can be set via flags.

# adding a personality

to add a new personality to soulshack, follow these steps:

1. create a new yml file in the `personalities` directory with the desired name (e.g., `marvin.yml`).
2. add your desired settings to the yml file. for example:

```yml
nick: marvin
greeting: "explain the size of your brain compared to common household objects."
goodbye: "goodbye."
prompt: "you are marvin the paranoid android. respond with a short text message: "
```

```bash
soulshack --server localhost --channel '#marvinshouse' --become marvin 
```

## commands

- `/set`: set a configuration parameter (e.g., `/set nick newnick`)
- `/get`: get the current value of a configuration parameter (e.g., `/get nick`)
- `/save`: save the current configuration as a personality (e.g., `/save mypersonality`)
- `/become`: adopt a new personality (e.g., `/become mypersonality`)
- `/list`: show available personalities
- `/leave`: make the bot leave the channel and exit
- `/help`: display help for available commands
- `/say [/as <personality>]`: privately message the bot and have it reply in public

[jacob, high five me](https://i.redd.it/8y2blwiyvira1.png)

## named as tribute to my old friend dayv, sp0t, who i think of often
