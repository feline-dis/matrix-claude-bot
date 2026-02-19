package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MCPServerConfig describes how to connect to an MCP server.
type MCPServerConfig struct {
	Name      string            `mapstructure:"name"`
	Command   string            `mapstructure:"command"`
	Args      []string          `mapstructure:"args"`
	Env       map[string]string `mapstructure:"env"`
	URL       string            `mapstructure:"url"`
	Transport string            `mapstructure:"transport"` // "stdio", "sse", or "streamable"
}

type mcpConnection struct {
	name    string
	session *mcp.ClientSession
}

// MCPManager manages connections to MCP servers.
type MCPManager struct {
	connections []*mcpConnection
}

func NewMCPManager() *MCPManager {
	return &MCPManager{}
}

// Connect establishes connections to the configured MCP servers, discovers
// their tools, and registers them in the given ToolRegistry.
func (m *MCPManager) Connect(ctx context.Context, servers []MCPServerConfig, registry *ToolRegistry) error {
	var errs []string

	for _, serverCfg := range servers {
		transport, err := createTransport(serverCfg)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", serverCfg.Name, err))
			continue
		}

		client := mcp.NewClient(&mcp.Implementation{
			Name:    "matrix-claude-bot",
			Version: "1.0.0",
		}, nil)

		session, err := client.Connect(ctx, transport, nil)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: connection failed: %v", serverCfg.Name, err))
			continue
		}

		conn := &mcpConnection{
			name:    serverCfg.Name,
			session: session,
		}
		m.connections = append(m.connections, conn)

		toolCount := 0
		for tool, err := range session.Tools(ctx, nil) {
			if err != nil {
				errs = append(errs, fmt.Sprintf("%s: tool listing failed: %v", serverCfg.Name, err))
				break
			}

			wrapped := &mcpTool{
				serverName:  serverCfg.Name,
				toolName:    tool.Name,
				description: tool.Description,
				inputSchema: tool.InputSchema,
				session:     session,
			}
			registry.Register(wrapped)
			toolCount++
		}

		log.Printf("MCP server %q connected: %d tools", serverCfg.Name, toolCount)
	}

	if len(errs) > 0 {
		return fmt.Errorf("MCP connection errors: %s", strings.Join(errs, "; "))
	}
	return nil
}

// Close shuts down all MCP sessions.
func (m *MCPManager) Close() {
	for _, conn := range m.connections {
		if err := conn.session.Close(); err != nil {
			log.Printf("Error closing MCP session %q: %v", conn.name, err)
		}
	}
}

func createTransport(cfg MCPServerConfig) (mcp.Transport, error) {
	switch cfg.Transport {
	case "stdio", "":
		if cfg.Command == "" {
			return nil, fmt.Errorf("stdio transport requires 'command'")
		}
		cmd := exec.Command(cfg.Command, cfg.Args...)
		for k, v := range cfg.Env {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
		return &mcp.CommandTransport{Command: cmd}, nil

	case "sse":
		if cfg.URL == "" {
			return nil, fmt.Errorf("sse transport requires 'url'")
		}
		return &mcp.SSEClientTransport{Endpoint: cfg.URL}, nil

	case "streamable":
		if cfg.URL == "" {
			return nil, fmt.Errorf("streamable transport requires 'url'")
		}
		return &mcp.StreamableClientTransport{Endpoint: cfg.URL}, nil

	default:
		return nil, fmt.Errorf("unknown transport type: %q", cfg.Transport)
	}
}

// mcpTool wraps a single MCP server tool as a local Tool.
type mcpTool struct {
	serverName  string
	toolName    string
	description string
	inputSchema any
	session     *mcp.ClientSession
}

func (t *mcpTool) Name() string {
	return t.serverName + "_" + t.toolName
}

func (t *mcpTool) Definition() anthropic.ToolUnionParam {
	return anthropic.ToolUnionParam{
		OfTool: &anthropic.ToolParam{
			Name:        t.Name(),
			Description: anthropic.String(t.description),
			InputSchema: mcpSchemaToAnthropicSchema(t.inputSchema),
		},
	}
}

func (t *mcpTool) Execute(ctx context.Context, input json.RawMessage) (string, bool, error) {
	var args map[string]any
	if len(input) > 0 {
		if err := json.Unmarshal(input, &args); err != nil {
			return "invalid tool input: " + err.Error(), true, nil
		}
	}

	result, err := t.session.CallTool(ctx, &mcp.CallToolParams{
		Name:      t.toolName,
		Arguments: args,
	})
	if err != nil {
		return "", false, fmt.Errorf("MCP tool call failed: %w", err)
	}

	text := mcpResultToText(result)
	return text, result.IsError, nil
}

// mcpSchemaToAnthropicSchema converts an MCP tool's InputSchema to the
// Anthropic ToolInputSchemaParam format.
func mcpSchemaToAnthropicSchema(schema any) anthropic.ToolInputSchemaParam {
	if schema == nil {
		return anthropic.ToolInputSchemaParam{
			Properties: map[string]any{},
		}
	}

	// The MCP SDK returns InputSchema as a map[string]any from JSON unmarshaling.
	m, ok := schema.(map[string]any)
	if !ok {
		return anthropic.ToolInputSchemaParam{
			Properties: map[string]any{},
		}
	}

	result := anthropic.ToolInputSchemaParam{}

	if props, ok := m["properties"]; ok {
		result.Properties = props
	} else {
		result.Properties = map[string]any{}
	}

	if req, ok := m["required"].([]any); ok {
		for _, r := range req {
			if s, ok := r.(string); ok {
				result.Required = append(result.Required, s)
			}
		}
	}

	return result
}

// mcpResultToText extracts text from an MCP CallToolResult.
func mcpResultToText(result *mcp.CallToolResult) string {
	if result == nil {
		return ""
	}

	var parts []string
	for _, content := range result.Content {
		switch c := content.(type) {
		case *mcp.TextContent:
			parts = append(parts, c.Text)
		default:
			// For non-text content, try to marshal it as JSON.
			data, err := json.Marshal(content)
			if err == nil {
				parts = append(parts, string(data))
			}
		}
	}

	return strings.Join(parts, "\n")
}
