package bot

import (
	"context"
	"log"
	"strings"
	"time"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"github.com/feline-dis/matrix-claude-bot/internal/config"
	"github.com/feline-dis/matrix-claude-bot/internal/tools"
)

type Bot struct {
	matrix        MatrixClient
	claude        ClaudeMessenger
	config        config.Config
	conversations *ConversationStore
	tools         *tools.Registry
	startTime     time.Time
}

func NewBot(matrix MatrixClient, claude ClaudeMessenger, cfg config.Config, reg *tools.Registry) *Bot {
	return &Bot{
		matrix:        matrix,
		claude:        claude,
		config:        cfg,
		conversations: NewConversationStore(),
		tools:         reg,
		startTime:     time.Now(),
	}
}

// RegisterHandlers needs the concrete *mautrix.Client for syncer type-assertion.
func RegisterHandlers(matrixClient *mautrix.Client, b *Bot) {
	syncer := matrixClient.Syncer.(*mautrix.DefaultSyncer)

	syncer.OnEventType(event.EventMessage, func(ctx context.Context, evt *event.Event) {
		go b.handleMessage(ctx, evt)
	})

	syncer.OnEventType(event.StateMember, func(ctx context.Context, evt *event.Event) {
		b.handleMemberEvent(ctx, evt)
	})
}

func (b *Bot) handleMessage(ctx context.Context, evt *event.Event) {
	if evt.Sender == b.config.UserID {
		return
	}

	if evt.Timestamp < b.startTime.UnixMilli() {
		return
	}

	msg := evt.Content.AsMessage()
	if msg == nil {
		return
	}

	if !b.isMentioned(msg) {
		return
	}

	userText := stripMention(msg.Body, b.config.UserID)
	if userText == "" {
		return
	}

	threadRootID := evt.ID
	if msg.RelatesTo != nil && msg.RelatesTo.Type == event.RelThread {
		threadRootID = msg.RelatesTo.EventID
	}

	response, err := b.getClaudeResponse(ctx, threadRootID, userText)
	if err != nil {
		log.Printf("Claude API error: %v", err)
		response = "Sorry, I encountered an error generating a response."
	}

	b.sendThreadReply(ctx, evt.RoomID, threadRootID, evt.ID, response)
}

func (b *Bot) handleMemberEvent(ctx context.Context, evt *event.Event) {
	if evt.GetStateKey() != b.config.UserID.String() {
		return
	}
	if evt.Content.AsMember().Membership != event.MembershipInvite {
		return
	}

	log.Printf("Invited to %s by %s", evt.RoomID, evt.Sender)

	_, err := b.matrix.JoinRoomByID(ctx, evt.RoomID)
	if err != nil {
		log.Printf("Failed to join room %s: %v", evt.RoomID, err)
		return
	}

	log.Printf("Joined room %s", evt.RoomID)
}

func (b *Bot) isMentioned(msg *event.MessageEventContent) bool {
	if msg.Mentions != nil {
		for _, uid := range msg.Mentions.UserIDs {
			if uid == b.config.UserID {
				return true
			}
		}
	}
	return strings.Contains(msg.Body, b.config.UserID.String())
}

func stripMention(body string, userID id.UserID) string {
	cleaned := strings.ReplaceAll(body, userID.String(), "")
	return strings.TrimSpace(cleaned)
}

func (b *Bot) sendThreadReply(ctx context.Context, roomID id.RoomID, threadRootID, replyToID id.EventID, text string) {
	content := &event.MessageEventContent{
		MsgType: event.MsgText,
		Body:    text,
	}

	content.RelatesTo = &event.RelatesTo{
		Type:    event.RelThread,
		EventID: threadRootID,
		InReplyTo: &event.InReplyTo{
			EventID: replyToID,
		},
		IsFallingBack: true,
	}

	_, err := b.matrix.SendMessageEvent(ctx, roomID, event.EventMessage, content)
	if err != nil {
		log.Printf("Failed to send reply in %s: %v", roomID, err)
	}
}
