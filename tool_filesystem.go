package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
)

const (
	maxFileReadSize  = 1 << 20 // 1 MB
	maxListEntries   = 200
)

// resolveSandboxedPath resolves the given path within sandboxDir, following
// symlinks, and returns an error if the resolved path escapes the sandbox.
func resolveSandboxedPath(sandboxDir, path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path is empty")
	}

	absSandbox, err := filepath.Abs(sandboxDir)
	if err != nil {
		return "", fmt.Errorf("invalid sandbox dir: %w", err)
	}

	// Clean the path first and check that it doesn't escape via ".." before
	// anything touches the filesystem.
	joined := filepath.Join(absSandbox, filepath.Clean(path))
	if !isWithin(joined, absSandbox) {
		return "", fmt.Errorf("path escapes sandbox")
	}

	// Try to resolve symlinks. If the file exists, verify the resolved path
	// is still within the sandbox (symlink could point outside).
	resolved, err := filepath.EvalSymlinks(joined)
	if err == nil {
		resolvedSandbox, _ := filepath.EvalSymlinks(absSandbox)
		if !isWithin(resolved, resolvedSandbox) {
			return "", fmt.Errorf("path escapes sandbox")
		}
		return resolved, nil
	}

	// File doesn't exist yet (valid for writes). Walk up to the nearest
	// existing ancestor and verify it's within the sandbox.
	ancestor := joined
	for {
		parent := filepath.Dir(ancestor)
		if parent == ancestor {
			break
		}
		ancestor = parent
		resolvedAncestor, aerr := filepath.EvalSymlinks(ancestor)
		if aerr == nil {
			resolvedSandbox, _ := filepath.EvalSymlinks(absSandbox)
			if !isWithin(resolvedAncestor, resolvedSandbox) {
				return "", fmt.Errorf("path escapes sandbox")
			}
			return joined, nil
		}
	}

	return joined, nil
}

func isWithin(path, dir string) bool {
	return path == dir || strings.HasPrefix(path, dir+string(os.PathSeparator))
}

// NewFilesystemTools returns the fs_read, fs_write, and fs_list tools
// operating within the given sandbox directory.
func NewFilesystemTools(sandboxDir string) []Tool {
	return []Tool{
		&fsReadTool{sandboxDir: sandboxDir},
		&fsWriteTool{sandboxDir: sandboxDir},
		&fsListTool{sandboxDir: sandboxDir},
	}
}

// --- fs_read ---

type fsReadTool struct{ sandboxDir string }

type fsReadInput struct {
	Path string `json:"path"`
}

func (t *fsReadTool) Name() string { return "fs_read" }

func (t *fsReadTool) Definition() anthropic.ToolUnionParam {
	return anthropic.ToolUnionParam{
		OfTool: &anthropic.ToolParam{
			Name:        "fs_read",
			Description: anthropic.String("Read a file from the sandbox directory. Returns file contents as text. Max 1MB."),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "Relative path within the sandbox directory",
					},
				},
				Required: []string{"path"},
			},
		},
	}
}

func (t *fsReadTool) Execute(ctx context.Context, input json.RawMessage) (string, bool, error) {
	var params fsReadInput
	if err := json.Unmarshal(input, &params); err != nil {
		return "invalid input: " + err.Error(), true, nil
	}

	resolved, err := resolveSandboxedPath(t.sandboxDir, params.Path)
	if err != nil {
		return err.Error(), true, nil
	}

	info, err := os.Stat(resolved)
	if err != nil {
		return "file not found: " + params.Path, true, nil
	}
	if info.IsDir() {
		return "path is a directory, use fs_list instead", true, nil
	}
	if info.Size() > maxFileReadSize {
		return fmt.Sprintf("file too large: %d bytes (max %d)", info.Size(), maxFileReadSize), true, nil
	}

	data, err := os.ReadFile(resolved)
	if err != nil {
		return "failed to read file: " + err.Error(), true, nil
	}

	return string(data), false, nil
}

// --- fs_write ---

type fsWriteTool struct{ sandboxDir string }

type fsWriteInput struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

func (t *fsWriteTool) Name() string { return "fs_write" }

func (t *fsWriteTool) Definition() anthropic.ToolUnionParam {
	return anthropic.ToolUnionParam{
		OfTool: &anthropic.ToolParam{
			Name:        "fs_write",
			Description: anthropic.String("Write content to a file in the sandbox directory. Creates parent directories as needed."),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "Relative path within the sandbox directory",
					},
					"content": map[string]any{
						"type":        "string",
						"description": "Content to write to the file",
					},
				},
				Required: []string{"path", "content"},
			},
		},
	}
}

func (t *fsWriteTool) Execute(ctx context.Context, input json.RawMessage) (string, bool, error) {
	var params fsWriteInput
	if err := json.Unmarshal(input, &params); err != nil {
		return "invalid input: " + err.Error(), true, nil
	}

	resolved, err := resolveSandboxedPath(t.sandboxDir, params.Path)
	if err != nil {
		return err.Error(), true, nil
	}

	dir := filepath.Dir(resolved)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "failed to create directories: " + err.Error(), true, nil
	}

	if err := os.WriteFile(resolved, []byte(params.Content), 0o644); err != nil {
		return "failed to write file: " + err.Error(), true, nil
	}

	return fmt.Sprintf("wrote %d bytes to %s", len(params.Content), params.Path), false, nil
}

// --- fs_list ---

type fsListTool struct{ sandboxDir string }

type fsListInput struct {
	Path string `json:"path"`
}

func (t *fsListTool) Name() string { return "fs_list" }

func (t *fsListTool) Definition() anthropic.ToolUnionParam {
	return anthropic.ToolUnionParam{
		OfTool: &anthropic.ToolParam{
			Name:        "fs_list",
			Description: anthropic.String("List files and directories in a path within the sandbox directory. Max 200 entries."),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "Relative path within the sandbox directory (empty or \".\" for root)",
					},
				},
			},
		},
	}
}

func (t *fsListTool) Execute(ctx context.Context, input json.RawMessage) (string, bool, error) {
	var params fsListInput
	if err := json.Unmarshal(input, &params); err != nil {
		return "invalid input: " + err.Error(), true, nil
	}

	if params.Path == "" {
		params.Path = "."
	}

	resolved, err := resolveSandboxedPath(t.sandboxDir, params.Path)
	if err != nil {
		return err.Error(), true, nil
	}

	entries, err := os.ReadDir(resolved)
	if err != nil {
		return "failed to list directory: " + err.Error(), true, nil
	}

	var b strings.Builder
	for i, entry := range entries {
		if i >= maxListEntries {
			fmt.Fprintf(&b, "... and %d more entries\n", len(entries)-maxListEntries)
			break
		}
		suffix := ""
		if entry.IsDir() {
			suffix = "/"
		}
		fmt.Fprintf(&b, "%s%s\n", entry.Name(), suffix)
	}

	if b.Len() == 0 {
		return "(empty directory)", false, nil
	}

	return b.String(), false, nil
}
