package commands

import (
	"fmt"
	"strings"

	"pkdindustries/soulshack/internal/core"
	"pkdindustries/soulshack/internal/llm"
)

// CompletionCommand handles the default chat completion
type CompletionCommand struct{}

func (c *CompletionCommand) Name() string    { return "" }
func (c *CompletionCommand) AdminOnly() bool { return false }

func (c *CompletionCommand) Execute(turn *core.Turn) {
	msg := strings.Join(turn.GetArgs(), " ")

	llm.Drain(llm.Complete(turn, fmt.Sprintf("(nick:%s) %s", turn.GetSource(), msg)), turn.Reply)
}
