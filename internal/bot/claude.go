package bot

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

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

func extractText(content []anthropic.ContentBlockUnion) string {
	var parts []string
	for _, block := range content {
		if block.Type == "text" {
			parts = append(parts, block.Text)
		}
	}
	return strings.Join(parts, "\n")
}

// toolCapabilitiesPrompt generates a system prompt section describing the
// tools currently available, built from the Registry so it stays in sync
// with what is actually registered.
func (b *Bot) toolCapabilitiesPrompt() string {
	if b.tools == nil || b.tools.IsEmpty() {
		return ""
	}

	var parts []string

	if b.tools.HasServerTools() {
		parts = append(parts, "- Web search: you can search the web for current information")
	}

	localNames := b.tools.LocalToolNames()
	for _, name := range localNames {
		switch {
		case strings.HasPrefix(name, "fs_"):
			parts = append(parts, "- Filesystem: you can read, write, and list files in a sandboxed directory")
		default:
			parts = append(parts, fmt.Sprintf("- %s", name))
		}
	}

	if len(parts) == 0 {
		return ""
	}

	// Deduplicate (e.g. multiple fs_ tools produce one line)
	seen := make(map[string]bool)
	var unique []string
	for _, p := range parts {
		if !seen[p] {
			seen[p] = true
			unique = append(unique, p)
		}
	}

	return "\n\nYou have access to the following tools:\n" + strings.Join(unique, "\n")
}

func (b *Bot) getClaudeResponse(ctx context.Context, threadID id.EventID, userText string) (string, error) {
	userMsg := anthropic.NewUserMessage(anthropic.NewTextBlock(userText))
	b.conversations.Append(threadID, userMsg)

	maxIterations := b.config.MaxToolIterations
	if maxIterations <= 0 {
		maxIterations = 1
	}

	toolTimeout := b.config.ToolTimeout
	if toolTimeout <= 0 {
		toolTimeout = 30 * time.Second
	}

	hasTools := b.tools != nil && !b.tools.IsEmpty()

	for i := 0; i < maxIterations; i++ {
		params := anthropic.MessageNewParams{
			Model:     anthropic.Model(b.config.Model),
			Messages:  b.conversations.Get(threadID),
			MaxTokens: b.config.MaxTokens,
		}

		systemPrompt := b.config.SystemPrompt + b.toolCapabilitiesPrompt()
		if systemPrompt != "" {
			params.System = []anthropic.TextBlockParam{
				{Text: systemPrompt},
			}
		}

		if hasTools {
			defs := b.tools.Definitions()
			params.Tools = defs
			if i == 0 {
				names := make([]string, len(defs))
				for j, d := range defs {
					if d.OfTool != nil {
						names[j] = d.OfTool.Name
					} else if d.OfWebSearchTool20250305 != nil {
						names[j] = "web_search"
					} else {
						names[j] = "(unknown)"
					}
				}
				log.Printf("Sending %d tool(s) to Claude: %v", len(defs), names)
			}
		}

		resp, err := b.claude.NewMessage(ctx, params)
		if err != nil {
			return "", fmt.Errorf("claude API call failed: %w", err)
		}

		b.conversations.Append(threadID, resp.ToParam())

		if resp.StopReason != anthropic.StopReasonToolUse {
			return extractText(resp.Content), nil
		}

		// No local tools to execute -- shouldn't happen, but guard against
		// infinite loops if only server tools are registered.
		if !hasTools {
			return extractText(resp.Content), nil
		}

		var toolResults []anthropic.ContentBlockParamUnion
		for _, block := range resp.Content {
			if block.Type != "tool_use" {
				continue
			}
			if !b.tools.HasLocalTool(block.Name) {
				continue
			}

			toolCtx, cancel := context.WithTimeout(ctx, toolTimeout)
			result, isError, err := b.tools.Execute(toolCtx, block.Name, block.Input)
			cancel()

			if err != nil {
				log.Printf("Tool execution error (%s): %v", block.Name, err)
				result = "internal error executing tool"
				isError = true
			}

			toolResults = append(toolResults, anthropic.NewToolResultBlock(block.ID, result, isError))
		}

		if len(toolResults) == 0 {
			return extractText(resp.Content), nil
		}

		b.conversations.Append(threadID, anthropic.NewUserMessage(toolResults...))
	}

	return "reached maximum tool use iterations", nil
}
