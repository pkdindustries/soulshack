package memory

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/alexschlessinger/pollytool/sessions"
)

func (m *Memory) cleanupLoop(ctx context.Context) {
	defer close(m.done)
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := m.expire(ctx); err != nil && ctx.Err() == nil {
				slog.Warn("conversation_cleanup_failed", "error", err)
			}
		}
	}
}

// expire deletes idle conversations by the current TTL, not each session's stored one.
func (m *Memory) expire(ctx context.Context) error {
	m.expiryMu.Lock()
	defer m.expiryMu.Unlock()
	ttl := m.TTL()
	if ttl <= 0 {
		return nil
	}
	summaries, err := m.store.ListSummaries(ctx)
	if err != nil {
		return err
	}
	now := m.now()
	for _, summary := range summaries {
		metadata := summary.Metadata
		if summary.InUse || strings.HasPrefix(metadata.Name, scratchPrefix) || now.Sub(metadata.LastUsed) <= ttl {
			continue
		}
		// expiryMu blocks acquisition; Delete also refuses a live lease.
		err := m.store.Delete(ctx, metadata.Name)
		if err != nil && !errors.Is(err, sessions.ErrSessionInUse) {
			return err
		}
		if err == nil {
			m.pruneIdle(metadata.Name)
		}
	}
	return nil
}
