package core

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/alexschlessinger/pollytool/sessions"
)

var detachedCounter atomic.Uint64

// WithConversation holds the conversation's queue lock and session lease until
// run returns, including the completion stream's final history save.
func WithConversation(ctx ChatContextInterface, operation string, run func(ChatContextInterface), onTimeout func()) {
	withConversation(ctx, operation, false, run, onTimeout)
}

// WithDetachedConversation uses the same request queue but a temporary session.
// Its transcript is deleted after the lease closes, even if run is canceled.
func WithDetachedConversation(ctx ChatContextInterface, operation string, run func(ChatContextInterface), onTimeout func()) {
	withConversation(ctx, operation, true, run, onTimeout)
}

func withConversation(ctx ChatContextInterface, operation string, detached bool, run func(ChatContextInterface), onTimeout func()) {
	WithRequestLock(ctx, ctx.GetLockKey(), operation, func() {
		if ctx.Err() != nil {
			if onTimeout != nil {
				onTimeout()
			}
			return
		}
		key := ctx.GetLockKey()
		if detached {
			key = fmt.Sprintf("__detached_%d", detachedCounter.Add(1))
		}
		store := ctx.GetSystem().GetSessions()
		session, err := store.Acquire(ctx, key, sessions.AcquireOptions{})
		if err != nil {
			ctx.GetLogger().Error("session_acquire_failed", "operation", operation, "error", err)
			if !detached {
				ctx.Reply("Failed to open conversation")
			}
			return
		}
		defer func() {
			if err := session.Close(); err != nil {
				ctx.GetLogger().Warn("session_close_failed", "error", err)
			}
			if detached {
				cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if err := store.Delete(cleanupCtx, key); err != nil {
					ctx.GetLogger().Warn("detached_session_delete_failed", "key", key, "error", err)
				}
			}
		}()

		turn, cancel := context.WithCancelCause(ctx)
		stop := context.AfterFunc(session.Context(), func() { cancel(context.Cause(session.Context())) })
		defer func() { stop(); cancel(nil) }()
		if session.Context().Err() != nil {
			cancel(context.Cause(session.Context()))
		}
		if turn.Err() == nil {
			run(&conversationContext{ChatContextInterface: ctx, turn: turn, session: session})
		}
	}, onTimeout)
}

type conversationContext struct {
	ChatContextInterface
	turn    context.Context
	session sessions.Session
}

func (c *conversationContext) GetSession() sessions.Session { return c.session }
func (c *conversationContext) Deadline() (time.Time, bool)  { return c.turn.Deadline() }
func (c *conversationContext) Done() <-chan struct{}        { return c.turn.Done() }
func (c *conversationContext) Err() error                   { return c.turn.Err() }
func (c *conversationContext) Value(key any) any            { return c.turn.Value(key) }
