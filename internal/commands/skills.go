package commands

import (
	"fmt"
	"strings"

	"pkdindustries/soulshack/internal/irc"
)

// SkillsCommand handles the /skills command for listing available skills
type SkillsCommand struct{}

func (c *SkillsCommand) Name() string    { return "/skills" }
func (c *SkillsCommand) AdminOnly() bool { return false }

func (c *SkillsCommand) Execute(ctx irc.ChatContextInterface) {
	catalog := ctx.GetSystem().GetSkillCatalog()
	if catalog == nil || catalog.IsEmpty() {
		ctx.Reply("No skills available")
		return
	}

	var parts []string
	for _, skill := range catalog.List() {
		parts = append(parts, fmt.Sprintf("%s - %s", skill.Name, skill.Description))
	}

	message := strings.Join(parts, " | ")
	maxLen := ctx.GetConfig().Session.ChunkMax
	if maxLen <= 0 {
		maxLen = 350
	}
	if len(message) > maxLen {
		message = message[:maxLen-3] + "..."
	}
	ctx.Reply(message)
}
