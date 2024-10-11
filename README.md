    //  ____                    _   ____    _                      _
    // / ___|    ___    _   _  | | / ___|  | |__     __ _    ___  | | __
    // \___ \   / _ \  | | | | | | \___ \  | '_ \   / _` |  / __| | |/ /
    //  ___) | | (_) | | |_| | | |  ___) | | | | | | (_| | | (__  |   <
    // |____/   \___/   \__,_| |_| |____/  |_| |_|  \__,_|  \___| |_|\_\
    //  .  .  .  because  real  people  are  overrated

soulshack is an irc chat bot that utilizes an openai api endpoint to generate human-like responses. 

## features

- connects to an irc server and joins a specified channel
- utilizes the openai api and compatible endpoints like ollama
- allows dynamic configuration of bot settings through commands
- supports ssl and SASL


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

soulshack can be configured using command line flags, environment variables, or configuration files. 

### flags
```
Usage:
  soulshack --channel <channel> [--nick <nickname>] [--server <server>] [--port <port>] [--tls] [--openaikey <key>] [flags]

Examples:
soulshack --nick chatbot --server irc.freenode.net --port 6697 --channel '#soulshack' --tls --openaikey ****************

Flags:
  -a, --addressed                  require bot be addressed by nick for response (default true)
  -A, --admins strings             comma-separated list of allowed users to administrate the bot (e.g., user1,user2,user3)
  -t, --apitimeout duration        timeout for each completion request (default 5m0s)
  -c, --channel string             irc channel to join
  -C, --chunkdelay duration        after this delay, bot will look to split the incoming buffer on sentence boundaries (default 15s)
  -m, --chunkmax int               maximum number of characters to send as a single message (default 350)
  -b, --config string              use the named configuration file
      --greeting string            prompt to be used when the bot joins the channel (default "hello.")
  -h, --help                       help for soulshack
      --maxtokens int              maximum number of tokens to generate (default 512)
      --model string               model to be used for responses (default "gpt-4o")
  -n, --nick string                bot's nickname on the irc server (default "soulshack")
      --openaikey string           openai api key
      --openaiurl string           alternative base url to use instead of openai
  -p, --port int                   irc server port (default 6667)
      --prompt string              initial system prompt (default "you are a helpful chatbot. do not use caps. do not use emoji.")
      --reactmode                  enable ReAct mode for the bot
      --saslnick string            nick used for SASL
      --saslpass string            password for SASL plain
  -s, --server string              irc server address (default "localhost")
  -S, --sessionduration duration   duration for the chat session; message context will be cleared after this time (default 3m0s)
  -H, --sessionhistory int         maximum number of lines of context to keep per session (default 50)
      --temperature float32        temperature for the completion (default 0.7)
  -e, --tls                        enable TLS for the IRC connection
      --tlsinsecure                skip TLS certificate verification
      --tools                      enable tool use (default false)
      --toolsdir                   directory to load tools from (default "examples/tools")
      --top_p float32              top P value for the completion (default 1)
  -v, --verbose                    enable verbose logging of sessions and configuration
      --version                    version for soulshack
```

### environment variables

all flags can also be set via environment variables with the prefix `soulshack_`. for example, `soulshack_server` for the `--server` flag.

### configuration files

configuration files use the yaml format. they can be loaded using the `--config` flag. a configuration file can contain any of the settings that can be set via flags.

# adding a config

to use create a new configuration file, follow these steps:

1. create a new yml file with the desired name (e.g., `marvin.yml`).
2. add your desired settings to the yml file. for example:

```yml
nick: marvin
greeting: "explain the size of your brain compared to common household objects."
prompt: "you are marvin the paranoid android. respond with a short text message: "
channel: "#marvinshouse"
server: localhost
```

```bash
soulshack --config /path/to/marvin.yml 
```

## commands

- `/set`: set a configuration parameter (e.g., `/set nick newnick`)
- `/get`: get the current value of a configuration parameter (e.g., `/get nick`)
- `/leave`: make the bot leave the channel and exit
- `/help`: display help for available commands


## tools

put a an executable or script in your tooldir location. in order for it to be registerd it must respond to the following commands:

- --schema: Outputs a JSON schema describing the tool use.
- --name: Outputs the name of the tool.
- --description: Outputs a description of the tool.
- --execute $json: Will be called with JSON matching your schema


try to make sure the llm can't inject something in the execute json that will ruin your life. 
because you can't trust bots or the people who use them.

```bash
#!/bin/bash

set -e

# Check if --schema argument is provided
if [[ "$1" == "--schema" ]]; then
  # Output a JSON schema to describe the tool
  # shellcheck disable=SC2016
  cat <<EOF
{
  "schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "properties": {
    "format": {
      "type": "string",
      "description": "The format for the date output (e.g., +%Y-%m-%d %H:%M:%S)"
    }
  },
  "required": ["format"],
  "additionalProperties": false
}
EOF
  exit 0
fi

if [[ "$1" == "--name" ]]; then
  echo "get_current_date_with_format"
  exit 0
fi

if [[ "$1" == "--description" ]]; then
  echo "provides the current date in the specified unix date command format"
  exit 0
fi

if [[ "$1" == "--execute" ]]; then

  # extract format field from JSON
  format=$(jq -r '.format' <<< "$2")

  # Use exec to prevent command injection
  exec date "$format"
  exit 0
fi

# if no arguments, show usage
echo "Usage: currentdate.sh [--schema | --name | --description | --execute <format>]"
```
![jacob, high five me](https://i.imgur.com/CDccJ5r.png)

## named as tribute to my old friend dayv, sp0t, who i think of often
