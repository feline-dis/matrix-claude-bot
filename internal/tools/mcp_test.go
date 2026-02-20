package tools

import (
	"testing"

	"github.com/feline-dis/matrix-claude-bot/internal/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMcpSchemaToAnthropicSchema_Nil(t *testing.T) {
	result := mcpSchemaToAnthropicSchema(nil)
	if result.Properties == nil {
		t.Error("expected non-nil Properties for nil schema")
	}
}

func TestMcpSchemaToAnthropicSchema_WithProperties(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{
				"type":        "string",
				"description": "The name",
			},
		},
		"required": []any{"name"},
	}

	result := mcpSchemaToAnthropicSchema(schema)

	props, ok := result.Properties.(map[string]any)
	if !ok {
		t.Fatal("Properties should be map[string]any")
	}
	if _, ok := props["name"]; !ok {
		t.Error("expected 'name' in properties")
	}
	if len(result.Required) != 1 || result.Required[0] != "name" {
		t.Errorf("expected required=[name], got %v", result.Required)
	}
}

func TestMcpSchemaToAnthropicSchema_InvalidType(t *testing.T) {
	result := mcpSchemaToAnthropicSchema("not a map")
	if result.Properties == nil {
		t.Error("expected non-nil Properties for invalid schema type")
	}
}

func TestMcpResultToText_Nil(t *testing.T) {
	result := mcpResultToText(nil)
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestMcpResultToText_TextContent(t *testing.T) {
	result := mcpResultToText(&mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: "hello"},
			&mcp.TextContent{Text: "world"},
		},
	})
	if result != "hello\nworld" {
		t.Errorf("expected 'hello\\nworld', got %q", result)
	}
}

func TestMcpResultToText_EmptyContent(t *testing.T) {
	result := mcpResultToText(&mcp.CallToolResult{})
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestMcpTool_Name(t *testing.T) {
	tool := &mcpTool{
		serverName: "myserver",
		toolName:   "mytool",
	}
	if tool.Name() != "myserver_mytool" {
		t.Errorf("expected 'myserver_mytool', got %q", tool.Name())
	}
}

func TestMcpTool_Definition(t *testing.T) {
	tool := &mcpTool{
		serverName:  "srv",
		toolName:    "test",
		description: "A test tool",
		inputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	}
	def := tool.Definition()
	if def.OfTool == nil {
		t.Fatal("expected OfTool to be non-nil")
	}
	if def.OfTool.Name != "srv_test" {
		t.Errorf("expected name 'srv_test', got %q", def.OfTool.Name)
	}
}

func TestCreateTransport_Stdio(t *testing.T) {
	cfg := config.MCPServerConfig{
		Name:      "test",
		Command:   "/usr/bin/echo",
		Args:      []string{"hello"},
		Transport: "stdio",
	}
	transport, err := createTransport(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if transport == nil {
		t.Error("expected non-nil transport")
	}
}

func TestCreateTransport_StdioMissingCommand(t *testing.T) {
	cfg := config.MCPServerConfig{
		Name:      "test",
		Transport: "stdio",
	}
	_, err := createTransport(cfg)
	if err == nil {
		t.Error("expected error for missing command")
	}
}

func TestCreateTransport_SSE(t *testing.T) {
	cfg := config.MCPServerConfig{
		Name:      "test",
		URL:       "http://localhost:8080/sse",
		Transport: "sse",
	}
	transport, err := createTransport(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if transport == nil {
		t.Error("expected non-nil transport")
	}
}

func TestCreateTransport_SSEMissingURL(t *testing.T) {
	cfg := config.MCPServerConfig{
		Name:      "test",
		Transport: "sse",
	}
	_, err := createTransport(cfg)
	if err == nil {
		t.Error("expected error for missing URL")
	}
}

func TestCreateTransport_Streamable(t *testing.T) {
	cfg := config.MCPServerConfig{
		Name:      "test",
		URL:       "http://localhost:8080/mcp",
		Transport: "streamable",
	}
	transport, err := createTransport(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if transport == nil {
		t.Error("expected non-nil transport")
	}
}

func TestCreateTransport_Unknown(t *testing.T) {
	cfg := config.MCPServerConfig{
		Name:      "test",
		Transport: "grpc",
	}
	_, err := createTransport(cfg)
	if err == nil {
		t.Error("expected error for unknown transport")
	}
}
