package main

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/anthropics/anthropic-sdk-go"
	"maunium.net/go/mautrix/id"
)

type ConversationStore struct {
	mu    sync.RWMutex
	convs map[id.EventID][]anthropic.MessageParam
}

func NewConversationStore() *ConversationStore {
	return &ConversationStore{
		convs: make(map[id.EventID][]anthropic.MessageParam),
	}
}

func (s *ConversationStore) Get(threadID id.EventID) []anthropic.MessageParam {
	s.mu.RLock()
	defer s.mu.RUnlock()
	history := s.convs[threadID]
	copied := make([]anthropic.MessageParam, len(history))
	copy(copied, history)
	return copied
}

func (s *ConversationStore) Append(threadID id.EventID, msgs ...anthropic.MessageParam) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.convs[threadID] = append(s.convs[threadID], msgs...)
}

func (b *Bot) getClaudeResponse(ctx context.Context, threadID id.EventID, userText string) (string, error) {
	userMsg := anthropic.NewUserMessage(anthropic.NewTextBlock(userText))
	b.conversations.Append(threadID, userMsg)

	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(b.config.Model),
		Messages:  b.conversations.Get(threadID),
		MaxTokens: b.config.MaxTokens,
	}

	if b.config.SystemPrompt != "" {
		params.System = []anthropic.TextBlockParam{
			{Text: b.config.SystemPrompt},
		}
	}

	resp, err := b.claude.NewMessage(ctx, params)
	if err != nil {
		return "", fmt.Errorf("claude API call failed: %w", err)
	}

	var parts []string
	for _, block := range resp.Content {
		if block.Type == "text" {
			parts = append(parts, block.Text)
		}
	}

	responseText := strings.Join(parts, "\n")

	b.conversations.Append(threadID, resp.ToParam())

	return responseText, nil
}
