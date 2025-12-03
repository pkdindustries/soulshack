# Modifying Soulshack Code

This guide provides an overview of the codebase and instructions for common modification tasks.

## Project Structure

-   **`cmd/soulshack/`**: Application entry point (`main.go`).
-   **`internal/bot/`**: Core bot runtime, system initialization, and event loop.
-   **`internal/commands/`**: Implementation of IRC commands (e.g., `/help`, `/tools`).
-   **`internal/config/`**: Configuration loading and validation.
-   **`internal/core/`**: Core interfaces (`ChatContextInterface`, `System`, `LLM`) and types.
-   **`internal/irc/`**: IRC connection handling, context management, and message parsing.
-   **`internal/llm/`**: Integration with the `pollytool` library for LLM capabilities.

## Common Tasks

### Adding a New Command

1.  **Create a new file** in `internal/commands/` (e.g., `mycommand.go`).
2.  **Implement the `Command` interface**:
    ```go
    type MyCommand struct{}

    func (c *MyCommand) Name() string { return "/mycommand" }
    func (c *MyCommand) AdminOnly() bool { return false }
    func (c *MyCommand) Execute(ctx irc.ChatContextInterface) {
        ctx.Reply("Hello from MyCommand!")
    }
    ```
3.  **Register the command** in `internal/bot/run.go`:
    ```go
    cmdRegistry.Register(&commands.MyCommand{})
    ```

### Adding a Tool

Soulshack supports a unified tool system that includes native Go tools, Shell scripts, and MCP servers.

#### 1. Shell Tools
Shell tools are executable scripts that implement a simple protocol:
-   `--schema`: Output JSON schema for the tool.
-   `--execute <json_args>`: Execute the tool with the given arguments.

**Example (`get_date.sh`):**
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

To use: Add the script path to your config or use `/tools add ./get_date.sh`.

#### 2. MCP Servers
Soulshack supports the [Model Context Protocol](https://modelcontextprotocol.io). You can load MCP servers by providing a JSON configuration file.

**Example (`filesystem.json`):**
```json
{
  "command": "npx",
  "args": ["@modelcontextprotocol/server-filesystem", "/tmp"],
  "env": {
    "DEBUG": "true"
  }
}
```

To use: Add the JSON file path to your config or use `/tools add ./filesystem.json`.

#### 3. Native Go Tools
1.  Implement the tool using the `pollytool/tools` interface.
2.  Register it in `internal/bot/system.go` or via `internal/irc/tools.go`.

### Modifying LLM Logic

-   **`internal/llm/polly.go`**: Handles the interaction with the LLM agent.
-   **`internal/llm/completion.go`**: Constructs the completion request.

To change how the bot prompts the LLM or handles the stream, modify `ChatCompletionStream` in `polly.go`.

## Testing

Run unit tests using standard Go tooling:

```bash
go test ./...
```

For integration testing, you may need to mock the IRC connection or LLM responses. See `internal/testing/` for mock implementations.
