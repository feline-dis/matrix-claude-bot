# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

A Matrix chat bot that responds to @-mentions using the Anthropic Claude API. When mentioned in a room, it sends the message to Claude and replies in a Matrix thread, maintaining per-thread conversation history in memory.

## Build and Run

```bash
go build -tags goolm -o matrix-claude-bot .
./matrix-claude-bot -config path/to/config.yaml
```

The `goolm` build tag selects the pure-Go Olm implementation (no CGO/libolm required).

Run tests with `go test -tags goolm ./...`. Integration tests (tagged `integration`) require a `config.test.yaml`.

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
| `crypto.pickle_key`      | `CRYPTO_PICKLE_KEY`    | No       |
| `crypto.database_path`   | `CRYPTO_DATABASE_PATH` | No       |

The Anthropic SDK reads its API key from the `ANTHROPIC_API_KEY` env var, which is set programmatically from the config in `main.go:loadConfig`.

### E2EE (End-to-End Encryption)

E2EE is opt-in. Set `crypto.pickle_key` to enable it. When set, the bot uses mautrix-go's `cryptohelper` package with a pure-Go SQLite backend (`modernc.org/sqlite`) to handle Olm/Megolm session management transparently. The crypto state is stored in a SQLite database at `crypto.database_path` (default: `matrix-claude-bot.db`). Without a pickle key, the bot works in unencrypted rooms only, exactly as before.

## Architecture

All code is in package `main` across four files:

- **main.go** -- Entrypoint. Defines `Config` struct, loads config via Viper, creates Matrix + Claude clients, optionally initializes E2EE, wires up the `Bot`, and runs the Matrix sync loop with graceful shutdown on SIGINT/SIGTERM.
- **bot.go** -- Matrix event handling. `Bot` struct holds both clients, config, conversation store, and start time. Registers handlers for message events (responds to mentions) and member events (auto-joins on invite). Messages are dispatched to goroutines. Replies are sent as Matrix threads.
- **claude.go** -- Claude API interaction and conversation state. `ConversationStore` is a thread-safe in-memory map keyed by Matrix thread root event ID. `getClaudeResponse` appends the user message, calls the Claude API with full thread history, and appends the assistant response.
- **crypto.go** -- E2EE setup. `setupCrypto` creates a `cryptohelper.CryptoHelper` with a pure-Go SQLite database and attaches it to the Matrix client. `openCryptoDatabase` opens the SQLite DB with appropriate pragmas.

## Key Dependencies

- `maunium.net/go/mautrix` -- Matrix client SDK (mautrix-go), including `crypto/cryptohelper` for E2EE
- `github.com/anthropics/anthropic-sdk-go` -- Anthropic Claude SDK
- `github.com/spf13/viper` -- Configuration management
- `modernc.org/sqlite` -- Pure-Go SQLite driver (no CGO required)
