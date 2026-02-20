package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"maunium.net/go/mautrix/id"
)

func TestConversationStore_EmptyGet(t *testing.T) {
	store := NewConversationStore()
	msgs := store.Get("$unknown")
	if msgs == nil {
		t.Fatal("Get on unknown thread should return non-nil slice")
	}
	if len(msgs) != 0 {
		t.Fatalf("expected empty slice, got %d items", len(msgs))
	}
}

func TestConversationStore_AppendAndGet(t *testing.T) {
	store := NewConversationStore()
	threadID := id.EventID("$thread1")
	msg := anthropic.NewUserMessage(anthropic.NewTextBlock("hello"))

	store.Append(threadID, msg)
	msgs := store.Get(threadID)

	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
}

func TestConversationStore_GetReturnsCopy(t *testing.T) {
	store := NewConversationStore()
	threadID := id.EventID("$thread1")
	store.Append(threadID, anthropic.NewUserMessage(anthropic.NewTextBlock("hello")))

	msgs := store.Get(threadID)
	msgs = append(msgs, anthropic.NewUserMessage(anthropic.NewTextBlock("extra")))

	original := store.Get(threadID)
	if len(original) != 1 {
		t.Fatalf("mutating returned slice should not affect store; expected 1, got %d", len(original))
	}
}

func TestConversationStore_SeparateThreads(t *testing.T) {
	store := NewConversationStore()
	thread1 := id.EventID("$thread1")
	thread2 := id.EventID("$thread2")

	store.Append(thread1, anthropic.NewUserMessage(anthropic.NewTextBlock("a")))
	store.Append(thread2, anthropic.NewUserMessage(anthropic.NewTextBlock("b")), anthropic.NewUserMessage(anthropic.NewTextBlock("c")))

	if len(store.Get(thread1)) != 1 {
		t.Fatal("thread1 should have 1 message")
	}
	if len(store.Get(thread2)) != 2 {
		t.Fatal("thread2 should have 2 messages")
	}
}

func TestConversationStore_ConcurrentAccess(t *testing.T) {
	store := NewConversationStore()
	threadID := id.EventID("$concurrent")
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			store.Append(threadID, anthropic.NewUserMessage(anthropic.NewTextBlock("msg")))
		}()
		go func() {
			defer wg.Done()
			store.Get(threadID)
		}()
	}
	wg.Wait()
}

func TestGetClaudeResponse_Success(t *testing.T) {
	matrix := &mockMatrixClient{}
	claude := &mockClaudeMessenger{}
	bot := newTestBot(matrix, claude)
	threadID := id.EventID("$thread1")

	resp, err := bot.getClaudeResponse(context.Background(), threadID, "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != "mock response" {
		t.Fatalf("expected 'mock response', got %q", resp)
	}
	if len(claude.capturedParams) != 1 {
		t.Fatal("expected 1 Claude API call")
	}

	msgs := bot.conversations.Get(threadID)
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages in store (user + assistant), got %d", len(msgs))
	}
}

func TestGetClaudeResponse_MultipleTextBlocks(t *testing.T) {
	matrix := &mockMatrixClient{}
	claude := &mockClaudeMessenger{
		newMessageFunc: func(ctx context.Context, params anthropic.MessageNewParams) (*anthropic.Message, error) {
			return makeClaudeResponse("part one", "part two"), nil
		},
	}
	bot := newTestBot(matrix, claude)

	resp, err := bot.getClaudeResponse(context.Background(), "$thread1", "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != "part one\npart two" {
		t.Fatalf("expected joined text blocks, got %q", resp)
	}
}

func TestGetClaudeResponse_APIError(t *testing.T) {
	matrix := &mockMatrixClient{}
	claude := &mockClaudeMessenger{
		newMessageFunc: func(ctx context.Context, params anthropic.MessageNewParams) (*anthropic.Message, error) {
			return nil, fmt.Errorf("API unavailable")
		},
	}
	bot := newTestBot(matrix, claude)
	threadID := id.EventID("$thread1")

	resp, err := bot.getClaudeResponse(context.Background(), threadID, "hello")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if resp != "" {
		t.Fatalf("expected empty response on error, got %q", resp)
	}

	msgs := bot.conversations.Get(threadID)
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message in store (user only), got %d", len(msgs))
	}
}

func TestGetClaudeResponse_ConversationHistory(t *testing.T) {
	matrix := &mockMatrixClient{}
	claude := &mockClaudeMessenger{}
	bot := newTestBot(matrix, claude)
	threadID := id.EventID("$thread1")

	_, err := bot.getClaudeResponse(context.Background(), threadID, "first")
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}

	_, err = bot.getClaudeResponse(context.Background(), threadID, "second")
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}

	if len(claude.capturedParams) != 2 {
		t.Fatalf("expected 2 Claude calls, got %d", len(claude.capturedParams))
	}

	secondCallMsgs := claude.capturedParams[1].Messages
	if len(secondCallMsgs) != 3 {
		t.Fatalf("second call should have 3 messages (user1, assistant1, user2), got %d", len(secondCallMsgs))
	}
}

func TestGetClaudeResponse_WithSystemPrompt(t *testing.T) {
	matrix := &mockMatrixClient{}
	claude := &mockClaudeMessenger{}
	bot := newTestBot(matrix, claude)
	bot.config.SystemPrompt = "You are a helpful bot."

	_, err := bot.getClaudeResponse(context.Background(), "$thread1", "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	params := claude.capturedParams[0]
	if len(params.System) == 0 {
		t.Fatal("expected system prompt to be set")
	}
	// No tools registered, so prompt should be exactly the configured value.
	if params.System[0].Text != "You are a helpful bot." {
		t.Fatalf("unexpected system prompt: %q", params.System[0].Text)
	}
}

func TestGetClaudeResponse_WithSystemPromptAndTools(t *testing.T) {
	matrix := &mockMatrixClient{}
	claude := &mockClaudeMessenger{}
	bot := newTestBot(matrix, claude)
	bot.config.SystemPrompt = "You are a helpful bot."
	bot.tools.Register(&fakeTool{name: "my_tool", result: "ok"})

	_, err := bot.getClaudeResponse(context.Background(), "$thread1", "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	params := claude.capturedParams[0]
	if len(params.System) == 0 {
		t.Fatal("expected system prompt to be set")
	}
	prompt := params.System[0].Text
	if !strings.HasPrefix(prompt, "You are a helpful bot.") {
		t.Fatalf("system prompt should start with configured value, got %q", prompt)
	}
	if !strings.Contains(prompt, "my_tool") {
		t.Fatalf("system prompt should mention registered tool, got %q", prompt)
	}
}

func TestGetClaudeResponse_NoSystemPrompt(t *testing.T) {
	matrix := &mockMatrixClient{}
	claude := &mockClaudeMessenger{}
	bot := newTestBot(matrix, claude)

	_, err := bot.getClaudeResponse(context.Background(), "$thread1", "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	params := claude.capturedParams[0]
	if len(params.System) != 0 {
		t.Fatalf("expected no system prompt when no config and no tools, got %d blocks", len(params.System))
	}
}

func TestGetClaudeResponse_NoSystemPromptWithTools(t *testing.T) {
	matrix := &mockMatrixClient{}
	claude := &mockClaudeMessenger{}
	bot := newTestBot(matrix, claude)
	bot.tools.Register(&fakeTool{name: "my_tool", result: "ok"})

	_, err := bot.getClaudeResponse(context.Background(), "$thread1", "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	params := claude.capturedParams[0]
	if len(params.System) == 0 {
		t.Fatal("expected system prompt with tool capabilities even without configured prompt")
	}
	if !strings.Contains(params.System[0].Text, "my_tool") {
		t.Fatalf("system prompt should mention registered tool, got %q", params.System[0].Text)
	}
}

// --- Tool use loop tests ---

func TestGetClaudeResponse_ToolUseLoop(t *testing.T) {
	matrix := &mockMatrixClient{}
	callCount := 0
	claude := &mockClaudeMessenger{
		newMessageFunc: func(ctx context.Context, params anthropic.MessageNewParams) (*anthropic.Message, error) {
			callCount++
			if callCount == 1 {
				return makeToolUseResponse("tool_1", "echo", json.RawMessage(`{"text":"hi"}`)), nil
			}
			return makeClaudeResponse("final answer"), nil
		},
	}
	bot := newTestBot(matrix, claude)
	bot.tools.Register(&fakeTool{name: "echo", result: "echoed: hi"})

	resp, err := bot.getClaudeResponse(context.Background(), "$thread1", "test tool use")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != "final answer" {
		t.Errorf("expected 'final answer', got %q", resp)
	}
	if callCount != 2 {
		t.Errorf("expected 2 Claude API calls, got %d", callCount)
	}

	// Conversation should have: user, assistant(tool_use), user(tool_result), assistant(text)
	msgs := bot.conversations.Get("$thread1")
	if len(msgs) != 4 {
		t.Fatalf("expected 4 messages in store, got %d", len(msgs))
	}
}

func TestGetClaudeResponse_NoToolsPreservesExistingBehavior(t *testing.T) {
	matrix := &mockMatrixClient{}
	claude := &mockClaudeMessenger{}
	bot := newTestBot(matrix, claude)
	// tools registry is empty (no tools registered)

	resp, err := bot.getClaudeResponse(context.Background(), "$thread1", "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != "mock response" {
		t.Errorf("expected 'mock response', got %q", resp)
	}
	if len(claude.capturedParams) != 1 {
		t.Errorf("expected 1 API call, got %d", len(claude.capturedParams))
	}
	if len(claude.capturedParams[0].Tools) != 0 {
		t.Error("expected no tools in API call when registry is empty")
	}
}

func TestGetClaudeResponse_ToolsIncludedInAPICall(t *testing.T) {
	matrix := &mockMatrixClient{}
	claude := &mockClaudeMessenger{}
	bot := newTestBot(matrix, claude)
	bot.tools.Register(&fakeTool{name: "my_tool", result: "ok"})

	_, err := bot.getClaudeResponse(context.Background(), "$thread1", "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(claude.capturedParams[0].Tools) != 1 {
		t.Errorf("expected 1 tool definition, got %d", len(claude.capturedParams[0].Tools))
	}
}

func TestGetClaudeResponse_MaxIterationsReached(t *testing.T) {
	matrix := &mockMatrixClient{}
	claude := &mockClaudeMessenger{
		newMessageFunc: func(ctx context.Context, params anthropic.MessageNewParams) (*anthropic.Message, error) {
			// Always return tool_use to force hitting the max iterations limit.
			return makeToolUseResponse("tool_1", "echo", json.RawMessage(`{}`)), nil
		},
	}
	bot := newTestBot(matrix, claude)
	bot.config.MaxToolIterations = 3
	bot.tools.Register(&fakeTool{name: "echo", result: "ok"})

	resp, err := bot.getClaudeResponse(context.Background(), "$thread1", "loop forever")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != "reached maximum tool use iterations" {
		t.Errorf("expected max iterations message, got %q", resp)
	}
	if len(claude.capturedParams) != 3 {
		t.Errorf("expected 3 API calls (max_iterations=3), got %d", len(claude.capturedParams))
	}
}

func TestGetClaudeResponse_ToolExecutionError(t *testing.T) {
	matrix := &mockMatrixClient{}
	callCount := 0
	claude := &mockClaudeMessenger{
		newMessageFunc: func(ctx context.Context, params anthropic.MessageNewParams) (*anthropic.Message, error) {
			callCount++
			if callCount == 1 {
				return makeToolUseResponse("tool_1", "failing", json.RawMessage(`{}`)), nil
			}
			return makeClaudeResponse("handled error"), nil
		},
	}
	bot := newTestBot(matrix, claude)

	// Register a tool that returns isError=true
	bot.tools.Register(&fakeTool{name: "failing", result: "something went wrong"})

	resp, err := bot.getClaudeResponse(context.Background(), "$thread1", "test error")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != "handled error" {
		t.Errorf("expected 'handled error', got %q", resp)
	}
}

func TestExtractText(t *testing.T) {
	blocks := []anthropic.ContentBlockUnion{
		{Type: "thinking", Thinking: "hmm"},
		{Type: "text", Text: "hello"},
		{Type: "tool_use", Name: "some_tool"},
		{Type: "text", Text: "world"},
	}
	result := extractText(blocks)
	if result != "hello\nworld" {
		t.Errorf("expected 'hello\\nworld', got %q", result)
	}
}

func TestExtractText_Empty(t *testing.T) {
	result := extractText(nil)
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

// --- toolCapabilitiesPrompt tests ---

func TestToolCapabilitiesPrompt_NoTools(t *testing.T) {
	bot := newTestBot(&mockMatrixClient{}, &mockClaudeMessenger{})
	if got := bot.toolCapabilitiesPrompt(); got != "" {
		t.Errorf("expected empty string for no tools, got %q", got)
	}
}

func TestToolCapabilitiesPrompt_NilRegistry(t *testing.T) {
	bot := newTestBot(&mockMatrixClient{}, &mockClaudeMessenger{})
	bot.tools = nil
	if got := bot.toolCapabilitiesPrompt(); got != "" {
		t.Errorf("expected empty string for nil registry, got %q", got)
	}
}

func TestToolCapabilitiesPrompt_WebSearch(t *testing.T) {
	bot := newTestBot(&mockMatrixClient{}, &mockClaudeMessenger{})
	bot.tools.AddServerTool(anthropic.ToolUnionParam{
		OfWebSearchTool20250305: &anthropic.WebSearchTool20250305Param{},
	})

	got := bot.toolCapabilitiesPrompt()
	if !strings.Contains(got, "Web search") {
		t.Errorf("expected web search capability, got %q", got)
	}
}

func TestToolCapabilitiesPrompt_Filesystem(t *testing.T) {
	bot := newTestBot(&mockMatrixClient{}, &mockClaudeMessenger{})
	bot.tools.Register(&fakeTool{name: "fs_read", result: "ok"})
	bot.tools.Register(&fakeTool{name: "fs_write", result: "ok"})
	bot.tools.Register(&fakeTool{name: "fs_list", result: "ok"})

	got := bot.toolCapabilitiesPrompt()
	if !strings.Contains(got, "Filesystem") {
		t.Errorf("expected filesystem capability, got %q", got)
	}
	// All three fs_ tools should be deduplicated into one line.
	if strings.Count(got, "Filesystem") != 1 {
		t.Errorf("expected exactly one Filesystem line, got %q", got)
	}
}

func TestToolCapabilitiesPrompt_CustomTool(t *testing.T) {
	bot := newTestBot(&mockMatrixClient{}, &mockClaudeMessenger{})
	bot.tools.Register(&fakeTool{name: "weather_lookup", result: "ok"})

	got := bot.toolCapabilitiesPrompt()
	if !strings.Contains(got, "weather_lookup") {
		t.Errorf("expected custom tool name in output, got %q", got)
	}
}

func TestToolCapabilitiesPrompt_AllTypes(t *testing.T) {
	bot := newTestBot(&mockMatrixClient{}, &mockClaudeMessenger{})
	bot.tools.AddServerTool(anthropic.ToolUnionParam{
		OfWebSearchTool20250305: &anthropic.WebSearchTool20250305Param{},
	})
	bot.tools.Register(&fakeTool{name: "fs_read", result: "ok"})
	bot.tools.Register(&fakeTool{name: "custom_tool", result: "ok"})

	got := bot.toolCapabilitiesPrompt()
	if !strings.Contains(got, "Web search") {
		t.Errorf("expected web search capability, got %q", got)
	}
	if !strings.Contains(got, "Filesystem") {
		t.Errorf("expected filesystem capability, got %q", got)
	}
	if !strings.Contains(got, "custom_tool") {
		t.Errorf("expected custom tool name, got %q", got)
	}
}
