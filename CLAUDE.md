# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

A Matrix chat bot that responds to @-mentions using the Anthropic Claude API. When mentioned in a room, it sends the message to Claude and replies in a Matrix thread, maintaining per-thread conversation history in memory. Supports tool use including web search, sandboxed filesystem access, and MCP server integration. Optionally supports E2EE via mautrix-go's cryptohelper.

## Build and Run

```bash
go build -tags goolm -o matrix-claude-bot ./cmd/claude-bot
./matrix-claude-bot -config path/to/config.yaml
```

The `goolm` build tag selects the pure-Go Olm implementation (no CGO/libolm required).

Run tests with `go test -tags goolm ./...`. Integration tests (tagged `integration`) require a `config.test.yaml`.

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
| `crypto.pickle_key`           | `CRYPTO_PICKLE_KEY`        | No       |
| `crypto.database_path`        | `CRYPTO_DATABASE_PATH`     | No       |

The Anthropic SDK reads its API key from the `ANTHROPIC_API_KEY` env var, which is set programmatically from the config in `config.LoadConfig()`.

### E2EE (End-to-End Encryption)

E2EE is opt-in. Set `crypto.pickle_key` to enable it. When set, the bot uses mautrix-go's `cryptohelper` package with a pure-Go SQLite backend (`modernc.org/sqlite`) to handle Olm/Megolm session management transparently. The crypto state is stored in a SQLite database at `crypto.database_path` (default: `matrix-claude-bot.db`). Without a pickle key, the bot works in unencrypted rooms only, exactly as before.

## Architecture

The codebase follows `cmd/` + `internal/` layout with focused packages:

```
cmd/claude-bot/main.go    -- Entrypoint: flags, viper init, wiring, sync loop
internal/
  config/config.go        -- Config and MCPServerConfig structs, LoadConfig()
  bot/bot.go              -- Bot struct, NewBot(), RegisterHandlers(), message handling
  bot/claude.go           -- ConversationStore, getClaudeResponse, tool capabilities prompt
  bot/interfaces.go       -- MatrixClient and ClaudeMessenger interfaces, NewClaudeAdapter()
  crypto/crypto.go        -- E2EE Setup() via mautrix cryptohelper
  tools/tools.go          -- Tool interface and Registry for managing tools
  tools/filesystem.go     -- Sandboxed filesystem tools (fs_read, fs_write, fs_list)
  tools/mcp.go            -- MCPManager for connecting to external MCP servers
```

Dependency graph (no cycles): `config -> (external only)`, `tools -> config`, `crypto -> config`, `bot -> config + tools`, `main -> all`.

## Tool Use

The bot supports three categories of tools:

1. **Web search** -- Anthropic's server-side web search (no local execution needed). Enable with `tools.web_search_enabled: true`.
2. **Filesystem** -- Read/write/list files in a sandboxed directory. Enable with `tools.sandbox_dir: /path/to/dir`.
3. **MCP servers** -- Connect to external MCP servers via stdio, SSE, or streamable HTTP transports. Configure in `tools.mcp_servers`.

Server-side tools (web search) produce `server_tool_use` / `web_search_tool_result` blocks handled by the Anthropic API. Local tools (filesystem, MCP) produce `tool_use` blocks executed by the bot and sent back as `tool_result`.

## Key Dependencies

- `maunium.net/go/mautrix` -- Matrix client SDK (mautrix-go), including `crypto/cryptohelper` for E2EE
- `github.com/anthropics/anthropic-sdk-go` -- Anthropic Claude SDK
- `github.com/spf13/viper` -- Configuration management
- `github.com/modelcontextprotocol/go-sdk` -- MCP client SDK
- `modernc.org/sqlite` -- Pure-Go SQLite driver (no CGO required)
