package irc

import (
	"context"
	"fmt"
	"pkdindustries/soulshack/internal/core"
	"strings"

	"github.com/alexschlessinger/pollytool/tools"
	"github.com/google/jsonschema-go/jsonschema"
)

// IRCContextualTool extends pollytool's ContextualTool for IRC-specific context
type IRCContextualTool interface {
	tools.ContextualTool
}

type contextKey string

const kContextKey contextKey = "irc_context"

func GetIRCContext(ctx context.Context) (core.ChatContextInterface, error) {
	if chatCtx, ok := ctx.Value(kContextKey).(core.ChatContextInterface); ok {
		return chatCtx, nil
	}
	return nil, fmt.Errorf("no IRC context available")
}

// isBotOpped checks if the bot has operator status in the channel
func isBotOpped(ctx core.ChatContextInterface) bool {
	channel := ctx.GetConfig().Server.Channel
	client := ctx.GetClient()
	botNick := client.GetNick()

	ch := ctx.LookupChannel(channel)
	if ch == nil {
		return false
	}

	admins := ch.Admins(client)
	for _, admin := range admins {
		if admin.Nick == botNick {
			return true
		}
	}
	return false
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
	registry.RegisterNative("irc_history", func() tools.Tool {
		return &IrcHistoryTool{}
	})
}

// IrcOpTool grants or revokes operator status
type IrcOpTool struct {
}

func (t *IrcOpTool) SetContext(ctx any) {
}

func (t *IrcOpTool) GetSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Title:       "irc_op",
		Description: "Grant or revoke IRC operator status for one or more users",
		Type:        "object",
		Properties: map[string]*jsonschema.Schema{
			"users": {
				Type:        "array",
				Description: "List of user nicknames to op/deop",
				Items: &jsonschema.Schema{
					Type: "string",
				},
			},
			"grant": {
				Type:        "boolean",
				Description: "true to grant op, false to revoke",
			},
		},
		Required: []string{"users", "grant"},
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
	chatCtx, err := GetIRCContext(ctx)
	if err != nil {
		return "", err
	}

	// Check admin permission
	if !chatCtx.IsAdmin() {
		return "You are not authorized to use this tool", nil
	}

	// Check if bot has operator status
	if !isBotOpped(chatCtx) {
		return "Bot does not have operator status in the channel", nil
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

	grant, ok := args["grant"].(bool)
	if !ok {
		return "", fmt.Errorf("grant must be a boolean")
	}

	mode := "-o"
	if grant {
		mode = "+o"
	}

	// Execute the IRC command for each user
	channel := chatCtx.GetConfig().Server.Channel
	for _, nick := range users {
		chatCtx.Mode(channel, mode, nick)
	}

	usersStr := strings.Join(users, ", ")
	chatCtx.GetLogger().Infof("IRC OP: Set mode %s for %s in %s", mode, usersStr, channel)
	return fmt.Sprintf("Set mode %s for %s", mode, usersStr), nil
}

// IrcKickTool kicks a user from the channel
type IrcKickTool struct {
}

func (t *IrcKickTool) SetContext(ctx any) {
}

func (t *IrcKickTool) GetSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Title:       "irc_kick",
		Description: "Kick one or more users from the IRC channel",
		Type:        "object",
		Properties: map[string]*jsonschema.Schema{
			"users": {
				Type:        "array",
				Description: "List of user nicknames to kick",
				Items: &jsonschema.Schema{
					Type: "string",
				},
			},
			"reason": {
				Type:        "string",
				Description: "The reason for kicking",
			},
		},
		Required: []string{"users", "reason"},
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
	chatCtx, err := GetIRCContext(ctx)
	if err != nil {
		return "", err
	}

	// Check admin permission
	if !chatCtx.IsAdmin() {
		return "You are not authorized to use this tool", nil
	}

	// Check if bot has operator status
	if !isBotOpped(chatCtx) {
		return "Bot does not have operator status in the channel", nil
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

	reason, ok := args["reason"].(string)
	if !ok {
		return "", fmt.Errorf("reason must be a string")
	}

	// Execute the IRC command for each user
	channel := chatCtx.GetConfig().Server.Channel
	for _, nick := range users {
		chatCtx.Kick(channel, nick, reason)
	}

	usersStr := strings.Join(users, ", ")
	chatCtx.GetLogger().Infow("IRC KICK: Kicked user", "users", usersStr, "channel", channel, "reason", reason)
	return fmt.Sprintf("Kicked %s: %s", usersStr, reason), nil
}

// IrcBanTool bans or unbans a user from the channel
type IrcBanTool struct {
}

func (t *IrcBanTool) SetContext(ctx any) {
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
	chatCtx, err := GetIRCContext(ctx)
	if err != nil {
		return "", err
	}

	// Check admin permission
	if !chatCtx.IsAdmin() {
		return "You are not authorized to use this tool", nil
	}

	// Check if bot has operator status
	if !isBotOpped(chatCtx) {
		return "Bot does not have operator status in the channel", nil
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
		if ident, host, found := chatCtx.LookupUser(target); found {
			// Create a ban mask that bans *!ident@host
			// This is more specific than *!*@host and prevents banning other users on the same host
			banMask = fmt.Sprintf("*!%s@%s", ident, host)
			chatCtx.GetLogger().Infow("IRC BAN: Found user", "target", target, "ident", ident, "host", host, "ban_mask", banMask)
		} else {
			// User not found in channel, use simple pattern
			banMask = target + "!*@*"
			chatCtx.GetLogger().Infow("IRC BAN: User not found in channel", "target", target, "ban_mask", banMask)
		}
	}

	// Execute the IRC command using girc's dedicated Ban/Unban methods
	channel := chatCtx.GetConfig().Server.Channel
	client := chatCtx.GetClient()
	if ban {
		client.Cmd.Ban(channel, banMask)
	} else {
		client.Cmd.Unban(channel, banMask)
	}

	chatCtx.GetLogger().Infow("IRC BAN: Action completed", "action", action, "ban_mask", banMask, "channel", channel)
	return fmt.Sprintf("%s %s", action, banMask), nil
}

// IrcTopicTool sets the channel topic
type IrcTopicTool struct {
}

func (t *IrcTopicTool) SetContext(ctx any) {
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
	chatCtx, err := GetIRCContext(ctx)
	if err != nil {
		return "", err
	}

	// Check admin permission
	if !chatCtx.IsAdmin() {
		return "You are not authorized to use this tool", nil
	}

	// Check if bot has operator status
	if !isBotOpped(chatCtx) {
		return "Bot does not have operator status in the channel", nil
	}

	topic, ok := args["topic"].(string)
	if !ok {
		return "", fmt.Errorf("topic must be a string")
	}

	// Execute the IRC command
	channel := chatCtx.GetConfig().Server.Channel
	chatCtx.Topic(channel, topic)

	chatCtx.GetLogger().Infow("IRC TOPIC: Set topic", "channel", channel, "topic", topic)
	return fmt.Sprintf("Set topic: %s", topic), nil
}

// IrcActionTool sends an action message to the channel
type IrcActionTool struct {
}

func (t *IrcActionTool) SetContext(ctx any) {
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
	chatCtx, err := GetIRCContext(ctx)
	if err != nil {
		return "", err
	}

	message, ok := args["message"].(string)
	if !ok {
		return "", fmt.Errorf("message must be a string")
	}

	// Send IRC action directly to the configured channel
	channel := chatCtx.GetConfig().Server.Channel
	chatCtx.Action(channel, message)

	chatCtx.GetLogger().Infow("IRC ACTION: Sent action", "message", message)
	return fmt.Sprintf("* %s", message), nil
}

// IrcModeSetTool sets channel-wide modes
type IrcModeSetTool struct {
}

func (t *IrcModeSetTool) SetContext(ctx any) {
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
	chatCtx, err := GetIRCContext(ctx)
	if err != nil {
		return "", err
	}

	// Check admin permission
	if !chatCtx.IsAdmin() {
		return "You are not authorized to use this tool", nil
	}

	// Check if bot has operator status
	if !isBotOpped(chatCtx) {
		return "Bot does not have operator status in the channel", nil
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
	channel := chatCtx.GetConfig().Server.Channel
	client := chatCtx.GetClient()
	if len(modeParams) > 0 {
		client.Cmd.Mode(channel, modeFlags, modeParams...)
	} else {
		client.Cmd.Mode(channel, modeFlags)
	}

	chatCtx.GetLogger().Infow("IRC MODE SET: Set modes", "modes", modes, "channel", channel)
	return fmt.Sprintf("Set channel mode %s on %s", modes, channel), nil
}

// IrcModeQueryTool queries current channel modes
type IrcModeQueryTool struct {
}

func (t *IrcModeQueryTool) SetContext(ctx any) {
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
	chatCtx, err := GetIRCContext(ctx)
	if err != nil {
		return "", err
	}

	channel := chatCtx.GetConfig().Server.Channel
	ch := chatCtx.LookupChannel(channel)

	if ch == nil {
		return fmt.Sprintf("Channel %s not found in state (not joined yet?)", channel), nil
	}

	// Get modes as string from girc's CModes type
	modeStr := ch.Modes.String()

	if modeStr == "" {
		return fmt.Sprintf("Channel %s has no modes set", channel), nil
	}

	chatCtx.GetLogger().Infow("IRC MODE QUERY: Channel modes", "channel", channel, "modes", modeStr)
	return fmt.Sprintf("Channel %s modes: %s", channel, modeStr), nil
}

// IrcInviteTool invites users to the channel
type IrcInviteTool struct {
}

func (t *IrcInviteTool) SetContext(ctx any) {
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
	chatCtx, err := GetIRCContext(ctx)
	if err != nil {
		return "", err
	}

	// Check admin permission
	if !chatCtx.IsAdmin() {
		return "You are not authorized to use this tool", nil
	}

	// Check if bot has operator status
	if !isBotOpped(chatCtx) {
		return "Bot does not have operator status in the channel", nil
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
	channel := chatCtx.GetConfig().Server.Channel
	client := chatCtx.GetClient()
	client.Cmd.Invite(channel, users...)

	usersStr := strings.Join(users, ", ")
	chatCtx.GetLogger().Infow("IRC INVITE: Invited users", "users", usersStr, "channel", channel)
	return fmt.Sprintf("Invited %s to %s", usersStr, channel), nil
}

// IrcNamesTool lists all users in the channel
type IrcNamesTool struct {
}

func (t *IrcNamesTool) SetContext(ctx any) {
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
	chatCtx, err := GetIRCContext(ctx)
	if err != nil {
		return "", err
	}

	channel := chatCtx.GetConfig().Server.Channel
	ch := chatCtx.LookupChannel(channel)

	if ch == nil {
		return fmt.Sprintf("Channel %s not found in state (not joined yet?)", channel), nil
	}

	client := chatCtx.GetClient()
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
	chatCtx.GetLogger().Infow("IRC NAMES: List users", "channel", channel, "count", len(users), "nicks", nicksStr)
	return fmt.Sprintf("Users in %s (%d): %s", channel, len(users), nicksStr), nil
}

// IrcWhoisTool gets detailed information about a user
type IrcWhoisTool struct {
	ctx core.ChatContextInterface
}

func (t *IrcWhoisTool) SetContext(ctx any) {
	if chatCtx, ok := ctx.(core.ChatContextInterface); ok {
		t.ctx = chatCtx
	}
}

func (t *IrcWhoisTool) SetIRCContext(ctx core.ChatContextInterface) {
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

	// Get the chat context for logging
	chatCtx, err := GetIRCContext(ctx)
	if err == nil {
		chatCtx.GetLogger().Infow("IRC WHOIS result", "nick", nick)
	}

	return result, nil
}

// IrcHistoryTool retrieves chat history for a channel
type IrcHistoryTool struct {
}

func (t *IrcHistoryTool) SetContext(ctx any) {
}

func (t *IrcHistoryTool) GetSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Title:       "irc_history",
		Description: "Get recent chat history for a specific channel. This accesses the full channel history logs, which is distinct from the current chat session context.",
		Type:        "object",
		Properties: map[string]*jsonschema.Schema{
			"channel": {
				Type:        "string",
				Description: "The channel or user to get history for. Defaults to the current channel/user if omitted.",
			},
			"limit": {
				Type:        "integer",
				Description: "Number of messages to retrieve (default 50)",
			},
			"search": {
				Type:        "string",
				Description: "Filter history by this search term (optional, case-insensitive. if user use just the nick, no brackets or braces)",
			},
		},
		// channel is now optional, defaults to current context
		Required: []string{},
	}
}

func (t *IrcHistoryTool) GetName() string {
	return "irc_history"
}

func (t *IrcHistoryTool) GetType() string {
	return "native"
}

func (t *IrcHistoryTool) GetSource() string {
	return "builtin"
}

func (t *IrcHistoryTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	chatCtx, err := GetIRCContext(ctx)
	if err != nil {
		return "", err
	}

	channel := ""
	if c, ok := args["channel"].(string); ok && c != "" {
		channel = c
	} else {
		// Default to current context
		if chatCtx.IsPrivate() {
			// For PMs, history is stored under the sender's nick
			channel = chatCtx.GetSource()
		} else {
			// For channels, use the configured channel
			channel = chatCtx.GetConfig().Server.Channel
		}
	}

	filter := core.HistoryFilter{
		Limit: 50, // Default limit
	}

	if l, ok := args["limit"].(float64); ok {
		filter.Limit = int(l)
	}

	if s, ok := args["search"].(string); ok {
		filter.Search = s
	}

	historyStore := chatCtx.GetSystem().GetHistory()
	if historyStore == nil {
		return "History storage is not available", nil
	}

	messages, _, err := historyStore.Get(channel, filter)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve history: %v", err)
	}

	if len(messages) == 0 {
		return fmt.Sprintf("No history found for %s", channel), nil
	}

	return strings.Join(messages, "\n"), nil
}
