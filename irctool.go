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

// RegisterIRCTools registers IRC tools as native tools with polly's registry
func RegisterIRCTools(registry *tools.ToolRegistry) {
	registry.RegisterNative("irc_op", func() tools.Tool {
		return &IrcOpTool{}
	})
	registry.RegisterNative("irc_kick", func() tools.Tool {
		return &IrcKickTool{}
	})
	registry.RegisterNative("irc_ban", func() tools.Tool {
		return &IrcBanTool{}
	})
	registry.RegisterNative("irc_topic", func() tools.Tool {
		return &IrcTopicTool{}
	})
	registry.RegisterNative("irc_action", func() tools.Tool {
		return &IrcActionTool{}
	})
	registry.RegisterNative("irc_mode_set", func() tools.Tool {
		return &IrcModeSetTool{}
	})
	registry.RegisterNative("irc_mode_query", func() tools.Tool {
		return &IrcModeQueryTool{}
	})
	registry.RegisterNative("irc_invite", func() tools.Tool {
		return &IrcInviteTool{}
	})
	registry.RegisterNative("irc_names", func() tools.Tool {
		return &IrcNamesTool{}
	})
	registry.RegisterNative("irc_whois", func() tools.Tool {
		return &IrcWhoisTool{}
	})
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

// IrcBanTool bans or unbans a user from the channel
type IrcBanTool struct {
	ctx ChatContextInterface
}

func (t *IrcBanTool) SetContext(ctx any) {
	if chatCtx, ok := ctx.(ChatContextInterface); ok {
		t.ctx = chatCtx
	}
}

func (t *IrcBanTool) SetIRCContext(ctx ChatContextInterface) {
	t.ctx = ctx
}

func (t *IrcBanTool) GetSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Title:       "irc_ban",
		Description: "Ban or unban a user from the IRC channel",
		Type:        "object",
		Properties: map[string]*jsonschema.Schema{
			"target": {
				Type:        "string",
				Description: "The nick or hostmask to ban/unban",
			},
			"ban": {
				Type:        "boolean",
				Description: "true to ban, false to unban",
			},
		},
		Required: []string{"target", "ban"},
	}
}

func (t *IrcBanTool) GetName() string {
	return "irc_ban"
}

func (t *IrcBanTool) GetType() string {
	return "native"
}

func (t *IrcBanTool) GetSource() string {
	return "builtin"
}

func (t *IrcBanTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	if t.ctx == nil {
		return "", fmt.Errorf("no IRC context available")
	}

	// Check admin permission
	if !t.ctx.IsAdmin() {
		return "You are not authorized to use this tool", nil
	}

	target, ok := args["target"].(string)
	if !ok {
		return "", fmt.Errorf("target must be a string")
	}

	ban, ok := args["ban"].(bool)
	if !ok {
		return "", fmt.Errorf("ban must be a boolean")
	}

	action := "Unbanned"
	if ban {
		action = "Banned"
	}

	// If target doesn't contain wildcards or !, assume it's a nick and convert to hostmask
	banMask := target
	if !strings.Contains(target, "!") && !strings.Contains(target, "*") {
		// Try to look up the user to get their actual ident and host
		if ident, host, found := t.ctx.LookupUser(target); found {
			// Create a ban mask that bans *!ident@host
			// This is more specific than *!*@host and prevents banning other users on the same host
			banMask = fmt.Sprintf("*!%s@%s", ident, host)
			log.Printf("IRC BAN: Found user %s with ident=%s host=%s, using ban mask %s", target, ident, host, banMask)
		} else {
			// User not found in channel, use simple pattern
			banMask = target + "!*@*"
			log.Printf("IRC BAN: User %s not found in channel, using simple pattern %s", target, banMask)
		}
	}

	// Execute the IRC command using girc's dedicated Ban/Unban methods
	channel := t.ctx.GetConfig().Server.Channel
	client := t.ctx.GetClient()
	if ban {
		client.Cmd.Ban(channel, banMask)
	} else {
		client.Cmd.Unban(channel, banMask)
	}

	log.Printf("IRC BAN: %s %s in %s", action, banMask, channel)
	return fmt.Sprintf("%s %s", action, banMask), nil
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

// IrcModeSetTool sets channel-wide modes
type IrcModeSetTool struct {
	ctx ChatContextInterface
}

func (t *IrcModeSetTool) SetContext(ctx any) {
	if chatCtx, ok := ctx.(ChatContextInterface); ok {
		t.ctx = chatCtx
	}
}

func (t *IrcModeSetTool) SetIRCContext(ctx ChatContextInterface) {
	t.ctx = ctx
}

func (t *IrcModeSetTool) GetSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Title:       "irc_mode_set",
		Description: "Set or unset channel-wide modes like +m (moderated), +t (topic protection), +n (no external messages), +i (invite only), +k (channel key), +l (user limit)",
		Type:        "object",
		Properties: map[string]*jsonschema.Schema{
			"modes": {
				Type:        "string",
				Description: "The mode string to set, e.g., '+m', '-t', '+mnt', '+k password', '+l 50'",
			},
		},
		Required: []string{"modes"},
	}
}

func (t *IrcModeSetTool) GetName() string {
	return "irc_mode_set"
}

func (t *IrcModeSetTool) GetType() string {
	return "native"
}

func (t *IrcModeSetTool) GetSource() string {
	return "builtin"
}

func (t *IrcModeSetTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	if t.ctx == nil {
		return "", fmt.Errorf("no IRC context available")
	}

	// Check admin permission
	if !t.ctx.IsAdmin() {
		return "You are not authorized to use this tool", nil
	}

	modes, ok := args["modes"].(string)
	if !ok {
		return "", fmt.Errorf("modes must be a string")
	}

	// Parse mode string - first part is the mode flags, rest are parameters
	parts := strings.Fields(modes)
	if len(parts) == 0 {
		return "", fmt.Errorf("modes string cannot be empty")
	}

	modeFlags := parts[0]
	modeParams := parts[1:]

	// Execute the IRC MODE command for channel-wide modes
	channel := t.ctx.GetConfig().Server.Channel
	client := t.ctx.GetClient()
	if len(modeParams) > 0 {
		client.Cmd.Mode(channel, modeFlags, modeParams...)
	} else {
		client.Cmd.Mode(channel, modeFlags)
	}

	log.Printf("IRC MODE SET: Set modes %s on %s", modes, channel)
	return fmt.Sprintf("Set channel mode %s on %s", modes, channel), nil
}

// IrcModeQueryTool queries current channel modes
type IrcModeQueryTool struct {
	ctx ChatContextInterface
}

func (t *IrcModeQueryTool) SetContext(ctx any) {
	if chatCtx, ok := ctx.(ChatContextInterface); ok {
		t.ctx = chatCtx
	}
}

func (t *IrcModeQueryTool) SetIRCContext(ctx ChatContextInterface) {
	t.ctx = ctx
}

func (t *IrcModeQueryTool) GetSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Title:       "irc_mode_query",
		Description: "Query the current channel modes (uses cached state, instant response)",
		Type:        "object",
		Properties:  map[string]*jsonschema.Schema{},
		Required:    []string{},
	}
}

func (t *IrcModeQueryTool) GetName() string {
	return "irc_mode_query"
}

func (t *IrcModeQueryTool) GetType() string {
	return "native"
}

func (t *IrcModeQueryTool) GetSource() string {
	return "builtin"
}

func (t *IrcModeQueryTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	if t.ctx == nil {
		return "", fmt.Errorf("no IRC context available")
	}

	channel := t.ctx.GetConfig().Server.Channel
	ch := t.ctx.LookupChannel(channel)

	if ch == nil {
		return fmt.Sprintf("Channel %s not found in state (not joined yet?)", channel), nil
	}

	// Get modes as string from girc's CModes type
	modeStr := ch.Modes.String()

	if modeStr == "" {
		return fmt.Sprintf("Channel %s has no modes set", channel), nil
	}

	log.Printf("IRC MODE QUERY: Channel %s modes: %s", channel, modeStr)
	return fmt.Sprintf("Channel %s modes: %s", channel, modeStr), nil
}

// IrcInviteTool invites users to the channel
type IrcInviteTool struct {
	ctx ChatContextInterface
}

func (t *IrcInviteTool) SetContext(ctx any) {
	if chatCtx, ok := ctx.(ChatContextInterface); ok {
		t.ctx = chatCtx
	}
}

func (t *IrcInviteTool) SetIRCContext(ctx ChatContextInterface) {
	t.ctx = ctx
}

func (t *IrcInviteTool) GetSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Title:       "irc_invite",
		Description: "Invite one or more users to the IRC channel",
		Type:        "object",
		Properties: map[string]*jsonschema.Schema{
			"users": {
				Type:        "array",
				Description: "List of user nicknames to invite",
				Items: &jsonschema.Schema{
					Type: "string",
				},
			},
		},
		Required: []string{"users"},
	}
}

func (t *IrcInviteTool) GetName() string {
	return "irc_invite"
}

func (t *IrcInviteTool) GetType() string {
	return "native"
}

func (t *IrcInviteTool) GetSource() string {
	return "builtin"
}

func (t *IrcInviteTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	if t.ctx == nil {
		return "", fmt.Errorf("no IRC context available")
	}

	// Check admin permission
	if !t.ctx.IsAdmin() {
		return "You are not authorized to use this tool", nil
	}

	usersRaw, ok := args["users"].([]any)
	if !ok {
		return "", fmt.Errorf("users must be an array")
	}

	if len(usersRaw) == 0 {
		return "", fmt.Errorf("users array cannot be empty")
	}

	// Convert []any to []string
	users := make([]string, len(usersRaw))
	for i, u := range usersRaw {
		user, ok := u.(string)
		if !ok {
			return "", fmt.Errorf("all users must be strings")
		}
		users[i] = user
	}

	// Execute the IRC INVITE command
	channel := t.ctx.GetConfig().Server.Channel
	client := t.ctx.GetClient()
	client.Cmd.Invite(channel, users...)

	usersStr := strings.Join(users, ", ")
	log.Printf("IRC INVITE: Invited %s to %s", usersStr, channel)
	return fmt.Sprintf("Invited %s to %s", usersStr, channel), nil
}

// IrcNamesTool lists all users in the channel
type IrcNamesTool struct {
	ctx ChatContextInterface
}

func (t *IrcNamesTool) SetContext(ctx any) {
	if chatCtx, ok := ctx.(ChatContextInterface); ok {
		t.ctx = chatCtx
	}
}

func (t *IrcNamesTool) SetIRCContext(ctx ChatContextInterface) {
	t.ctx = ctx
}

func (t *IrcNamesTool) GetSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Title:       "irc_names",
		Description: "List all users currently in the IRC channel (uses cached state, instant response)",
		Type:        "object",
		Properties:  map[string]*jsonschema.Schema{},
		Required:    []string{},
	}
}

func (t *IrcNamesTool) GetName() string {
	return "irc_names"
}

func (t *IrcNamesTool) GetType() string {
	return "native"
}

func (t *IrcNamesTool) GetSource() string {
	return "builtin"
}

func (t *IrcNamesTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	if t.ctx == nil {
		return "", fmt.Errorf("no IRC context available")
	}

	channel := t.ctx.GetConfig().Server.Channel
	ch := t.ctx.LookupChannel(channel)

	if ch == nil {
		return fmt.Sprintf("Channel %s not found in state (not joined yet?)", channel), nil
	}

	client := t.ctx.GetClient()
	users := ch.Users(client)

	if len(users) == 0 {
		return fmt.Sprintf("No users found in %s", channel), nil
	}

	// Get lists of admins and trusted users for prefix determination
	admins := ch.Admins(client)
	trusted := ch.Trusted(client)

	// Create maps for quick lookup
	adminMap := make(map[string]bool)
	for _, admin := range admins {
		adminMap[admin.Nick] = true
	}
	trustedMap := make(map[string]bool)
	for _, t := range trusted {
		trustedMap[t.Nick] = true
	}

	// Build list of nicks with their prefixes (@, +, etc.)
	var nicks []string
	for _, user := range users {
		prefix := ""
		if adminMap[user.Nick] {
			prefix = "@"
		} else if trustedMap[user.Nick] {
			prefix = "+"
		}
		nicks = append(nicks, prefix+user.Nick)
	}

	nicksStr := strings.Join(nicks, ", ")
	log.Printf("IRC NAMES: %s (%d users): %s", channel, len(users), nicksStr)
	return fmt.Sprintf("Users in %s (%d): %s", channel, len(users), nicksStr), nil
}

// IrcWhoisTool gets detailed information about a user
type IrcWhoisTool struct {
	ctx ChatContextInterface
}

func (t *IrcWhoisTool) SetContext(ctx any) {
	if chatCtx, ok := ctx.(ChatContextInterface); ok {
		t.ctx = chatCtx
	}
}

func (t *IrcWhoisTool) SetIRCContext(ctx ChatContextInterface) {
	t.ctx = ctx
}

func (t *IrcWhoisTool) GetSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Title:       "irc_whois",
		Description: "Get detailed information about a user (uses cached state, instant response)",
		Type:        "object",
		Properties: map[string]*jsonschema.Schema{
			"nick": {
				Type:        "string",
				Description: "The nickname to look up",
			},
		},
		Required: []string{"nick"},
	}
}

func (t *IrcWhoisTool) GetName() string {
	return "irc_whois"
}

func (t *IrcWhoisTool) GetType() string {
	return "native"
}

func (t *IrcWhoisTool) GetSource() string {
	return "builtin"
}

func (t *IrcWhoisTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	if t.ctx == nil {
		return "", fmt.Errorf("no IRC context available")
	}

	nick, ok := args["nick"].(string)
	if !ok {
		return "", fmt.Errorf("nick must be a string")
	}

	client := t.ctx.GetClient()
	user := client.LookupUser(nick)

	if user == nil {
		return fmt.Sprintf("User %s not found in cached state", nick), nil
	}

	// Build detailed info from cached state
	var info strings.Builder
	info.WriteString(fmt.Sprintf("User: %s\n", user.Nick))
	info.WriteString(fmt.Sprintf("Hostmask: %s!%s@%s\n", user.Nick, user.Ident, user.Host))

	// Add extras if available
	if user.Extras.Name != "" {
		info.WriteString(fmt.Sprintf("Real name: %s\n", user.Extras.Name))
	}
	if user.Extras.Account != "" {
		info.WriteString(fmt.Sprintf("Account: %s (authenticated)\n", user.Extras.Account))
	}
	if user.Extras.Away != "" {
		info.WriteString(fmt.Sprintf("Away: %s\n", user.Extras.Away))
	}

	// List channels
	channels := user.ChannelList
	if len(channels) > 0 {
		info.WriteString(fmt.Sprintf("Channels (%d): %s\n", len(channels), strings.Join(channels, ", ")))
	}

	result := strings.TrimSpace(info.String())
	log.Printf("IRC WHOIS: %s", result)
	return result, nil
}
