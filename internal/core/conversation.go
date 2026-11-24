package core

import (
	"pkdindustries/soulshack/internal/memory"
)

// Turn is one chat turn: the IRC context plus its conversation.
// Only WithConversation and WithDetachedConversation create one.
type Turn struct {
	ChatContextInterface

	Conversation *memory.Conversation

	// Silent suppresses replies, announcements, and channel-acting IRC tools.
	Silent bool
}

func (t *Turn) Reply(message string) {
	if !t.Silent {
		t.ChatContextInterface.Reply(message)
	}
}

func (t *Turn) ReplyAction(message string) {
	if !t.Silent {
		t.ChatContextInterface.ReplyAction(message)
	}
}

// WithConversation runs one turn on ctx's conversation, queued behind any running turn.
// onTimeout runs instead if ctx ends first or the conversation can't be opened.
func WithConversation(ctx ChatContextInterface, operation string, run func(*Turn), onTimeout func()) {
	withConversation(ctx, operation, false, run, onTimeout)
}

// WithDetachedConversation runs one turn on an unqueued, discarded scratch conversation.
func WithDetachedConversation(ctx ChatContextInterface, operation string, run func(*Turn), onTimeout func()) {
	withConversation(ctx, operation, true, run, onTimeout)
}

func withConversation(ctx ChatContextInterface, operation string, detached bool, run func(*Turn), onTimeout func()) {
	key := ctx.GetConversationKey()
	logger := ctx.GetLogger()
	mem := ctx.GetSystem().GetMemory()

	turn := func(conversation *memory.Conversation) {
		logger.Debug("turn_started", "conversation", key, "operation", operation, "messages", conversation.Len())
		run(&Turn{ChatContextInterface: ctx, Conversation: conversation})
	}

	var err error
	if detached {
		err = mem.WithScratch(ctx, turn)
	} else {
		logger.Debug("turn_waiting", "conversation", key, "operation", operation)
		err = mem.With(ctx, key, turn)
	}
	if err != nil {
		if ctx.Err() != nil {
			logger.Warn("turn_timeout", "conversation", key, "operation", operation)
		} else {
			logger.Error("turn_failed", "conversation", key, "operation", operation, "error", err)
		}
		if onTimeout != nil {
			onTimeout()
		}
	}
}
