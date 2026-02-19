package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/spf13/viper"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/id"
)

type Config struct {
	HomeserverURL      string
	UserID             id.UserID
	AccessToken        string
	Model              string
	MaxTokens          int64
	SystemPrompt       string
	PickleKey          string
	CryptoDatabasePath string
}

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

	viper.BindEnv("crypto.pickle_key", "CRYPTO_PICKLE_KEY")
	viper.BindEnv("crypto.database_path", "CRYPTO_DATABASE_PATH")

	viper.SetDefault("claude.model", "claude-sonnet-4-20250514")
	viper.SetDefault("claude.max_tokens", 4096)
	viper.SetDefault("crypto.database_path", "matrix-claude-bot.db")

	if err := viper.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			log.Fatalf("Failed to parse config file: %v", err)
		}
	}
}

func loadConfig() (Config, error) {
	homeserverURL := viper.GetString("matrix.homeserver_url")
	userID := viper.GetString("matrix.user_id")
	accessToken := viper.GetString("matrix.access_token")
	apiKey := viper.GetString("anthropic.api_key")

	if homeserverURL == "" || userID == "" || accessToken == "" || apiKey == "" {
		return Config{}, fmt.Errorf("required config: matrix.homeserver_url, matrix.user_id, matrix.access_token, anthropic.api_key")
	}

	// The Anthropic SDK reads the API key from the environment.
	os.Setenv("ANTHROPIC_API_KEY", apiKey)

	return Config{
		HomeserverURL:      homeserverURL,
		UserID:             id.UserID(userID),
		AccessToken:        accessToken,
		Model:              viper.GetString("claude.model"),
		MaxTokens:          viper.GetInt64("claude.max_tokens"),
		SystemPrompt:       viper.GetString("claude.system_prompt"),
		PickleKey:          viper.GetString("crypto.pickle_key"),
		CryptoDatabasePath: viper.GetString("crypto.database_path"),
	}, nil
}

func main() {
	initConfig()
	cfg, err := loadConfig()
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

		cryptoHelper, err := setupCrypto(ctx, matrixClient, cfg)
		if err != nil {
			log.Fatalf("Failed to setup E2EE: %v", err)
		}
		defer cryptoHelper.Close()
	}

	bot := &Bot{
		matrix:        matrixClient,
		claude:        &claudeAdapter{client: anthropic.NewClient()},
		config:        cfg,
		conversations: NewConversationStore(),
		startTime:     time.Now(),
	}

	RegisterHandlers(matrixClient, bot)

	log.Printf("Bot started as %s", cfg.UserID)

	err = matrixClient.SyncWithContext(ctx)
	if err != nil && ctx.Err() == nil {
		log.Fatalf("Sync failed: %v", err)
	}

	log.Println("Bot stopped")
}
