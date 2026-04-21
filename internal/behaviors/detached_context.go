package behaviors

import (
	"fmt"
	"sync/atomic"

	"github.com/alexschlessinger/pollytool/sessions"

	"pkdindustries/soulshack/internal/irc"
)

var detachedCounter atomic.Uint64

type detachedContext struct {
	irc.ChatContextInterface
	session sessions.Session
}

func (d *detachedContext) GetSession() sessions.Session {
	return d.session
}

func newDetachedContext(ctx irc.ChatContextInterface) (*detachedContext, func(), error) {
	store := ctx.GetSystem().GetSessionStore()
	key := fmt.Sprintf("__detached_%d", detachedCounter.Add(1))
	session, err := store.Get(key)
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() { store.Delete(key) }
	return &detachedContext{ChatContextInterface: ctx, session: session}, cleanup, nil
}
