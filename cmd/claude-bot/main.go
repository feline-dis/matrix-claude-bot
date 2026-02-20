package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/spf13/viper"
	"maunium.net/go/mautrix"

	"github.com/feline-dis/matrix-claude-bot/internal/bot"
	"github.com/feline-dis/matrix-claude-bot/internal/config"
	"github.com/feline-dis/matrix-claude-bot/internal/crypto"
	"github.com/feline-dis/matrix-claude-bot/internal/tools"
)

var configFile string

func init() {
	flag.StringVar(&configFile, "config", "", "path to config file")
}

func initConfig() {
	flag.Parse()

	if configFile != "" {
		viper.SetConfigFile(configFile)
	} else {
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
		viper.AddConfigPath("$XDG_CONFIG_HOME/matrix-claude-bot/")
		viper.AddConfigPath("$HOME/.config/matrix-claude-bot/")
	}

	viper.BindEnv("matrix.homeserver_url", "MATRIX_HOMESERVER_URL")
	viper.BindEnv("matrix.user_id", "MATRIX_USER_ID")
	viper.BindEnv("matrix.access_token", "MATRIX_ACCESS_TOKEN")
	viper.BindEnv("anthropic.api_key", "ANTHROPIC_API_KEY")
	viper.BindEnv("claude.model", "CLAUDE_MODEL")
	viper.BindEnv("claude.max_tokens", "CLAUDE_MAX_TOKENS")
	viper.BindEnv("claude.system_prompt", "CLAUDE_SYSTEM_PROMPT")
	viper.BindEnv("tools.web_search_enabled", "TOOLS_WEB_SEARCH_ENABLED")
	viper.BindEnv("tools.sandbox_dir", "TOOLS_SANDBOX_DIR")
	viper.BindEnv("tools.max_iterations", "TOOLS_MAX_ITERATIONS")
	viper.BindEnv("tools.timeout_seconds", "TOOLS_TIMEOUT_SECONDS")

	viper.BindEnv("crypto.pickle_key", "CRYPTO_PICKLE_KEY")
	viper.BindEnv("crypto.database_path", "CRYPTO_DATABASE_PATH")

	viper.SetDefault("claude.model", "claude-sonnet-4-20250514")
	viper.SetDefault("claude.max_tokens", 4096)
	viper.SetDefault("tools.max_iterations", 10)
	viper.SetDefault("tools.timeout_seconds", 30)
	viper.SetDefault("crypto.database_path", "matrix-claude-bot.db")

	if err := viper.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			log.Fatalf("Failed to parse config file: %v", err)
		}
	}
}

func main() {
	initConfig()
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatal(err)
	}

	matrixClient, err := mautrix.NewClient(cfg.HomeserverURL, cfg.UserID, cfg.AccessToken)
	if err != nil {
		log.Fatalf("Failed to create Matrix client: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if cfg.PickleKey != "" {
		whoami, err := matrixClient.Whoami(ctx)
		if err != nil {
			log.Fatalf("Failed to identify device: %v", err)
		}
		matrixClient.DeviceID = whoami.DeviceID

		cryptoHelper, err := crypto.Setup(ctx, matrixClient, cfg)
		if err != nil {
			log.Fatalf("Failed to setup E2EE: %v", err)
		}
		defer cryptoHelper.Close()
	}

	reg := tools.NewRegistry()

	if cfg.WebSearchEnabled {
		reg.AddServerTool(anthropic.ToolUnionParam{
			OfWebSearchTool20250305: &anthropic.WebSearchTool20250305Param{},
		})
		log.Println("Web search tool enabled")
	}

	if cfg.SandboxDir != "" {
		if err := os.MkdirAll(cfg.SandboxDir, 0o755); err != nil {
			log.Fatalf("Failed to create sandbox directory %s: %v", cfg.SandboxDir, err)
		}
		for _, t := range tools.NewFilesystemTools(cfg.SandboxDir) {
			reg.Register(t)
		}
		log.Printf("Filesystem tools enabled (sandbox: %s)", cfg.SandboxDir)
	}

	var mcpManager *tools.MCPManager
	if len(cfg.MCPServers) > 0 {
		mcpManager = tools.NewMCPManager()
		connectCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		if err := mcpManager.Connect(connectCtx, cfg.MCPServers, reg); err != nil {
			log.Printf("Warning: MCP connection error: %v", err)
		}
		cancel()
	}

	b := bot.NewBot(matrixClient, bot.NewClaudeAdapter(), cfg, reg)
	bot.RegisterHandlers(matrixClient, b)

	log.Printf("Bot started as %s", cfg.UserID)

	err = matrixClient.SyncWithContext(ctx)
	if err != nil && ctx.Err() == nil {
		log.Fatalf("Sync failed: %v", err)
	}

	if mcpManager != nil {
		mcpManager.Close()
	}

	log.Println("Bot stopped")
}
