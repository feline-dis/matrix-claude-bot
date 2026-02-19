package main

import (
	"context"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

type mockMatrixClient struct {
	joinRoomByIDFunc     func(ctx context.Context, roomID id.RoomID) (*mautrix.RespJoinRoom, error)
	sendMessageEventFunc func(ctx context.Context, roomID id.RoomID, eventType event.Type, contentJSON interface{}, extra ...mautrix.ReqSendEvent) (*mautrix.RespSendEvent, error)
	sentEvents           []sentEvent
	joinedRooms          []id.RoomID
}

type sentEvent struct {
	RoomID    id.RoomID
	EventType event.Type
	Content   interface{}
}

func (m *mockMatrixClient) JoinRoomByID(ctx context.Context, roomID id.RoomID) (*mautrix.RespJoinRoom, error) {
	m.joinedRooms = append(m.joinedRooms, roomID)
	if m.joinRoomByIDFunc != nil {
		return m.joinRoomByIDFunc(ctx, roomID)
	}
	return &mautrix.RespJoinRoom{RoomID: roomID}, nil
}

func (m *mockMatrixClient) SendMessageEvent(ctx context.Context, roomID id.RoomID, eventType event.Type, contentJSON interface{}, extra ...mautrix.ReqSendEvent) (*mautrix.RespSendEvent, error) {
	m.sentEvents = append(m.sentEvents, sentEvent{RoomID: roomID, EventType: eventType, Content: contentJSON})
	if m.sendMessageEventFunc != nil {
		return m.sendMessageEventFunc(ctx, roomID, eventType, contentJSON, extra...)
	}
	return &mautrix.RespSendEvent{EventID: "$reply"}, nil
}

type mockClaudeMessenger struct {
	newMessageFunc func(ctx context.Context, params anthropic.MessageNewParams) (*anthropic.Message, error)
	capturedParams []anthropic.MessageNewParams
}

func (m *mockClaudeMessenger) NewMessage(ctx context.Context, params anthropic.MessageNewParams) (*anthropic.Message, error) {
	m.capturedParams = append(m.capturedParams, params)
	if m.newMessageFunc != nil {
		return m.newMessageFunc(ctx, params)
	}
	return makeClaudeResponse("mock response"), nil
}

func makeClaudeResponse(texts ...string) *anthropic.Message {
	blocks := make([]anthropic.ContentBlockUnion, len(texts))
	for i, t := range texts {
		blocks[i] = anthropic.ContentBlockUnion{Type: "text", Text: t}
	}
	return &anthropic.Message{
		Role:    "assistant",
		Content: blocks,
	}
}

func newTestBot(matrix *mockMatrixClient, claude *mockClaudeMessenger) *Bot {
	return &Bot{
		matrix: matrix,
		claude: claude,
		config: Config{
			UserID:    "@bot:example.com",
			Model:     "claude-sonnet-4-20250514",
			MaxTokens: 1024,
		},
		conversations: NewConversationStore(),
		startTime:     time.UnixMilli(1000),
	}
}

func makeMessageEvent(sender id.UserID, roomID id.RoomID, eventID id.EventID, timestamp int64, body string, mentions *event.Mentions, relatesTo *event.RelatesTo) *event.Event {
	msg := &event.MessageEventContent{
		MsgType:   event.MsgText,
		Body:      body,
		Mentions:  mentions,
		RelatesTo: relatesTo,
	}
	return &event.Event{
		Sender:    sender,
		RoomID:    roomID,
		ID:        eventID,
		Timestamp: timestamp,
		Content:   event.Content{Parsed: msg},
	}
}

func makeMemberEvent(sender id.UserID, roomID id.RoomID, stateKey string, membership event.Membership) *event.Event {
	member := &event.MemberEventContent{
		Membership: membership,
	}
	sk := stateKey
	return &event.Event{
		Sender:   sender,
		RoomID:   roomID,
		StateKey: &sk,
		Content:  event.Content{Parsed: member},
	}
}
