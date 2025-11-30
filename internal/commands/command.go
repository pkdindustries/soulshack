package commands

import (
	"pkdindustries/soulshack/internal/irc"
)

// Command defines the interface for bot commands
type Command interface {
	Name() string
	Execute(ctx irc.ChatContextInterface)
	AdminOnly() bool
}

// Registry manages command registration and dispatch
type Registry struct {
	commands       map[string]Command
	defaultCommand Command
}

// NewRegistry creates a new command registry
func NewRegistry() *Registry {
	return &Registry{
		commands: make(map[string]Command),
	}
}

// Register adds a command to the registry
// Commands with empty name are registered as the default fallback
func (r *Registry) Register(cmd Command) {
	name := cmd.Name()
	if name == "" {
		r.defaultCommand = cmd
		return
	}
	r.commands[name] = cmd
}

// Get retrieves a command by name
func (r *Registry) Get(name string) (Command, bool) {
	cmd, ok := r.commands[name]
	return cmd, ok
}

// Dispatch executes the appropriate command based on context
// Returns true if a command was executed, false otherwise
func (r *Registry) Dispatch(ctx irc.ChatContextInterface) bool {
	cmdName := ctx.GetCommand()

	cmd, ok := r.commands[cmdName]
	if !ok {
		// Use default command if no match
		if r.defaultCommand != nil {
			r.defaultCommand.Execute(ctx)
			return true
		}
		return false
	}

	// Check admin permission
	if cmd.AdminOnly() && !ctx.IsAdmin() {
		ctx.Reply("You don't have permission to perform this action.")
		return true
	}

	cmd.Execute(ctx)
	return true
}

// All returns all registered commands (excluding default)
func (r *Registry) All() []Command {
	cmds := make([]Command, 0, len(r.commands))
	for _, cmd := range r.commands {
		cmds = append(cmds, cmd)
	}
	return cmds
}
