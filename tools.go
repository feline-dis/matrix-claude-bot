package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"

	"github.com/anthropics/anthropic-sdk-go"
)

// Tool represents a locally-executed tool that Claude can invoke.
type Tool interface {
	Name() string
	Definition() anthropic.ToolUnionParam
	Execute(ctx context.Context, input json.RawMessage) (result string, isError bool, err error)
}

// ToolRegistry holds both locally-executed tools and server-side tool
// definitions (like web search) that the Anthropic API handles.
type ToolRegistry struct {
	mu          sync.RWMutex
	localTools  map[string]Tool
	serverTools []anthropic.ToolUnionParam
}

func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		localTools: make(map[string]Tool),
	}
}

func (r *ToolRegistry) Register(t Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.localTools[t.Name()] = t
}

// AddServerTool adds a server-side tool definition (e.g. web search) that the
// Anthropic API executes. These are included in API requests but not executed locally.
func (r *ToolRegistry) AddServerTool(t anthropic.ToolUnionParam) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.serverTools = append(r.serverTools, t)
}

// Definitions returns all tool definitions for inclusion in API requests.
func (r *ToolRegistry) Definitions() []anthropic.ToolUnionParam {
	r.mu.RLock()
	defer r.mu.RUnlock()

	defs := make([]anthropic.ToolUnionParam, 0, len(r.localTools)+len(r.serverTools))
	for _, t := range r.localTools {
		defs = append(defs, t.Definition())
	}
	defs = append(defs, r.serverTools...)
	return defs
}

// Execute runs a locally-registered tool by name.
func (r *ToolRegistry) Execute(ctx context.Context, name string, input json.RawMessage) (string, bool, error) {
	r.mu.RLock()
	t, ok := r.localTools[name]
	r.mu.RUnlock()

	if !ok {
		return "", false, fmt.Errorf("unknown tool: %s", name)
	}
	return t.Execute(ctx, input)
}

func (r *ToolRegistry) HasLocalTool(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.localTools[name]
	return ok
}

func (r *ToolRegistry) IsEmpty() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.localTools) == 0 && len(r.serverTools) == 0
}

// LocalToolNames returns a sorted list of all registered local tool names.
func (r *ToolRegistry) LocalToolNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.localTools))
	for name := range r.localTools {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// HasServerTools reports whether any server-side tools are registered.
func (r *ToolRegistry) HasServerTools() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.serverTools) > 0
}
