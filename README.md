    //  ____                    _   ____    _                      _
    // / ___|    ___    _   _  | | / ___|  | |__     __ _    ___  | | __
    // \___ \   / _ \  | | | | | | \___ \  | '_ \   / _` |  / __| | |/ /
    //  ___) | | (_) | | |_| | | |  ___) | | | | | | (_| | | (__  |   <
    // |____/   \___/   \__,_| |_| |____/  |_| |_|  \__,_|  \___| |_|\_\
    //  .  .  .  because  real  people  are  overrated

soulshack is an IRC chatbot that can talk in channels and call tools. It supports multiple LLM providers (OpenAI, Anthropic, Google Gemini, and Ollama) and generates human‑like responses.

## features

- connects to an irc server and joins a specified channel
- utilizes the openai api and compatible endpoints like ollama
- allows dynamic configuration of bot settings through commands
- supports ssl and SASL authentication for irc servers
- tool calling, example tools, channel management tools

## building

```bash
go build .
```

```
docker build . -t soulshack:dev
```

## usage

Quick examples (model must include provider prefix):

- OpenAI
  ```bash
  SOULSHACK_OPENAIKEY=sk-... \
  soulshack --nick chatbot --server irc.example.net --port 6697 --tls \
    --channel '#soulshack' \
    --model openai/gpt-4o
  ```

- Anthropic
  ```bash
  SOULSHACK_ANTHROPICKEY=sk-ant-... \
  soulshack --channel '#soulshack' --model anthropic/claude-3-7-sonnet-20250219
  ```

- Gemini
  ```bash
  SOULSHACK_GEMINIKEY=AIza... \
  soulshack --channel '#soulshack' --model gemini/gemini-2.5-flash
  ```

- Ollama (local)
  ```bash
  soulshack --channel '#soulshack' --model ollama/llama3.2
  ```

See examples/chatbot.yml for a working config file (uses an anthropic/* model as an example).

To enable shell tools, pass tool paths:
```bash
soulshack --channel '#soulshack' --model ollama/llama3.2 \
  --shelltools examples/tools/datetime.sh,examples/tools/weather.py
```

## configuration

soulshack can be configured using command line flags, environment variables, or configuration files. 

### flags
```
Usage:
  soulshack [flags]

Connection:
  -n, --nick string                bot's nickname on the IRC server (default "soulshack")
  -s, --server string              IRC server address (default "localhost")
  -p, --port int                   IRC server port (default 6667)
  -c, --channel string             IRC channel to join
      --saslnick string            nick used for SASL
      --saslpass string            password for SASL PLAIN
  -e, --tls                        enable TLS for the IRC connection
      --tlsinsecure                skip TLS certificate verification

Config & logging:
  -b, --config string              use the named configuration file (YAML)
  -A, --admins strings             comma-separated list of allowed hostmasks to administrate the bot
  -v, --verbose                    enable verbose logging of sessions and configuration

LLM/API configuration:
      --openaikey string           OpenAI API key
      --openaiurl string           OpenAI API URL (for custom/compatible endpoints)
      --anthropickey string        Anthropic API key
      --geminikey string           Google Gemini API key
      --ollamaurl string           Ollama API URL (default "http://localhost:11434")
      --ollamakey string           Ollama API key (Bearer token for authentication)
      --model string               model to be used (provider/name, default "ollama/llama3.2")
      --maxtokens int              maximum number of tokens to generate (default 4096)
  -t, --apitimeout duration        timeout for each completion request (default 5m0s)
      --temperature float32        temperature for the completion (default 0.7)
      --top_p float32              top P value for the completion (default 1)
      --shelltools strings         comma-separated list of shell tool paths to load
      --irctools strings           comma-separated list of IRC tools to enable (default: irc_op,irc_kick,irc_topic,irc_action)

Behavior & session:
  -a, --addressed                  require bot be addressed by nick for response (default true)
  -S, --sessionduration duration   message context will be cleared after it is unused for this duration (default 10m0s)
  -H, --sessionhistory int         maximum number of lines of context to keep per session (default 250)
  -m, --chunkmax int               maximum number of characters to send as a single message (default 350)

Prompting:
      --greeting string            prompt to be used when the bot joins the channel (default "hello.")
      --prompt string              initial system prompt (default "you are a helpful chatbot. do not use caps. do not use emoji.")
```

### environment variables

all flags can also be set via environment variables with the prefix `soulshack_`. for example, `soulshack_server` for the `--server` flag.


### configuration files

configuration files use the yaml format. they can be loaded using the `--config` flag. a configuration file can contain any of the settings that can be set via flags.



## commands

- `/set <key> <value>`: set a configuration parameter (e.g., `/set model ollama/llama3.2`)
- `/get <key>`: get the current value of a configuration parameter (e.g., `/get model`)
- `/leave`: make the bot leave the channel and exit
- `/help`: display help for available commands

Modifiable parameters via `/set` and `/get`:
- `model` - LLM model to use
- `addressed` - require bot to be addressed by nick
- `prompt` - system prompt for the bot
- `maxtokens` - maximum tokens in response
- `temperature` - LLM temperature (0.0-2.0)
- `top_p` - top-p sampling parameter
- `admins` - comma-separated admin hostmasks
- `openaiurl` - OpenAI API endpoint
- `ollamaurl` - Ollama API endpoint
- `ollamakey` - Ollama API key (masked when reading)
- `openaikey` - OpenAI API key (masked when reading)
- `anthropickey` - Anthropic API key (masked when reading)
- `geminikey` - Gemini API key (masked when reading)
- `shelltools` - comma-separated shell tool paths
- `irctools` - comma-separated IRC tools (irc_op, irc_kick, irc_topic, irc_action)


## tools

Tools are enabled by providing paths to tool scripts via the `--shelltools` flag or configuration file. Each tool must be an executable that responds to the following commands:

- --schema: Outputs a JSON schema describing the tool use.
- --execute $json: Will be called with JSON matching your schema


try to make sure the llm can't inject something in the execute json that will ruin your life. 
because you can't trust bots or the people who use them.

to be honest you shouldn't do this will shell scripts, it's kind of a minefield.
so here's a shell script

```bash
set -e

# Check if --schema argument is provided
if [[ "$1" == "--schema" ]]; then
  # Output a JSON schema to describe the tool
  # shellcheck disable=SC2016
  cat <<EOF
  {
  "name": "get_current_date_with_format",
  "description": "provides the current time and date in the specified unix date command format",
  "type": "object",
  "properties": {
    "format": {
      "type": "string",
      "description": "The format for the date. use unix date command format (e.g., +%Y-%m-%d %H:%M:%S). always include the leading + sign."
    }
  },
  "required": ["format"],
  "additionalProperties": false
  }
EOF
  exit 0
fi



if [[ "$1" == "--execute" ]]; then
  # Ensure jq is available
  if ! command -v jq &> /dev/null; then
    echo "Error: jq is not installed." >&2
    exit 1
  fi

  # Extract format field from JSON
  format=$(jq -r '.format' <<< "$2")

  # Sanitize the format string
  if [[ "$format" =~ [^a-zA-Z0-9%+:/\ \-] ]]; then
    echo "Error: Invalid characters in format string." >&2
    exit 1
  fi

  # Use -- to prevent option parsing
  date_output=$(date -- "$format")
  echo "$date_output"
  exit 0
fi


# if no arguments, show usage
# shellcheck disable=SC2140
echo "Usage: currentdate.sh [--schema | --execute '{\"format\": \"+%Y-%m-%d %H:%M:%S\"}']"
```

![jacob, high five me](https://i.imgur.com/CDccJ5r.png)

## named as tribute to my old friend dayv, sp0t, who i think of often
