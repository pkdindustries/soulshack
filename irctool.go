package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/alexschlessinger/pollytool/tools"
	"github.com/modelcontextprotocol/go-sdk/jsonschema"
)

// IRCContextualTool extends pollytool's ContextualTool for IRC-specific context
type IRCContextualTool interface {
	tools.ContextualTool
	SetIRCContext(ctx ChatContextInterface)
}

// LoadToolWithIRC wraps registry.LoadToolAuto to handle IRC tools
func LoadToolWithIRC(registry *tools.ToolRegistry, toolSpec string) error {
	// Check if it's an IRC tool
	if strings.HasPrefix(toolSpec, "irc_") {
		allTools := map[string]tools.Tool{
			"irc_op":     &IrcOpTool{},
			"irc_kick":   &IrcKickTool{},
			"irc_topic":  &IrcTopicTool{},
			"irc_action": &IrcActionTool{},
		}

		tool, exists := allTools[toolSpec]
		if !exists {
			return fmt.Errorf("unknown IRC tool: %s", toolSpec)
		}

		registry.Register(tool)
		log.Printf("registered IRC tool: %s", toolSpec)
		return nil
	}

	// Otherwise delegate to polly's LoadToolAuto
	_, err := registry.LoadToolAuto(toolSpec)
	return err
}

// IrcOpTool grants or revokes operator status
type IrcOpTool struct {
	ctx ChatContextInterface
}

func (t *IrcOpTool) SetContext(ctx any) {
	if chatCtx, ok := ctx.(ChatContextInterface); ok {
		t.ctx = chatCtx
	}
}

func (t *IrcOpTool) SetIRCContext(ctx ChatContextInterface) {
	t.ctx = ctx
}

func (t *IrcOpTool) GetSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Title:       "irc_op",
		Description: "Grant or revoke IRC operator status",
		Type:        "object",
		Properties: map[string]*jsonschema.Schema{
			"nick": {
				Type:        "string",
				Description: "The nick to op/deop",
			},
			"grant": {
				Type:        "boolean",
				Description: "true to grant op, false to revoke",
			},
		},
		Required: []string{"nick", "grant"},
	}
}

func (t *IrcOpTool) GetName() string {
	return "irc_op"
}

func (t *IrcOpTool) GetType() string {
	return "native"
}

func (t *IrcOpTool) GetSource() string {
	return "builtin"
}

func (t *IrcOpTool) Execute(ctx context.Context, args map[string]any) (string, error) {
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

func (t *IrcKickTool) SetContext(ctx any) {
	if chatCtx, ok := ctx.(ChatContextInterface); ok {
		t.ctx = chatCtx
	}
}

func (t *IrcKickTool) SetIRCContext(ctx ChatContextInterface) {
	t.ctx = ctx
}

func (t *IrcKickTool) GetSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Title:       "irc_kick",
		Description: "Kick a user from the IRC channel",
		Type:        "object",
		Properties: map[string]*jsonschema.Schema{
			"nick": {
				Type:        "string",
				Description: "The nick to kick",
			},
			"reason": {
				Type:        "string",
				Description: "The reason for kicking",
			},
		},
		Required: []string{"nick", "reason"},
	}
}

func (t *IrcKickTool) GetName() string {
	return "irc_kick"
}

func (t *IrcKickTool) GetType() string {
	return "native"
}

func (t *IrcKickTool) GetSource() string {
	return "builtin"
}

func (t *IrcKickTool) Execute(ctx context.Context, args map[string]any) (string, error) {
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

func (t *IrcTopicTool) SetContext(ctx any) {
	if chatCtx, ok := ctx.(ChatContextInterface); ok {
		t.ctx = chatCtx
	}
}

func (t *IrcTopicTool) SetIRCContext(ctx ChatContextInterface) {
	t.ctx = ctx
}

func (t *IrcTopicTool) GetSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Title:       "irc_topic",
		Description: "Set the IRC channel topic",
		Type:        "object",
		Properties: map[string]*jsonschema.Schema{
			"topic": {
				Type:        "string",
				Description: "The new topic for the channel",
			},
		},
		Required: []string{"topic"},
	}
}

func (t *IrcTopicTool) GetName() string {
	return "irc_topic"
}

func (t *IrcTopicTool) GetType() string {
	return "native"
}

func (t *IrcTopicTool) GetSource() string {
	return "builtin"
}

func (t *IrcTopicTool) Execute(ctx context.Context, args map[string]any) (string, error) {
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

func (t *IrcActionTool) SetContext(ctx any) {
	if chatCtx, ok := ctx.(ChatContextInterface); ok {
		t.ctx = chatCtx
	}
}

func (t *IrcActionTool) SetIRCContext(ctx ChatContextInterface) {
	t.ctx = ctx
}

func (t *IrcActionTool) GetSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Title:       "irc_action",
		Description: "Send an action message to the IRC channel",
		Type:        "object",
		Properties: map[string]*jsonschema.Schema{
			"message": {
				Type:        "string",
				Description: "The action message to send",
			},
		},
		Required: []string{"message"},
	}
}

func (t *IrcActionTool) GetName() string {
	return "irc_action"
}

func (t *IrcActionTool) GetType() string {
	return "native"
}

func (t *IrcActionTool) GetSource() string {
	return "builtin"
}

func (t *IrcActionTool) Execute(ctx context.Context, args map[string]any) (string, error) {
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
