package core

import (
	"pkdindustries/soulshack/internal/memory"
)

// Turn is one chat turn: the IRC context plus its conversation.
// Only WithConversation and WithDetachedConversation create one.
type Turn struct {
	ChatContextInterface

	Conversation *memory.Conversation

	// Silent suppresses replies. IRC tool permissions are unaffected.
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

// WithDetachedConversation runs one turn on a discarded scratch conversation, on the same queue.
func WithDetachedConversation(ctx ChatContextInterface, operation string, run func(*Turn), onTimeout func()) {
	withConversation(ctx, operation, true, run, onTimeout)
}

func withConversation(ctx ChatContextInterface, operation string, detached bool, run func(*Turn), onTimeout func()) {
	key := ctx.GetConversationKey()
	logger := ctx.GetLogger()
	mem := ctx.GetSystem().GetMemory()

	logger.Debug("turn_waiting", "conversation", key, "operation", operation)

	turn := func(conversation *memory.Conversation) {
		logger.Debug("turn_started", "conversation", key, "operation", operation, "messages", conversation.Len())
		run(&Turn{ChatContextInterface: ctx, Conversation: conversation})
	}

	queued := mem.With
	if detached {
		queued = mem.WithDetached
	}
	if err := queued(ctx, key, turn); err != nil {
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
