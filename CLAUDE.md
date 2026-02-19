# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

A Matrix chat bot that responds to @-mentions using the Anthropic Claude API. When mentioned in a room, it sends the message to Claude and replies in a Matrix thread, maintaining per-thread conversation history in memory.

## Build and Run

```bash
go build -o matrix-claude-bot .
./matrix-claude-bot -config path/to/config.yaml
```

No tests, linter, or CI exist yet.

## Configuration

Config is loaded via Viper from `config.yaml` (searched in `$XDG_CONFIG_HOME/matrix-claude-bot/` or `$HOME/.config/matrix-claude-bot/`) or specified with `-config`. All values can be overridden with environment variables:

| Config key                | Env var                | Required |
|---------------------------|------------------------|----------|
| `matrix.homeserver_url`   | `MATRIX_HOMESERVER_URL`| Yes      |
| `matrix.user_id`          | `MATRIX_USER_ID`       | Yes      |
| `matrix.access_token`     | `MATRIX_ACCESS_TOKEN`  | Yes      |
| `anthropic.api_key`       | `ANTHROPIC_API_KEY`    | Yes      |
| `claude.model`            | `CLAUDE_MODEL`         | No       |
| `claude.max_tokens`       | `CLAUDE_MAX_TOKENS`    | No       |
| `claude.system_prompt`    | `CLAUDE_SYSTEM_PROMPT` | No       |

The Anthropic SDK reads its API key from the `ANTHROPIC_API_KEY` env var, which is set programmatically from the config in `main.go:loadConfig`.

## Architecture

All code is in package `main` across three files:

- **main.go** -- Entrypoint. Defines `Config` struct, loads config via Viper, creates Matrix + Claude clients, wires up the `Bot`, and runs the Matrix sync loop with graceful shutdown on SIGINT/SIGTERM.
- **bot.go** -- Matrix event handling. `Bot` struct holds both clients, config, conversation store, and start time. Registers handlers for message events (responds to mentions) and member events (auto-joins on invite). Messages are dispatched to goroutines. Replies are sent as Matrix threads.
- **claude.go** -- Claude API interaction and conversation state. `ConversationStore` is a thread-safe in-memory map keyed by Matrix thread root event ID. `getClaudeResponse` appends the user message, calls the Claude API with full thread history, and appends the assistant response.

## Key Dependencies

- `maunium.net/go/mautrix` -- Matrix client SDK (mautrix-go)
- `github.com/anthropics/anthropic-sdk-go` -- Anthropic Claude SDK
- `github.com/spf13/viper` -- Configuration management
