// Package subagents runs background child agents whose reports arrive as later turns.
package subagents

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"pkdindustries/soulshack/internal/core"
)

// Tracker holds the process-wide set of running children.
type Tracker struct {
	mu      sync.Mutex
	next    uint64
	running map[uint64]core.AgentInfo
}

var _ core.Agents = (*Tracker)(nil)

// NewTracker returns an empty Tracker.
func NewTracker() *Tracker {
	return &Tracker{running: make(map[uint64]core.AgentInfo)}
}

// List returns the running children, oldest first.
func (t *Tracker) List() []core.AgentInfo {
	t.mu.Lock()
	defer t.mu.Unlock()

	agents := make([]core.AgentInfo, 0, len(t.running))
	for _, info := range t.running {
		agents = append(agents, info)
	}
	sort.Slice(agents, func(i, j int) bool { return agents[i].Started.Before(agents[j].Started) })
	return agents
}

// admit atomically checks the caps and records a child. Caps <= 0 are unlimited.
func (t *Tracker) admit(info core.AgentInfo, max, maxPerChat int) (uint64, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if max > 0 && len(t.running) >= max {
		return 0, fmt.Errorf("%d agents are already running, which is the limit; wait for one to report back", max)
	}
	if maxPerChat > 0 {
		running := 0
		for _, other := range t.running {
			if other.Conversation == info.Conversation {
				running++
			}
		}
		if running >= maxPerChat {
			return 0, fmt.Errorf("%d agents are already running for this conversation, which is the limit; wait for one to report back", maxPerChat)
		}
	}

	id := t.next
	t.next++
	t.running[id] = info
	return id, nil
}

func (t *Tracker) forget(id uint64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.running, id)
}

// Report describes the children running for conversation. Other conversations'
// children are only counted, never described.
func Report(agents core.Agents, conversation string, now time.Time) string {
	var here []string
	elsewhere := 0
	if agents != nil {
		for _, info := range agents.List() {
			if info.Conversation != conversation {
				elsewhere++
				continue
			}
			here = append(here, fmt.Sprintf("%s (for %s, %s)",
				info.Label, info.Source, now.Sub(info.Started).Round(time.Second)))
		}
	}

	others := ""
	switch {
	case elsewhere == 1:
		others = " 1 agent is working for another conversation."
	case elsewhere > 1:
		others = fmt.Sprintf(" %d agents are working for other conversations.", elsewhere)
	}

	if len(here) == 0 {
		return strings.TrimSpace("No agents are working for this conversation." + others)
	}
	return strings.TrimSpace(fmt.Sprintf("%d working: %s.%s", len(here), strings.Join(here, "; "), others))
}
