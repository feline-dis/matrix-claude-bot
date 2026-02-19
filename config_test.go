package main

import (
	"os"
	"testing"

	"github.com/spf13/viper"
)

func setupConfigTest(t *testing.T) {
	t.Helper()
	viper.Reset()

	origAPIKey := os.Getenv("ANTHROPIC_API_KEY")
	t.Cleanup(func() {
		viper.Reset()
		os.Setenv("ANTHROPIC_API_KEY", origAPIKey)
	})
}

func setRequiredViperKeys() {
	viper.Set("matrix.homeserver_url", "https://matrix.example.com")
	viper.Set("matrix.user_id", "@bot:example.com")
	viper.Set("matrix.access_token", "syt_token")
	viper.Set("anthropic.api_key", "sk-ant-test")
}

func TestLoadConfig_AllFields(t *testing.T) {
	setupConfigTest(t)
	setRequiredViperKeys()
	viper.Set("claude.model", "claude-opus-4-20250514")
	viper.Set("claude.max_tokens", 2048)
	viper.Set("claude.system_prompt", "Be helpful.")

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.HomeserverURL != "https://matrix.example.com" {
		t.Errorf("wrong homeserver URL: %s", cfg.HomeserverURL)
	}
	if cfg.UserID != "@bot:example.com" {
		t.Errorf("wrong user ID: %s", cfg.UserID)
	}
	if cfg.AccessToken != "syt_token" {
		t.Errorf("wrong access token: %s", cfg.AccessToken)
	}
	if cfg.Model != "claude-opus-4-20250514" {
		t.Errorf("wrong model: %s", cfg.Model)
	}
	if cfg.MaxTokens != 2048 {
		t.Errorf("wrong max tokens: %d", cfg.MaxTokens)
	}
	if cfg.SystemPrompt != "Be helpful." {
		t.Errorf("wrong system prompt: %s", cfg.SystemPrompt)
	}
	if os.Getenv("ANTHROPIC_API_KEY") != "sk-ant-test" {
		t.Error("ANTHROPIC_API_KEY env var not set")
	}
}

func TestLoadConfig_Defaults(t *testing.T) {
	setupConfigTest(t)
	setRequiredViperKeys()
	viper.SetDefault("claude.model", "claude-sonnet-4-20250514")
	viper.SetDefault("claude.max_tokens", 4096)

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Model != "claude-sonnet-4-20250514" {
		t.Errorf("expected default model, got %s", cfg.Model)
	}
	if cfg.MaxTokens != 4096 {
		t.Errorf("expected default max tokens 4096, got %d", cfg.MaxTokens)
	}
}

func TestLoadConfig_MissingRequired(t *testing.T) {
	setupConfigTest(t)
	// Only set some required keys, omit homeserver_url
	viper.Set("matrix.user_id", "@bot:example.com")
	viper.Set("matrix.access_token", "syt_token")
	viper.Set("anthropic.api_key", "sk-ant-test")

	_, err := loadConfig()
	if err == nil {
		t.Fatal("expected error for missing homeserver_url")
	}
}

func TestLoadConfig_MissingAPIKey(t *testing.T) {
	setupConfigTest(t)
	viper.Set("matrix.homeserver_url", "https://matrix.example.com")
	viper.Set("matrix.user_id", "@bot:example.com")
	viper.Set("matrix.access_token", "syt_token")

	_, err := loadConfig()
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
}
