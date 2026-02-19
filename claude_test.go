package main

import (
	"context"
	"fmt"
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
	if params.System[0].Text != "You are a helpful bot." {
		t.Fatalf("unexpected system prompt: %q", params.System[0].Text)
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
		t.Fatalf("expected no system prompt, got %d blocks", len(params.System))
	}
}
