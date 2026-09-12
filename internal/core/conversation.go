package core

import (
	"pkdindustries/soulshack/internal/memory"
)

// Turn is the scope of one chat turn: the IRC context plus the conversation the
// turn is working on. Turns come from WithConversation or
// WithDetachedConversation, which are the only places a conversation is handed
// out, so everything a turn touches belongs to a conversation that is still
// live.
type Turn struct {
	ChatContextInterface

	// Conversation is the transcript this turn reads and appends to.
	Conversation *memory.Conversation
}

// WithConversation runs one turn against the conversation for ctx's key,
// queueing behind any turn already running for it. onTimeout runs instead of
// run when the queue has not cleared by the time ctx ends.
func WithConversation(ctx ChatContextInterface, operation string, run func(*Turn), onTimeout func()) {
	withConversation(ctx, operation, false, run, onTimeout)
}

// WithDetachedConversation runs one turn against a scratch conversation on the
// same queue: what it writes is discarded with the turn, so an observation can
// never leak into the conversation the key owns.
func WithDetachedConversation(ctx ChatContextInterface, operation string, run func(*Turn), onTimeout func()) {
	withConversation(ctx, operation, true, run, onTimeout)
}

func withConversation(ctx ChatContextInterface, operation string, detached bool, run func(*Turn), onTimeout func()) {
	key := ctx.GetConversationKey()
	logger := ctx.GetLogger()
	mem := ctx.GetSystem().GetMemory()

	logger.Debug("turn_waiting", "conversation", key, "operation", operation)

	turn := func(conversation *memory.Conversation) {
		logger.Debug("turn_started", "conversation", key, "operation", operation)
		run(&Turn{ChatContextInterface: ctx, Conversation: conversation})
	}

	queued := mem.With
	if detached {
		queued = mem.WithDetached
	}
	if !queued(ctx, key, turn) {
		logger.Warn("turn_timeout", "conversation", key, "operation", operation)
		if onTimeout != nil {
			onTimeout()
		}
	}
}
