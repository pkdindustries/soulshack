package behaviors

import (
	"github.com/lrstanley/girc"

	"pkdindustries/soulshack/internal/commands"
	"pkdindustries/soulshack/internal/core"
	"pkdindustries/soulshack/internal/irc"
	"pkdindustries/soulshack/internal/llm"
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

// Handles returns true if any behaviors are registered for the given event type
func (r *Registry) Handles(event string) bool {
	_, ok := r.behaviors[event]
	return ok
}

// Process routes an event to registered behaviors, runs Check, and if true runs Execute
// Returns true after the first matching behavior executes (first-match-wins)
func (r *Registry) Process(ctx irc.ChatContextInterface, event *girc.Event) bool {
	behaviors, ok := r.behaviors[event.Command]
	if !ok {
		return false
	}

	for _, b := range behaviors {
		if b.Check(ctx, event) {
			ctx.GetLogger().Info("behavior_executing", "behavior", b.Name())
			b.Execute(ctx, event)
			return true
		}
	}
	return false
}

// complete runs a completion for this turn, replying with each chunk.
func complete(turn *core.Turn, prompt string, silent bool) {
	drain(turn, llm.Complete(turn, prompt), silent)
}

// observe is complete for a turn the bot took on its own initiative rather
// than because it was asked. It cannot delegate: nobody requested this work,
// so it should not grow into more of it.
func observe(turn *core.Turn, prompt string, silent bool) {
	drain(turn, llm.CompleteWithoutDelegating(turn, prompt), silent)
}

// drain replies with each chunk unless the turn is only being recorded (a
// silent URL observation, say).
func drain(turn *core.Turn, chunks <-chan string, silent bool) {
	llm.Drain(chunks, func(chunk string) {
		if !silent {
			turn.Reply(chunk)
		}
	})
}

// dispatch runs one command turn for this message, in its own operation, and
// tells the sender when the conversation's queue is still busy.
func dispatch(ctx irc.ChatContextInterface, operation string, registry *commands.Registry) {
	core.WithConversation(ctx, operation, func(turn *core.Turn) {
		registry.Dispatch(turn)
	}, func() {
		ctx.Reply("Request timed out waiting for previous operation to complete")
	})
}
