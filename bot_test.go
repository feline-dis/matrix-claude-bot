package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

// --- stripMention tests ---

func TestStripMention(t *testing.T) {
	botID := id.UserID("@bot:example.com")
	tests := []struct {
		name     string
		body     string
		expected string
	}{
		{"start", "@bot:example.com hello", "hello"},
		{"middle", "hey @bot:example.com there", "hey  there"},
		{"end", "hello @bot:example.com", "hello"},
		{"multiple", "@bot:example.com hi @bot:example.com", "hi"},
		{"mention only", "@bot:example.com", ""},
		{"no mention", "hello world", "hello world"},
		{"empty body", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripMention(tt.body, botID)
			if got != tt.expected {
				t.Errorf("stripMention(%q) = %q, want %q", tt.body, got, tt.expected)
			}
		})
	}
}

// --- isMentioned tests ---

func TestIsMentioned_ViaUserIDs(t *testing.T) {
	bot := newTestBot(&mockMatrixClient{}, &mockClaudeMessenger{})
	msg := &event.MessageEventContent{
		Body:     "hello",
		Mentions: &event.Mentions{UserIDs: []id.UserID{"@bot:example.com"}},
	}
	if !bot.isMentioned(msg) {
		t.Error("expected mention via UserIDs")
	}
}

func TestIsMentioned_ViaBodyText(t *testing.T) {
	bot := newTestBot(&mockMatrixClient{}, &mockClaudeMessenger{})
	msg := &event.MessageEventContent{
		Body: "hi @bot:example.com",
	}
	if !bot.isMentioned(msg) {
		t.Error("expected mention via body text")
	}
}

func TestIsMentioned_NotMentioned(t *testing.T) {
	bot := newTestBot(&mockMatrixClient{}, &mockClaudeMessenger{})
	msg := &event.MessageEventContent{
		Body: "hello",
	}
	if bot.isMentioned(msg) {
		t.Error("expected no mention")
	}
}

func TestIsMentioned_DifferentUser(t *testing.T) {
	bot := newTestBot(&mockMatrixClient{}, &mockClaudeMessenger{})
	msg := &event.MessageEventContent{
		Body:     "hello",
		Mentions: &event.Mentions{UserIDs: []id.UserID{"@other:example.com"}},
	}
	if bot.isMentioned(msg) {
		t.Error("expected no mention for different user")
	}
}

func TestIsMentioned_EmptyMentions(t *testing.T) {
	bot := newTestBot(&mockMatrixClient{}, &mockClaudeMessenger{})
	msg := &event.MessageEventContent{
		Body:     "hello",
		Mentions: &event.Mentions{},
	}
	if bot.isMentioned(msg) {
		t.Error("expected no mention with empty Mentions struct")
	}
}

// --- handleMessage tests ---

func TestHandleMessage_IgnoresSelf(t *testing.T) {
	matrix := &mockMatrixClient{}
	claude := &mockClaudeMessenger{}
	bot := newTestBot(matrix, claude)

	evt := makeMessageEvent("@bot:example.com", "!room:example.com", "$evt1", 2000, "@bot:example.com hello", nil, nil)
	bot.handleMessage(context.Background(), evt)

	if len(claude.capturedParams) != 0 {
		t.Error("should not call Claude for self messages")
	}
	if len(matrix.sentEvents) != 0 {
		t.Error("should not send reply for self messages")
	}
}

func TestHandleMessage_IgnoresOldMessages(t *testing.T) {
	matrix := &mockMatrixClient{}
	claude := &mockClaudeMessenger{}
	bot := newTestBot(matrix, claude)

	evt := makeMessageEvent("@user:example.com", "!room:example.com", "$evt1", 500,
		"@bot:example.com hello",
		&event.Mentions{UserIDs: []id.UserID{"@bot:example.com"}}, nil)
	bot.handleMessage(context.Background(), evt)

	if len(claude.capturedParams) != 0 {
		t.Error("should not call Claude for old messages")
	}
}

func TestHandleMessage_IgnoresNoMention(t *testing.T) {
	matrix := &mockMatrixClient{}
	claude := &mockClaudeMessenger{}
	bot := newTestBot(matrix, claude)

	evt := makeMessageEvent("@user:example.com", "!room:example.com", "$evt1", 2000, "hello", nil, nil)
	bot.handleMessage(context.Background(), evt)

	if len(claude.capturedParams) != 0 {
		t.Error("should not call Claude without mention")
	}
}

func TestHandleMessage_IgnoresEmptyAfterStrip(t *testing.T) {
	matrix := &mockMatrixClient{}
	claude := &mockClaudeMessenger{}
	bot := newTestBot(matrix, claude)

	evt := makeMessageEvent("@user:example.com", "!room:example.com", "$evt1", 2000,
		"@bot:example.com",
		&event.Mentions{UserIDs: []id.UserID{"@bot:example.com"}}, nil)
	bot.handleMessage(context.Background(), evt)

	if len(claude.capturedParams) != 0 {
		t.Error("should not call Claude when body is only the mention")
	}
}

func TestHandleMessage_Success(t *testing.T) {
	matrix := &mockMatrixClient{}
	claude := &mockClaudeMessenger{}
	bot := newTestBot(matrix, claude)

	evt := makeMessageEvent("@user:example.com", "!room:example.com", "$evt1", 2000,
		"@bot:example.com what is 2+2?",
		&event.Mentions{UserIDs: []id.UserID{"@bot:example.com"}}, nil)
	bot.handleMessage(context.Background(), evt)

	if len(claude.capturedParams) != 1 {
		t.Fatalf("expected 1 Claude call, got %d", len(claude.capturedParams))
	}
	if len(matrix.sentEvents) != 1 {
		t.Fatalf("expected 1 sent event, got %d", len(matrix.sentEvents))
	}

	sent := matrix.sentEvents[0]
	if sent.RoomID != "!room:example.com" {
		t.Errorf("wrong room ID: %s", sent.RoomID)
	}

	content, ok := sent.Content.(*event.MessageEventContent)
	if !ok {
		t.Fatal("sent content is not MessageEventContent")
	}
	if content.Body != "mock response" {
		t.Errorf("expected 'mock response', got %q", content.Body)
	}
	if content.RelatesTo == nil || content.RelatesTo.Type != event.RelThread {
		t.Error("reply should be in a thread")
	}
	if content.RelatesTo.EventID != "$evt1" {
		t.Errorf("thread root should be $evt1, got %s", content.RelatesTo.EventID)
	}
}

func TestHandleMessage_ExistingThread(t *testing.T) {
	matrix := &mockMatrixClient{}
	claude := &mockClaudeMessenger{}
	bot := newTestBot(matrix, claude)

	relatesTo := &event.RelatesTo{
		Type:    event.RelThread,
		EventID: "$root",
	}
	evt := makeMessageEvent("@user:example.com", "!room:example.com", "$evt2", 2000,
		"@bot:example.com follow up",
		&event.Mentions{UserIDs: []id.UserID{"@bot:example.com"}}, relatesTo)
	bot.handleMessage(context.Background(), evt)

	if len(matrix.sentEvents) != 1 {
		t.Fatalf("expected 1 sent event, got %d", len(matrix.sentEvents))
	}
	content := matrix.sentEvents[0].Content.(*event.MessageEventContent)
	if content.RelatesTo.EventID != "$root" {
		t.Errorf("thread root should be $root, got %s", content.RelatesTo.EventID)
	}
}

func TestHandleMessage_ClaudeError(t *testing.T) {
	matrix := &mockMatrixClient{}
	claude := &mockClaudeMessenger{
		newMessageFunc: func(ctx context.Context, params anthropic.MessageNewParams) (*anthropic.Message, error) {
			return nil, fmt.Errorf("API error")
		},
	}
	bot := newTestBot(matrix, claude)

	evt := makeMessageEvent("@user:example.com", "!room:example.com", "$evt1", 2000,
		"@bot:example.com hello",
		&event.Mentions{UserIDs: []id.UserID{"@bot:example.com"}}, nil)
	bot.handleMessage(context.Background(), evt)

	if len(matrix.sentEvents) != 1 {
		t.Fatalf("expected error reply, got %d sent events", len(matrix.sentEvents))
	}
	content := matrix.sentEvents[0].Content.(*event.MessageEventContent)
	if content.Body != "Sorry, I encountered an error generating a response." {
		t.Errorf("unexpected error message: %q", content.Body)
	}
}

func TestHandleMessage_WithSystemPrompt(t *testing.T) {
	matrix := &mockMatrixClient{}
	claude := &mockClaudeMessenger{}
	bot := newTestBot(matrix, claude)
	bot.config.SystemPrompt = "Be concise."

	evt := makeMessageEvent("@user:example.com", "!room:example.com", "$evt1", 2000,
		"@bot:example.com hello",
		&event.Mentions{UserIDs: []id.UserID{"@bot:example.com"}}, nil)
	bot.handleMessage(context.Background(), evt)

	if len(claude.capturedParams) != 1 {
		t.Fatal("expected 1 Claude call")
	}
	params := claude.capturedParams[0]
	if len(params.System) == 0 {
		t.Fatal("expected system prompt to be set")
	}
	if params.System[0].Text != "Be concise." {
		t.Errorf("unexpected system prompt: %q", params.System[0].Text)
	}
}

// --- handleMemberEvent tests ---

func TestHandleMemberEvent_JoinsOnInvite(t *testing.T) {
	matrix := &mockMatrixClient{}
	bot := newTestBot(matrix, &mockClaudeMessenger{})

	evt := makeMemberEvent("@admin:example.com", "!room:example.com", "@bot:example.com", event.MembershipInvite)
	bot.handleMemberEvent(context.Background(), evt)

	if len(matrix.joinedRooms) != 1 {
		t.Fatalf("expected 1 join, got %d", len(matrix.joinedRooms))
	}
	if matrix.joinedRooms[0] != "!room:example.com" {
		t.Errorf("joined wrong room: %s", matrix.joinedRooms[0])
	}
}

func TestHandleMemberEvent_IgnoresDifferentUser(t *testing.T) {
	matrix := &mockMatrixClient{}
	bot := newTestBot(matrix, &mockClaudeMessenger{})

	evt := makeMemberEvent("@admin:example.com", "!room:example.com", "@other:example.com", event.MembershipInvite)
	bot.handleMemberEvent(context.Background(), evt)

	if len(matrix.joinedRooms) != 0 {
		t.Error("should not join for different user")
	}
}

func TestHandleMemberEvent_IgnoresNonInvite(t *testing.T) {
	matrix := &mockMatrixClient{}
	bot := newTestBot(matrix, &mockClaudeMessenger{})

	evt := makeMemberEvent("@admin:example.com", "!room:example.com", "@bot:example.com", event.MembershipJoin)
	bot.handleMemberEvent(context.Background(), evt)

	if len(matrix.joinedRooms) != 0 {
		t.Error("should not join for non-invite membership")
	}
}

func TestHandleMemberEvent_JoinError(t *testing.T) {
	matrix := &mockMatrixClient{
		joinRoomByIDFunc: func(ctx context.Context, roomID id.RoomID) (*mautrix.RespJoinRoom, error) {
			return nil, fmt.Errorf("join failed")
		},
	}
	bot := newTestBot(matrix, &mockClaudeMessenger{})

	evt := makeMemberEvent("@admin:example.com", "!room:example.com", "@bot:example.com", event.MembershipInvite)
	bot.handleMemberEvent(context.Background(), evt)
	// Should not panic; joinedRooms still has the room because our mock appends before checking func
}

// --- sendThreadReply tests ---

func TestSendThreadReply_CorrectContent(t *testing.T) {
	matrix := &mockMatrixClient{}
	bot := newTestBot(matrix, &mockClaudeMessenger{})

	bot.sendThreadReply(context.Background(), "!room:example.com", "$root", "$reply-to", "hello world")

	if len(matrix.sentEvents) != 1 {
		t.Fatalf("expected 1 sent event, got %d", len(matrix.sentEvents))
	}

	sent := matrix.sentEvents[0]
	if sent.RoomID != "!room:example.com" {
		t.Errorf("wrong room: %s", sent.RoomID)
	}
	if sent.EventType != event.EventMessage {
		t.Errorf("wrong event type: %s", sent.EventType.Type)
	}

	content, ok := sent.Content.(*event.MessageEventContent)
	if !ok {
		t.Fatal("content is not MessageEventContent")
	}
	if content.MsgType != event.MsgText {
		t.Errorf("wrong msg type: %s", content.MsgType)
	}
	if content.Body != "hello world" {
		t.Errorf("wrong body: %q", content.Body)
	}
	if content.RelatesTo == nil {
		t.Fatal("RelatesTo should be set")
	}
	if content.RelatesTo.Type != event.RelThread {
		t.Error("should be a thread relation")
	}
	if content.RelatesTo.EventID != "$root" {
		t.Errorf("wrong thread root: %s", content.RelatesTo.EventID)
	}
	if content.RelatesTo.InReplyTo == nil || content.RelatesTo.InReplyTo.EventID != "$reply-to" {
		t.Error("InReplyTo should reference the original event")
	}
	if !content.RelatesTo.IsFallingBack {
		t.Error("IsFallingBack should be true")
	}
}

func TestSendThreadReply_SendError(t *testing.T) {
	matrix := &mockMatrixClient{
		sendMessageEventFunc: func(ctx context.Context, roomID id.RoomID, eventType event.Type, contentJSON interface{}, extra ...mautrix.ReqSendEvent) (*mautrix.RespSendEvent, error) {
			return nil, fmt.Errorf("send failed")
		},
	}
	bot := newTestBot(matrix, &mockClaudeMessenger{})

	// Should not panic
	bot.sendThreadReply(context.Background(), "!room:example.com", "$root", "$reply-to", "hello")
}

// --- handleMessage timing edge case ---

func TestHandleMessage_ExactStartTime(t *testing.T) {
	matrix := &mockMatrixClient{}
	claude := &mockClaudeMessenger{}
	bot := newTestBot(matrix, claude)
	bot.startTime = time.UnixMilli(2000)

	evt := makeMessageEvent("@user:example.com", "!room:example.com", "$evt1", 2000,
		"@bot:example.com hello",
		&event.Mentions{UserIDs: []id.UserID{"@bot:example.com"}}, nil)
	bot.handleMessage(context.Background(), evt)

	// Timestamp == startTime.UnixMilli() means NOT less than, so should be processed
	if len(claude.capturedParams) != 1 {
		t.Error("message at exact start time should be processed")
	}
}
