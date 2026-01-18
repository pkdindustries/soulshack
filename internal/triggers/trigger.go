package triggers

import (
	"github.com/lrstanley/girc"

	"pkdindustries/soulshack/internal/irc"
)

// Trigger defines the interface for event-based triggers
type Trigger interface {
	Name() string
	Events() []string // IRC events to handle: girc.PRIVMSG, girc.MODE, etc.
	Check(ctx irc.ChatContextInterface, event *girc.Event) bool
	Execute(ctx irc.ChatContextInterface, event *girc.Event)
}

// Registry manages trigger registration and dispatch
type Registry struct {
	triggers map[string][]Trigger // event type -> triggers
}

// NewRegistry creates a new trigger registry
func NewRegistry() *Registry {
	return &Registry{
		triggers: make(map[string][]Trigger),
	}
}

// Register adds a trigger to the registry for all events it handles
func (r *Registry) Register(t Trigger) {
	for _, event := range t.Events() {
		r.triggers[event] = append(r.triggers[event], t)
	}
}

// Process routes an event to registered triggers, runs Check, and if true runs Execute
// Returns true if any trigger executed
func (r *Registry) Process(ctx irc.ChatContextInterface, event *girc.Event) bool {
	triggers, ok := r.triggers[event.Command]
	if !ok {
		return false
	}

	executed := false
	for _, t := range triggers {
		if t.Check(ctx, event) {
			ctx.GetLogger().Infow("trigger_executing", "trigger", t.Name())
			t.Execute(ctx, event)
			executed = true
		}
	}
	return executed
}
