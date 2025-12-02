    //  ____                    _   ____    _                      _
    // / ___|    ___    _   _  | | / ___|  | |__     __ _    ___  | | __
    // \___ \   / _ \  | | | | | | \___ \  | '_ \   / _` |  / __| | |/ /
    //  ___) | | (_) | | |_| | | |  ___) | | | | | | (_| | | (__  |   <
    // |____/   \___/   \__,_| |_| |____/  |_| |_|  \__,_|  \___| |_|\_\
    //  .  .  .  because  real  people  are  overrated  [v0.91]

## overview

soulshack is an IRC chatbot powered by LLMs. supports OpenAI, Anthropic, Google Gemini, and Ollama.

- multi-provider llm support (openai, anthropic, gemini, ollama)
- unified tool system (shell scripts, MCP servers, IRC channel tools)
- ssl/tls and SASL authentication
- session management with configurable history and ttl
- streaming responses with irc-appropriate chunking
- passive url watching mode
- runtime configuration via irc commands

## quickstart

```bash
go build ./cmd/soulshack
```

```bash
docker build . -t soulshack:dev
```

model format is `provider/model-name`:

```bash
# openai
SOULSHACK_OPENAIKEY=sk-... \
soulshack --nick chatbot --server irc.example.net --port 6697 --tls \
  --channel '#soulshack' --model openai/gpt-5

# anthropic
SOULSHACK_ANTHROPICKEY=sk-ant-... \
soulshack --channel '#soulshack' --model anthropic/claude-sonnet-4-5-20250929

# gemini
SOULSHACK_GEMINIKEY=AIza... \
soulshack --channel '#soulshack' --model gemini/gemini-3-pro-preview

# ollama (local)
soulshack --channel '#soulshack' --model ollama/llama3.2
```

with tools:

```bash
soulshack --channel '#soulshack' --model ollama/llama3.2 \
  --tool examples/tools/datetime.sh \
  --tool examples/mcp/filesystem.json \
  --tool irc_op --tool irc_kick
```

see `examples/chatbot.yml` for a full config example.

## configuration

configure via flags, environment variables (`SOULSHACK_*`), or yaml config file (`--config`).

### flags

| flag | default | description |
|------|---------|-------------|
| `-n, --nick` | soulshack | bot nickname |
| `-s, --server` | localhost | irc server |
| `-p, --port` | 6667 | irc port |
| `-c, --channel` | | channel to join |
| `-e, --tls` | false | enable tls |
| `--tlsinsecure` | false | skip cert verification |
| `--saslnick` | | sasl username |
| `--saslpass` | | sasl password |
| `-b, --config` | | yaml config file |
| `-A, --admins` | | admin hostmasks (comma-separated) |
| `-V, --verbose` | false | debug logging |
| `--model` | ollama/llama3.2 | llm model (provider/name) |
| `--maxtokens` | 4096 | max response tokens |
| `--temperature` | 0.7 | sampling temperature |
| `--top_p` | 1.0 | nucleus sampling |
| `-t, --apitimeout` | 5m | api request timeout |
| `--openaikey` | | openai api key |
| `--openaiurl` | | custom openai endpoint |
| `--anthropickey` | | anthropic api key |
| `--geminikey` | | gemini api key |
| `--ollamaurl` | http://localhost:11434 | ollama endpoint |
| `--ollamakey` | | ollama bearer token |
| `--tool` | | tool to load (repeatable) |
| `--thinking` | false | enable reasoning mode |
| `--showthinkingaction` | true | show [thinking] irc action |
| `--showtoolactions` | true | show [calling tool] irc action |
| `--urlwatcher` | false | passive url watching |
| `-a, --addressed` | true | require nick addressing |
| `-S, --sessionduration` | 10m | session ttl |
| `-H, --sessionhistory` | 250 | max history lines |
| `-m, --chunkmax` | 350 | max chars per message |
| `--prompt` | (default) | system prompt |
| `--greeting` | hello. | channel join greeting |

## commands

| command | admin | description |
|---------|-------|-------------|
| `/set <key> <value>` | yes | set config parameter |
| `/get <key>` | no | get config parameter |
| `/tools` | no | list loaded tools |
| `/tools add <spec>` | yes | add tool (path or irc_*) |
| `/tools remove <pattern>` | yes | remove tool(s) by name/pattern |
| `/admins` | yes | list bot admins |
| `/admins add <hostmask>` | yes | add admin hostmask |
| `/admins remove <hostmask>` | yes | remove admin hostmask |
| `/help` | no | show available commands |
| `/version` | no | show bot version |

### configurable parameters

`model`, `prompt`, `maxtokens`, `temperature`, `top_p`, `addressed`, `openaiurl`, `ollamaurl`, `ollamakey`, `openaikey`, `anthropickey`, `geminikey`, `thinking`, `showthinkingaction`, `showtoolactions`, `sessionduration`, `apitimeout`, `sessionhistory`, `chunkmax`, `urlwatcher`

## tools

unified tool system supporting shell scripts, MCP servers, and IRC tools.

tool names are namespaced: `script__datetime`, `filesystem__read_file`, `irc_op`

### shell tools

executable scripts responding to `--schema` (json) and `--execute <json>`:

```bash
#!/bin/bash
if [[ "$1" == "--schema" ]]; then
  cat <<EOF
{
  "title": "get_date",
  "description": "get current date",
  "type": "object",
  "properties": {
    "format": { "type": "string", "description": "date format" }
  },
  "required": ["format"]
}
EOF
  exit 0
fi

if [[ "$1" == "--execute" ]]; then
  format=$(jq -r '.format' <<< "$2")
  date -- "$format"
fi
```

### MCP servers

json config for MCP protocol servers:

```json
{
  "command": "npx",
  "args": ["@modelcontextprotocol/server-filesystem", "/tmp"]
}
```

see [modelcontextprotocol.io](https://modelcontextprotocol.io)

### IRC tools

built-in channel management (require appropriate permissions):

| tool | description |
|------|-------------|
| `irc_op` | grant/revoke operator status |
| `irc_kick` | kick users |
| `irc_ban` | ban/unban users |
| `irc_topic` | set channel topic |
| `irc_action` | send /me action |
| `irc_mode_set` | set channel modes |
| `irc_mode_query` | query channel modes |
| `irc_invite` | invite users |
| `irc_names` | list channel users |
| `irc_whois` | user info lookup |

![jacob, high five me](https://i.imgur.com/CDccJ5r.png)

## named as tribute to my old friend dayv, sp0t, who i think of often
