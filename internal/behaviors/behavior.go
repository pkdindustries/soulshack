package behaviors

import (
	"github.com/lrstanley/girc"

	"pkdindustries/soulshack/internal/irc"
)

// Behavior defines the interface for event-based behaviors
type Behavior interface {
	Name() string
	Events() []string // IRC events to handle: girc.PRIVMSG, girc.MODE, etc.
	Check(ctx irc.ChatContextInterface, event *girc.Event) bool
	Execute(ctx irc.ChatContextInterface, event *girc.Event)
}

// Registry manages behavior registration and dispatch
type Registry struct {
	behaviors map[string][]Behavior // event type -> behaviors
}

// NewRegistry creates a new behavior registry
func NewRegistry() *Registry {
	return &Registry{
		behaviors: make(map[string][]Behavior),
	}
}

// Register adds a behavior to the registry for all events it handles
func (r *Registry) Register(b Behavior) {
	for _, event := range b.Events() {
		r.behaviors[event] = append(r.behaviors[event], b)
	}
}

// Process routes an event to registered behaviors, runs Check, and if true runs Execute
// Returns true if any behavior executed
func (r *Registry) Process(ctx irc.ChatContextInterface, event *girc.Event) bool {
	behaviors, ok := r.behaviors[event.Command]
	if !ok {
		return false
	}

	executed := false
	for _, b := range behaviors {
		if b.Check(ctx, event) {
			ctx.GetLogger().Info("behavior_executing", "behavior", b.Name())
			b.Execute(ctx, event)
			executed = true
		}
	}
	return executed
}
