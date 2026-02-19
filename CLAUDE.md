# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

A Matrix chat bot that responds to @-mentions using the Anthropic Claude API. When mentioned in a room, it sends the message to Claude and replies in a Matrix thread, maintaining per-thread conversation history in memory. Supports tool use including web search, sandboxed filesystem access, and MCP server integration.

## Build and Run

```bash
go build -o matrix-claude-bot .
./matrix-claude-bot -config path/to/config.yaml
go test ./...
```

## Configuration

Config is loaded via Viper from `config.yaml` (searched in `$XDG_CONFIG_HOME/matrix-claude-bot/` or `$HOME/.config/matrix-claude-bot/`) or specified with `-config`. All values can be overridden with environment variables:

| Config key                    | Env var                    | Required |
|-------------------------------|----------------------------|----------|
| `matrix.homeserver_url`       | `MATRIX_HOMESERVER_URL`    | Yes      |
| `matrix.user_id`              | `MATRIX_USER_ID`           | Yes      |
| `matrix.access_token`         | `MATRIX_ACCESS_TOKEN`      | Yes      |
| `anthropic.api_key`           | `ANTHROPIC_API_KEY`        | Yes      |
| `claude.model`                | `CLAUDE_MODEL`             | No       |
| `claude.max_tokens`           | `CLAUDE_MAX_TOKENS`        | No       |
| `claude.system_prompt`        | `CLAUDE_SYSTEM_PROMPT`     | No       |
| `tools.web_search_enabled`    | `TOOLS_WEB_SEARCH_ENABLED` | No       |
| `tools.sandbox_dir`           | `TOOLS_SANDBOX_DIR`        | No       |
| `tools.max_iterations`        | `TOOLS_MAX_ITERATIONS`     | No       |
| `tools.timeout_seconds`       | `TOOLS_TIMEOUT_SECONDS`    | No       |
| `tools.mcp_servers`           | (YAML only)                | No       |

The Anthropic SDK reads its API key from the `ANTHROPIC_API_KEY` env var, which is set programmatically from the config in `main.go:loadConfig`.

## Architecture

All code is in package `main`:

- **main.go** -- Entrypoint. Defines `Config` struct, loads config via Viper, creates Matrix + Claude clients, sets up tool registry, wires up the `Bot`, and runs the Matrix sync loop with graceful shutdown on SIGINT/SIGTERM.
- **bot.go** -- Matrix event handling. `Bot` struct holds both clients, config, conversation store, tool registry, and start time. Registers handlers for message events (responds to mentions) and member events (auto-joins on invite). Messages are dispatched to goroutines. Replies are sent as Matrix threads.
- **claude.go** -- Claude API interaction and conversation state. `ConversationStore` is a thread-safe in-memory map keyed by Matrix thread root event ID. `getClaudeResponse` implements a tool use loop: sends messages to Claude, executes local tools when Claude returns `tool_use` stop reason, and loops until Claude returns text or max iterations is reached.
- **interfaces.go** -- `MatrixClient` and `ClaudeMessenger` interfaces for testability.
- **tools.go** -- `Tool` interface and `ToolRegistry` for managing local and server-side tools.
- **tool_filesystem.go** -- Sandboxed filesystem tools (`fs_read`, `fs_write`, `fs_list`) operating within a configured directory.
- **tool_mcp.go** -- MCP client manager for connecting to external MCP servers and exposing their tools to Claude.

## Tool Use

The bot supports three categories of tools:

1. **Web search** -- Anthropic's server-side web search (no local execution needed). Enable with `tools.web_search_enabled: true`.
2. **Filesystem** -- Read/write/list files in a sandboxed directory. Enable with `tools.sandbox_dir: /path/to/dir`.
3. **MCP servers** -- Connect to external MCP servers via stdio, SSE, or streamable HTTP transports. Configure in `tools.mcp_servers`.

Server-side tools (web search) produce `server_tool_use` / `web_search_tool_result` blocks handled by the Anthropic API. Local tools (filesystem, MCP) produce `tool_use` blocks executed by the bot and sent back as `tool_result`.

## Key Dependencies

- `maunium.net/go/mautrix` -- Matrix client SDK (mautrix-go)
- `github.com/anthropics/anthropic-sdk-go` -- Anthropic Claude SDK
- `github.com/spf13/viper` -- Configuration management
- `github.com/modelcontextprotocol/go-sdk` -- MCP client SDK
