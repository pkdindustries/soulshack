// Package subagents lets the model hand a task to a child agent. Children
// always run in the background: the turn that spawns one ends as soon as the
// child has started, and the child's report arrives in the channel as a turn
// of its own whenever it finishes.
package subagents

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"pkdindustries/soulshack/internal/core"
)

// Tracker holds the children running right now. A child outlives the turn
// that spawned it, so what is running belongs to the process rather than to
// any conversation.
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

// admit records a child that is about to start, or reports why it may not.
// Counting and inserting happen together, so concurrent spawns cannot both
// pass the last free slot. The returned release forgets the child; it is safe
// to call once, from the goroutine that ran it.
//
// The limits bound spending, not permission: anyone in the channel can cause
// a spawn, so what is bounded is how much can be in flight at once. Zero or
// less is unlimited, as elsewhere in the configuration.
func (t *Tracker) admit(info core.AgentInfo, max, maxPerChat int) (func(), error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if max > 0 && len(t.running) >= max {
		return nil, fmt.Errorf("%d agents are already running, which is the limit; wait for one to report back", max)
	}
	if maxPerChat > 0 {
		running := 0
		for _, other := range t.running {
			if other.Conversation == info.Conversation {
				running++
			}
		}
		if running >= maxPerChat {
			return nil, fmt.Errorf("%d agents are already running for this conversation, which is the limit; wait for one to report back", maxPerChat)
		}
	}

	id := t.next
	t.next++
	t.running[id] = info

	var once sync.Once
	return func() {
		once.Do(func() {
			t.mu.Lock()
			defer t.mu.Unlock()
			delete(t.running, id)
		})
	}, nil
}

// Running splits what is in flight into this conversation's children and a
// count of everything else. Conversations do not see each other's transcripts,
// and a child is something someone said in one: naming another channel's work,
// or who asked for it, would carry it across a boundary the rest of the bot
// keeps. The count is left because "busy elsewhere" explains a slow reply
// without saying anything about who or what.
func Running(agents core.Agents, conversation string) (here []core.AgentInfo, elsewhere int) {
	if agents == nil {
		return nil, 0
	}
	for _, info := range agents.List() {
		if info.Conversation == conversation {
			here = append(here, info)
			continue
		}
		elsewhere++
	}
	return here, elsewhere
}

// Describe renders one running child for whoever is reading.
func Describe(info core.AgentInfo, now time.Time) string {
	return fmt.Sprintf("%s (for %s, %s)",
		info.Label,
		info.Source,
		now.Sub(info.Started).Round(time.Second),
	)
}
