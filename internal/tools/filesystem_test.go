package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFsRead_Success(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("world"), 0o644)

	tool := &fsReadTool{sandboxDir: dir}
	result, isErr, err := tool.Execute(context.Background(), json.RawMessage(`{"path":"hello.txt"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if isErr {
		t.Errorf("expected no error flag, got result: %s", result)
	}
	if result != "world" {
		t.Errorf("expected 'world', got %q", result)
	}
}

func TestFsRead_NotFound(t *testing.T) {
	dir := t.TempDir()
	tool := &fsReadTool{sandboxDir: dir}
	result, isErr, _ := tool.Execute(context.Background(), json.RawMessage(`{"path":"missing.txt"}`))
	if !isErr {
		t.Error("expected isError=true for missing file")
	}
	if !strings.Contains(result, "not found") {
		t.Errorf("expected 'not found' in result, got %q", result)
	}
}

func TestFsRead_PathTraversal(t *testing.T) {
	dir := t.TempDir()
	tool := &fsReadTool{sandboxDir: dir}
	result, isErr, _ := tool.Execute(context.Background(), json.RawMessage(`{"path":"../../etc/passwd"}`))
	if !isErr {
		t.Error("expected isError=true for path traversal")
	}
	if !strings.Contains(result, "escapes sandbox") {
		t.Errorf("expected 'escapes sandbox' in result, got %q", result)
	}
}

func TestFsRead_Directory(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, "subdir"), 0o755)

	tool := &fsReadTool{sandboxDir: dir}
	result, isErr, _ := tool.Execute(context.Background(), json.RawMessage(`{"path":"subdir"}`))
	if !isErr {
		t.Error("expected isError=true for directory")
	}
	if !strings.Contains(result, "directory") {
		t.Errorf("expected 'directory' in result, got %q", result)
	}
}

func TestFsWrite_Success(t *testing.T) {
	dir := t.TempDir()
	tool := &fsWriteTool{sandboxDir: dir}
	result, isErr, err := tool.Execute(context.Background(), json.RawMessage(`{"path":"sub/test.txt","content":"hello"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if isErr {
		t.Errorf("expected no error flag, got result: %s", result)
	}

	data, err := os.ReadFile(filepath.Join(dir, "sub", "test.txt"))
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}
	if string(data) != "hello" {
		t.Errorf("expected 'hello', got %q", string(data))
	}
}

func TestFsWrite_PathTraversal(t *testing.T) {
	dir := t.TempDir()
	tool := &fsWriteTool{sandboxDir: dir}
	result, isErr, _ := tool.Execute(context.Background(), json.RawMessage(`{"path":"../../tmp/evil.txt","content":"bad"}`))
	if !isErr {
		t.Error("expected isError=true for path traversal")
	}
	if !strings.Contains(result, "escapes sandbox") {
		t.Errorf("expected 'escapes sandbox' in result, got %q", result)
	}
}

func TestFsList_Success(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644)
	os.Mkdir(filepath.Join(dir, "subdir"), 0o755)

	tool := &fsListTool{sandboxDir: dir}
	result, isErr, err := tool.Execute(context.Background(), json.RawMessage(`{"path":"."}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if isErr {
		t.Errorf("expected no error flag, got result: %s", result)
	}
	if !strings.Contains(result, "a.txt") {
		t.Errorf("expected 'a.txt' in result, got %q", result)
	}
	if !strings.Contains(result, "subdir/") {
		t.Errorf("expected 'subdir/' in result, got %q", result)
	}
}

func TestFsList_EmptyPath(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("x"), 0o644)

	tool := &fsListTool{sandboxDir: dir}
	result, isErr, _ := tool.Execute(context.Background(), json.RawMessage(`{}`))
	if isErr {
		t.Errorf("expected no error flag, got result: %s", result)
	}
	if !strings.Contains(result, "file.txt") {
		t.Errorf("expected 'file.txt' in result, got %q", result)
	}
}

func TestFsList_EmptyDirectory(t *testing.T) {
	dir := t.TempDir()
	tool := &fsListTool{sandboxDir: dir}
	result, isErr, _ := tool.Execute(context.Background(), json.RawMessage(`{"path":"."}`))
	if isErr {
		t.Error("expected no error flag")
	}
	if result != "(empty directory)" {
		t.Errorf("expected '(empty directory)', got %q", result)
	}
}

func TestFsList_PathTraversal(t *testing.T) {
	dir := t.TempDir()
	tool := &fsListTool{sandboxDir: dir}
	result, isErr, _ := tool.Execute(context.Background(), json.RawMessage(`{"path":"../../"}`))
	if !isErr {
		t.Error("expected isError=true for path traversal")
	}
	if !strings.Contains(result, "escapes sandbox") {
		t.Errorf("expected 'escapes sandbox' in result, got %q", result)
	}
}

func TestResolveSandboxedPath_EmptyPath(t *testing.T) {
	dir := t.TempDir()
	_, err := resolveSandboxedPath(dir, "")
	if err == nil {
		t.Error("expected error for empty path")
	}
}
