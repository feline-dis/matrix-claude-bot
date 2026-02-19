# matrix-claude-bot

A Matrix chat bot that responds to @-mentions using the Anthropic Claude API. Replies are sent in Matrix threads, and conversation history is maintained per-thread in memory.

## Prerequisites

- Go 1.25 or later
- A Matrix account for the bot with:
  - Homeserver URL
  - User ID (e.g. `@bot:example.com`)
  - Access token
- An [Anthropic API key](https://console.anthropic.com/)

## Installation

```bash
git clone https://github.com/feline-dis/matrix-claude-bot.git
cd matrix-claude-bot
go build -o matrix-claude-bot .
```

## Configuration

The bot can be configured via a YAML config file, environment variables, or both. Environment variables take precedence over the config file.

### Config file

Create a `config.yaml`:

```yaml
matrix:
  homeserver_url: "https://matrix.example.com"
  user_id: "@bot:example.com"
  access_token: "your-access-token"

anthropic:
  api_key: "sk-ant-..."

claude:
  model: "claude-sonnet-4-20250514"
  max_tokens: 4096
  system_prompt: "You are a helpful assistant."
```

The bot searches for `config.yaml` in these locations:

1. `$XDG_CONFIG_HOME/matrix-claude-bot/`
2. `$HOME/.config/matrix-claude-bot/`

Or specify a path directly:

```bash
./matrix-claude-bot -config /path/to/config.yaml
```

### Environment variables

All config values can be set or overridden with environment variables:

| Config key              | Environment variable   | Required | Default                    |
|-------------------------|------------------------|----------|----------------------------|
| `matrix.homeserver_url` | `MATRIX_HOMESERVER_URL`| Yes      |                            |
| `matrix.user_id`        | `MATRIX_USER_ID`       | Yes      |                            |
| `matrix.access_token`   | `MATRIX_ACCESS_TOKEN`  | Yes      |                            |
| `anthropic.api_key`     | `ANTHROPIC_API_KEY`    | Yes      |                            |
| `claude.model`          | `CLAUDE_MODEL`         | No       | `claude-sonnet-4-20250514` |
| `claude.max_tokens`     | `CLAUDE_MAX_TOKENS`    | No       | `4096`                     |
| `claude.system_prompt`  | `CLAUDE_SYSTEM_PROMPT` | No       |                            |

## Usage

```bash
./matrix-claude-bot
```

Or with a specific config file:

```bash
./matrix-claude-bot -config ./config.yaml
```

Or using only environment variables:

```bash
export MATRIX_HOMESERVER_URL="https://matrix.example.com"
export MATRIX_USER_ID="@bot:example.com"
export MATRIX_ACCESS_TOKEN="your-access-token"
export ANTHROPIC_API_KEY="sk-ant-..."
./matrix-claude-bot
```

### Behavior

- **Auto-join**: The bot automatically joins rooms when invited.
- **Mention-triggered**: The bot only responds when @-mentioned in a message.
- **Threaded replies**: Responses are sent as Matrix thread replies.
- **Conversation history**: The bot maintains conversation context within each thread for multi-turn conversations. History is stored in memory and lost on restart.
- **Graceful shutdown**: The bot stops cleanly on SIGINT or SIGTERM.
