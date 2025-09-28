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
- unified tool system (shell scripts, MCP servers, IRC tools)

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

To enable tools (shell scripts, MCP servers, or IRC tools), use the unified `--tool` flag:
```bash
# Shell tools
soulshack --channel '#soulshack' --model ollama/llama3.2 \
  --tool examples/tools/datetime.sh --tool examples/tools/weather.py

# MCP servers (use JSON config files)
soulshack --channel '#soulshack' --model ollama/llama3.2 \
  --tool filesystem.json --tool git-server.json

# IRC tools (use irc_ prefix)
soulshack --channel '#soulshack' --model ollama/llama3.2 \
  --tool irc_op --tool irc_kick --tool irc_action

# Mix all tool types together
soulshack --channel '#soulshack' --model ollama/llama3.2 \
  --tool examples/tools/weather.py --tool mcp-time.json --tool irc_topic
```

MCP server JSON config example (`filesystem.json`):
```json
{
  "command": "npx",
  "args": ["@modelcontextprotocol/server-filesystem", "/tmp"],
  "env": {}
}
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
      --tool strings               tools to load (shell scripts, MCP server JSON files, or IRC tools with irc_ prefix, can be specified multiple times or comma-separated)
      --thinking                   enable thinking/reasoning mode for supported models
      --showthinkingaction         show "[thinking]" IRC action when reasoning (default true)
      --showtoolactions            show "[calling toolname]" IRC actions when executing tools (default true)

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

### Tool Management Commands

- `/get tools` - List all loaded tools (comma-separated)
- `/set tools add <spec>` - Add a tool (shell script path, MCP JSON path, or irc_* name)
- `/set tools remove <name or pattern>` - Remove tools by name or wildcard pattern
  - Exact: `script__weather` - removes that specific tool
  - Wildcards: `filesystem__*` - removes all filesystem tools
  - `script__*` - removes all shell script tools
  - `irc_*` - removes all IRC tools

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
- `thinking` - enable thinking/reasoning mode for supported models (true/false)
- `showthinkingaction` - show "[thinking]" IRC action when reasoning (true/false)
- `showtoolactions` - show "[calling toolname]" IRC actions when executing tools (true/false)


## tools

Soulshack uses a unified tool system that automatically detects the tool type. Tools are enabled via the `--tool` flag or configuration file.

Tools have namespaced names:
- Shell scripts: `filename__weather`
- MCP tools: `filesystem__read_file`
- IRC tools: `irc_op` (no namespace prefix)


### Shell Tools

Shell tools are executable scripts that are automatically detected when you provide a path to an executable file. Each tool must be an executable that responds to the following commands:

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
  "title": "get_current_date_with_format",
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

### MCP Servers

Soulshack can connect to MCP (Model Context Protocol) servers, which provide tools via a standardized protocol. MCP servers must be configured using JSON files that specify the command and arguments.

Create a JSON config file for each MCP server:
```json
// filesystem.json
{
  "command": "npx",
  "args": ["@modelcontextprotocol/server-filesystem", "/tmp"]
}

// git.json
{
  "command": "uvx",
  "args": ["mcp-server-git"]
}
```

Then use them with the `--tool` flag:
```bash
# Multiple servers
--tool filesystem.json --tool git.json --tool time-server.json
```

MCP servers automatically expose their available tools to the bot. For more information about MCP, see [modelcontextprotocol.io](https://modelcontextprotocol.io).

### IRC Tools

Built-in IRC channel management tools are loaded using the same `--tool` flag with `irc_` prefix:
- `irc_op` - Grant/revoke operator status
- `irc_kick` - Kick users from the channel
- `irc_topic` - Change the channel topic
- `irc_action` - Send /me actions

Example:
```bash
# Enable specific IRC tools
soulshack --channel '#soulshack' --model ollama/llama3.2 \
  --tool irc_op --tool irc_action

# Or configure in YAML
tool:
  - irc_op
  - irc_kick
  - irc_topic
  - irc_action
```

These tools require appropriate channel permissions 

![jacob, high five me](https://i.imgur.com/CDccJ5r.png)

## named as tribute to my old friend dayv, sp0t, who i think of often
