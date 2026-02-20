package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
)

type fakeTool struct {
	name   string
	result string
}

func (t *fakeTool) Name() string { return t.name }
func (t *fakeTool) Definition() anthropic.ToolUnionParam {
	return anthropic.ToolUnionParam{
		OfTool: &anthropic.ToolParam{
			Name: t.name,
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: map[string]any{},
			},
		},
	}
}
func (t *fakeTool) Execute(ctx context.Context, input json.RawMessage) (string, bool, error) {
	return t.result, false, nil
}

func TestRegistry_IsEmpty(t *testing.T) {
	reg := NewRegistry()
	if !reg.IsEmpty() {
		t.Error("new registry should be empty")
	}

	reg.Register(&fakeTool{name: "test", result: "ok"})
	if reg.IsEmpty() {
		t.Error("registry with tool should not be empty")
	}
}

func TestRegistry_IsEmpty_ServerToolOnly(t *testing.T) {
	reg := NewRegistry()
	reg.AddServerTool(anthropic.ToolUnionParam{
		OfWebSearchTool20250305: &anthropic.WebSearchTool20250305Param{},
	})
	if reg.IsEmpty() {
		t.Error("registry with server tool should not be empty")
	}
}

func TestRegistry_RegisterAndExecute(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&fakeTool{name: "echo", result: "hello"})

	if !reg.HasLocalTool("echo") {
		t.Error("expected HasLocalTool to return true for 'echo'")
	}
	if reg.HasLocalTool("missing") {
		t.Error("expected HasLocalTool to return false for 'missing'")
	}

	result, isErr, err := reg.Execute(context.Background(), "echo", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if isErr {
		t.Error("expected isError=false")
	}
	if result != "hello" {
		t.Errorf("expected 'hello', got %q", result)
	}
}

func TestRegistry_ExecuteUnknownTool(t *testing.T) {
	reg := NewRegistry()
	_, _, err := reg.Execute(context.Background(), "missing", json.RawMessage(`{}`))
	if err == nil {
		t.Error("expected error for unknown tool")
	}
}

func TestRegistry_LocalToolNames(t *testing.T) {
	reg := NewRegistry()
	if names := reg.LocalToolNames(); len(names) != 0 {
		t.Errorf("expected empty names, got %v", names)
	}

	reg.Register(&fakeTool{name: "zebra", result: "ok"})
	reg.Register(&fakeTool{name: "alpha", result: "ok"})
	reg.Register(&fakeTool{name: "middle", result: "ok"})

	names := reg.LocalToolNames()
	if len(names) != 3 {
		t.Fatalf("expected 3 names, got %d", len(names))
	}
	if names[0] != "alpha" || names[1] != "middle" || names[2] != "zebra" {
		t.Errorf("expected sorted [alpha middle zebra], got %v", names)
	}
}

func TestRegistry_HasServerTools(t *testing.T) {
	reg := NewRegistry()
	if reg.HasServerTools() {
		t.Error("new registry should not have server tools")
	}

	reg.AddServerTool(anthropic.ToolUnionParam{
		OfWebSearchTool20250305: &anthropic.WebSearchTool20250305Param{},
	})
	if !reg.HasServerTools() {
		t.Error("expected HasServerTools=true after adding server tool")
	}
}

func TestRegistry_LocalToolNamesExcludesServerTools(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&fakeTool{name: "local1", result: "ok"})
	reg.AddServerTool(anthropic.ToolUnionParam{
		OfWebSearchTool20250305: &anthropic.WebSearchTool20250305Param{},
	})

	names := reg.LocalToolNames()
	if len(names) != 1 || names[0] != "local1" {
		t.Errorf("expected [local1], got %v", names)
	}
}

func TestRegistry_Definitions(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&fakeTool{name: "local1", result: "ok"})
	reg.AddServerTool(anthropic.ToolUnionParam{
		OfWebSearchTool20250305: &anthropic.WebSearchTool20250305Param{},
	})

	defs := reg.Definitions()
	if len(defs) != 2 {
		t.Fatalf("expected 2 definitions, got %d", len(defs))
	}
}
