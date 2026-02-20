package config

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/viper"
	"maunium.net/go/mautrix/id"
)

type Config struct {
	HomeserverURL      string
	UserID             id.UserID
	AccessToken        string
	Model              string
	MaxTokens          int64
	SystemPrompt       string
	WebSearchEnabled   bool
	SandboxDir         string
	MaxToolIterations  int
	ToolTimeout        time.Duration
	MCPServers         []MCPServerConfig
	PickleKey          string
	CryptoDatabasePath string
}

type MCPServerConfig struct {
	Name      string            `mapstructure:"name"`
	Command   string            `mapstructure:"command"`
	Args      []string          `mapstructure:"args"`
	Env       map[string]string `mapstructure:"env"`
	URL       string            `mapstructure:"url"`
	Transport string            `mapstructure:"transport"` // "stdio", "sse", or "streamable"
}

// LoadConfig reads configuration from viper and returns a validated Config.
// Viper must be initialized (env bindings, defaults, config file) before calling.
func LoadConfig() (Config, error) {
	homeserverURL := viper.GetString("matrix.homeserver_url")
	userID := viper.GetString("matrix.user_id")
	accessToken := viper.GetString("matrix.access_token")
	apiKey := viper.GetString("anthropic.api_key")

	if homeserverURL == "" || userID == "" || accessToken == "" || apiKey == "" {
		return Config{}, fmt.Errorf("required config: matrix.homeserver_url, matrix.user_id, matrix.access_token, anthropic.api_key")
	}

	// The Anthropic SDK reads the API key from the environment.
	os.Setenv("ANTHROPIC_API_KEY", apiKey)

	timeoutSec := viper.GetInt("tools.timeout_seconds")

	var mcpServers []MCPServerConfig
	viper.UnmarshalKey("tools.mcp_servers", &mcpServers)

	return Config{
		HomeserverURL:      homeserverURL,
		UserID:             id.UserID(userID),
		AccessToken:        accessToken,
		Model:              viper.GetString("claude.model"),
		MaxTokens:          viper.GetInt64("claude.max_tokens"),
		SystemPrompt:       viper.GetString("claude.system_prompt"),
		WebSearchEnabled:   viper.GetBool("tools.web_search_enabled"),
		SandboxDir:         viper.GetString("tools.sandbox_dir"),
		MaxToolIterations:  viper.GetInt("tools.max_iterations"),
		ToolTimeout:        time.Duration(timeoutSec) * time.Second,
		MCPServers:         mcpServers,
		PickleKey:          viper.GetString("crypto.pickle_key"),
		CryptoDatabasePath: viper.GetString("crypto.database_path"),
	}, nil
}
