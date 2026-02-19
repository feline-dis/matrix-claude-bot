//go:build integration

package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/spf13/viper"
	"maunium.net/go/mautrix"
)

func loadIntegrationConfig(t *testing.T) Config {
	t.Helper()
	viper.Reset()
	viper.SetConfigFile("config.test.yaml")
	if err := viper.ReadInConfig(); err != nil {
		t.Fatalf("Failed to read config.test.yaml: %v", err)
	}
	viper.SetDefault("claude.model", "claude-sonnet-4-20250514")
	viper.SetDefault("claude.max_tokens", 4096)

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Ensure homeserver URL has a scheme
	if !strings.HasPrefix(cfg.HomeserverURL, "http://") && !strings.HasPrefix(cfg.HomeserverURL, "https://") {
		cfg.HomeserverURL = "https://" + cfg.HomeserverURL
	}

	return cfg
}

func TestIntegration_MatrixConnect(t *testing.T) {
	cfg := loadIntegrationConfig(t)

	client, err := mautrix.NewClient(cfg.HomeserverURL, cfg.UserID, cfg.AccessToken)
	if err != nil {
		t.Fatalf("Failed to create Matrix client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// A short sync verifies credentials and connectivity
	err = client.SyncWithContext(ctx)
	if err != nil && ctx.Err() == nil {
		t.Fatalf("Sync failed (not timeout): %v", err)
	}
}

func TestIntegration_ClaudeAPI(t *testing.T) {
	cfg := loadIntegrationConfig(t)
	_ = cfg // config loaded to set ANTHROPIC_API_KEY env var

	client := anthropic.NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.ModelClaudeSonnet4_20250514,
		MaxTokens: 64,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock("Say hello in exactly one word.")),
		},
	})
	if err != nil {
		t.Fatalf("Claude API call failed: %v", err)
	}

	if len(resp.Content) == 0 {
		t.Fatal("Expected non-empty response from Claude")
	}

	hasText := false
	for _, block := range resp.Content {
		if block.Type == "text" && block.Text != "" {
			hasText = true
			break
		}
	}
	if !hasText {
		t.Fatal("Expected at least one non-empty text block in response")
	}
}

func TestIntegration_FullFlow(t *testing.T) {
	cfg := loadIntegrationConfig(t)

	matrixClient, err := mautrix.NewClient(cfg.HomeserverURL, cfg.UserID, cfg.AccessToken)
	if err != nil {
		t.Fatalf("Failed to create Matrix client: %v", err)
	}

	claudeClient := &claudeAdapter{client: anthropic.NewClient()}

	bot := &Bot{
		matrix:        matrixClient,
		claude:        claudeClient,
		config:        cfg,
		conversations: NewConversationStore(),
		tools:         NewToolRegistry(),
		startTime:     time.Now(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := bot.getClaudeResponse(ctx, "$integration-test", "Say hello in exactly one word.")
	if err != nil {
		t.Fatalf("getClaudeResponse failed: %v", err)
	}
	if resp == "" {
		t.Fatal("Expected non-empty response")
	}

	msgs := bot.conversations.Get("$integration-test")
	if len(msgs) != 2 {
		t.Fatalf("Expected 2 messages in conversation (user + assistant), got %d", len(msgs))
	}
}
