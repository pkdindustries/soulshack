package main

import (
	"context"
	"fmt"
	"log"
)

// ContextualTool is a tool that needs IRC context to execute
type ContextualTool interface {
	Tool
	SetContext(ctx ChatContextInterface)
}

// GetIrcTools returns enabled IRC-related tools (context is set later via SetContext)
func GetIrcTools(enabledTools []string) []Tool {
	allTools := map[string]Tool{
		"irc_op":     &IrcOpTool{},
		"irc_kick":   &IrcKickTool{},
		"irc_topic":  &IrcTopicTool{},
		"irc_action": &IrcActionTool{},
	}
	
	// Return only enabled tools
	var tools []Tool
	for _, name := range enabledTools {
		if tool, ok := allTools[name]; ok {
			tools = append(tools, tool)
		}
	}
	return tools
}

// IrcOpTool grants or revokes operator status
type IrcOpTool struct {
	ctx ChatContextInterface
}

func (t *IrcOpTool) SetContext(ctx ChatContextInterface) {
	t.ctx = ctx
}

func (t *IrcOpTool) GetSchema() ToolSchema {
	return ToolSchema{
		Name:        "irc_op",
		Description: "Grant or revoke IRC operator status",
		Type:        "object",
		Properties: map[string]interface{}{
			"nick": map[string]interface{}{
				"type":        "string",
				"description": "The nick to op/deop",
			},
			"grant": map[string]interface{}{
				"type":        "boolean",
				"description": "true to grant op, false to revoke",
			},
		},
		Required: []string{"nick", "grant"},
	}
}

func (t *IrcOpTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	if t.ctx == nil {
		return "", fmt.Errorf("no IRC context available")
	}

	// Check admin permission
	if !t.ctx.IsAdmin() {
		return "You are not authorized to use this tool", nil
	}

	nick, ok := args["nick"].(string)
	if !ok {
		return "", fmt.Errorf("nick must be a string")
	}

	grant, ok := args["grant"].(bool)
	if !ok {
		return "", fmt.Errorf("grant must be a boolean")
	}

	mode := "-o"
	if grant {
		mode = "+o"
	}

	// Execute the IRC command
	channel := t.ctx.GetConfig().Server.Channel
	t.ctx.Mode(channel, mode, nick)

	log.Printf("IRC OP: Set mode %s for %s in %s", mode, nick, channel)
	return fmt.Sprintf("Set mode %s for %s", mode, nick), nil
}

// IrcKickTool kicks a user from the channel
type IrcKickTool struct {
	ctx ChatContextInterface
}

func (t *IrcKickTool) SetContext(ctx ChatContextInterface) {
	t.ctx = ctx
}

func (t *IrcKickTool) GetSchema() ToolSchema {
	return ToolSchema{
		Name:        "irc_kick",
		Description: "Kick a user from the IRC channel",
		Type:        "object",
		Properties: map[string]interface{}{
			"nick": map[string]interface{}{
				"type":        "string",
				"description": "The nick to kick",
			},
			"reason": map[string]interface{}{
				"type":        "string",
				"description": "The reason for kicking",
			},
		},
		Required: []string{"nick", "reason"},
	}
}

func (t *IrcKickTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	if t.ctx == nil {
		return "", fmt.Errorf("no IRC context available")
	}

	// Check admin permission
	if !t.ctx.IsAdmin() {
		return "You are not authorized to use this tool", nil
	}

	nick, ok := args["nick"].(string)
	if !ok {
		return "", fmt.Errorf("nick must be a string")
	}

	reason, ok := args["reason"].(string)
	if !ok {
		return "", fmt.Errorf("reason must be a string")
	}

	// Execute the IRC command
	channel := t.ctx.GetConfig().Server.Channel
	t.ctx.Kick(channel, nick, reason)

	log.Printf("IRC KICK: Kicked %s from %s for: %s", nick, channel, reason)
	return fmt.Sprintf("Kicked %s: %s", nick, reason), nil
}

// IrcTopicTool sets the channel topic
type IrcTopicTool struct {
	ctx ChatContextInterface
}

func (t *IrcTopicTool) SetContext(ctx ChatContextInterface) {
	t.ctx = ctx
}

func (t *IrcTopicTool) GetSchema() ToolSchema {
	return ToolSchema{
		Name:        "irc_topic",
		Description: "Set the IRC channel topic",
		Type:        "object",
		Properties: map[string]interface{}{
			"topic": map[string]interface{}{
				"type":        "string",
				"description": "The new topic for the channel",
			},
		},
		Required: []string{"topic"},
	}
}

func (t *IrcTopicTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	if t.ctx == nil {
		return "", fmt.Errorf("no IRC context available")
	}

	// Check admin permission
	if !t.ctx.IsAdmin() {
		return "You are not authorized to use this tool", nil
	}

	topic, ok := args["topic"].(string)
	if !ok {
		return "", fmt.Errorf("topic must be a string")
	}

	// Execute the IRC command
	channel := t.ctx.GetConfig().Server.Channel
	t.ctx.Topic(channel, topic)

	log.Printf("IRC TOPIC: Set topic in %s to: %s", channel, topic)
	return fmt.Sprintf("Set topic: %s", topic), nil
}

// IrcActionTool sends an action message to the channel
type IrcActionTool struct {
	ctx ChatContextInterface
}

func (t *IrcActionTool) SetContext(ctx ChatContextInterface) {
	t.ctx = ctx
}

func (t *IrcActionTool) GetSchema() ToolSchema {
	return ToolSchema{
		Name:        "irc_action",
		Description: "Send an action message to the IRC channel",
		Type:        "object",
		Properties: map[string]interface{}{
			"message": map[string]interface{}{
				"type":        "string",
				"description": "The action message to send",
			},
		},
		Required: []string{"message"},
	}
}

func (t *IrcActionTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
    if t.ctx == nil {
        return "", fmt.Errorf("no IRC context available")
    }

    message, ok := args["message"].(string)
    if !ok {
        return "", fmt.Errorf("message must be a string")
    }

    // Send IRC action directly to the configured channel
    channel := t.ctx.GetConfig().Server.Channel
    t.ctx.Action(channel, message)

    log.Printf("IRC ACTION: Sent action: %s", message)
    return fmt.Sprintf("* %s", message), nil
}
