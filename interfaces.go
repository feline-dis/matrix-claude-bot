package main

import (
	"context"

	"github.com/anthropics/anthropic-sdk-go"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

// MatrixClient abstracts the mautrix.Client methods used by Bot.
type MatrixClient interface {
	JoinRoomByID(ctx context.Context, roomID id.RoomID) (*mautrix.RespJoinRoom, error)
	SendMessageEvent(ctx context.Context, roomID id.RoomID, eventType event.Type, contentJSON interface{}, extra ...mautrix.ReqSendEvent) (*mautrix.RespSendEvent, error)
}

// ClaudeMessenger abstracts the Claude message-creation capability.
type ClaudeMessenger interface {
	NewMessage(ctx context.Context, params anthropic.MessageNewParams) (*anthropic.Message, error)
}

// claudeAdapter wraps anthropic.Client to satisfy ClaudeMessenger.
type claudeAdapter struct {
	client anthropic.Client
}

func (a *claudeAdapter) NewMessage(ctx context.Context, params anthropic.MessageNewParams) (*anthropic.Message, error) {
	return a.client.Messages.New(ctx, params)
}
