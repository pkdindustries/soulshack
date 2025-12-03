package irc

import (
	"context"
	"fmt"
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

func GetIRCContext(ctx context.Context) (ChatContextInterface, error) {
	if chatCtx, ok := ctx.Value(kContextKey).(ChatContextInterface); ok {
		return chatCtx, nil
	}
	return nil, fmt.Errorf("no IRC context available")
}

// InjectContext stores the IRC context for tools to retrieve.
// This must be used (rather than direct context.WithValue) to ensure
// the correct key type is used.
func InjectContext(ctx context.Context, chatCtx ChatContextInterface) context.Context {
	return context.WithValue(ctx, kContextKey, chatCtx)
}

// isBotOpped checks if the bot has operator status in the channel
func isBotOpped(ctx ChatContextInterface) bool {
	channel := ctx.GetConfig().Server.Channel
	botNick := ctx.GetBotNick()

	users := ctx.GetChannelUsers(channel)
	if users == nil {
		return false
	}

	for _, user := range users {
		if user.Nick == botNick {
			return user.IsOp
		}
	}
	return false
}

// BaseIRCTool provides common functionality for all IRC tools
type BaseIRCTool struct{}

func (t *BaseIRCTool) SetContext(ctx any) {}
func (t *BaseIRCTool) GetType() string    { return "native" }
func (t *BaseIRCTool) GetSource() string  { return "builtin" }

// validateAdminOp validates admin permissions and bot op status
func validateAdminOp(ctx context.Context) (ChatContextInterface, string, error) {
	chatCtx, err := GetIRCContext(ctx)
	if err != nil {
		return nil, "", err
	}
	if err := ctx.Err(); err != nil {
		return nil, "", err
	}
	if !chatCtx.IsAdmin() {
		return nil, "You are not authorized to use this tool", nil
	}
	if !isBotOpped(chatCtx) {
		return nil, "Bot does not have operator status in the channel", nil
	}
	return chatCtx, "", nil
}

// validateContext validates context without requiring admin/op status
func validateContext(ctx context.Context) (ChatContextInterface, error) {
	chatCtx, err := GetIRCContext(ctx)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return chatCtx, nil
}

// parseUsersArg extracts and validates a "users" string array from tool arguments
func parseUsersArg(args map[string]any) ([]string, error) {
	usersRaw, ok := args["users"].([]any)
	if !ok || len(usersRaw) == 0 {
		return nil, fmt.Errorf("users must be a non-empty array")
	}
	users := make([]string, len(usersRaw))
	for i, u := range usersRaw {
		if users[i], ok = u.(string); !ok {
			return nil, fmt.Errorf("all users must be strings")
		}
	}
	return users, nil
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
	BaseIRCTool
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

func (t *IrcOpTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	chatCtx, msg, err := validateAdminOp(ctx)
	if err != nil {
		return "", err
	}
	if msg != "" {
		return msg, nil
	}

	users, err := parseUsersArg(args)
	if err != nil {
		return "", err
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
		if err := ctx.Err(); err != nil {
			return "", err
		}
		chatCtx.SetMode(channel, mode, nick)
	}

	usersStr := strings.Join(users, ", ")
	chatCtx.GetLogger().Infof("IRC OP: Set mode %s for %s in %s", mode, usersStr, channel)
	return fmt.Sprintf("Set mode %s for %s", mode, usersStr), nil
}

// IrcKickTool kicks a user from the channel
type IrcKickTool struct {
	BaseIRCTool
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

func (t *IrcKickTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	chatCtx, msg, err := validateAdminOp(ctx)
	if err != nil {
		return "", err
	}
	if msg != "" {
		return msg, nil
	}

	users, err := parseUsersArg(args)
	if err != nil {
		return "", err
	}

	reason, ok := args["reason"].(string)
	if !ok {
		return "", fmt.Errorf("reason must be a string")
	}

	// Execute the IRC command for each user
	channel := chatCtx.GetConfig().Server.Channel
	for _, nick := range users {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		chatCtx.Kick(channel, nick, reason)
	}

	usersStr := strings.Join(users, ", ")
	chatCtx.GetLogger().Infow("IRC KICK: Kicked user", "users", usersStr, "channel", channel, "reason", reason)
	return fmt.Sprintf("Kicked %s: %s", usersStr, reason), nil
}

// IrcBanTool bans or unbans a user from the channel
type IrcBanTool struct {
	BaseIRCTool
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

func (t *IrcBanTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	chatCtx, msg, err := validateAdminOp(ctx)
	if err != nil {
		return "", err
	}
	if msg != "" {
		return msg, nil
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
		if user := chatCtx.GetUser(target); user != nil {
			// Create a ban mask that bans *!ident@host
			// This is more specific than *!*@host and prevents banning other users on the same host
			banMask = fmt.Sprintf("*!%s@%s", user.Ident, user.Host)
			chatCtx.GetLogger().Infow("IRC BAN: Found user", "target", target, "ident", user.Ident, "host", user.Host, "ban_mask", banMask)
		} else {
			// User not found in channel, use simple pattern
			banMask = target + "!*@*"
			chatCtx.GetLogger().Infow("IRC BAN: User not found in channel", "target", target, "ban_mask", banMask)
		}
	}

	// Execute the IRC command using girc's dedicated Ban/Unban methods
	channel := chatCtx.GetConfig().Server.Channel
	if ban {
		chatCtx.Ban(channel, banMask)
	} else {
		chatCtx.Unban(channel, banMask)
	}

	chatCtx.GetLogger().Infow("IRC BAN: Action completed", "action", action, "ban_mask", banMask, "channel", channel)
	return fmt.Sprintf("%s %s", action, banMask), nil
}

// IrcTopicTool sets the channel topic
type IrcTopicTool struct {
	BaseIRCTool
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

func (t *IrcTopicTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	chatCtx, msg, err := validateAdminOp(ctx)
	if err != nil {
		return "", err
	}
	if msg != "" {
		return msg, nil
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
	BaseIRCTool
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

func (t *IrcActionTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	chatCtx, err := validateContext(ctx)
	if err != nil {
		return "", err
	}

	message, ok := args["message"].(string)
	if !ok {
		return "", fmt.Errorf("message must be a string")
	}

	// Send IRC action directly to the configured channel
	channel := chatCtx.GetConfig().Server.Channel
	chatCtx.SendAction(channel, message)

	chatCtx.GetLogger().Infow("IRC ACTION: Sent action", "message", message)
	return fmt.Sprintf("* %s", message), nil
}

// IrcModeSetTool sets channel-wide modes
type IrcModeSetTool struct {
	BaseIRCTool
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

func (t *IrcModeSetTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	chatCtx, msg, err := validateAdminOp(ctx)
	if err != nil {
		return "", err
	}
	if msg != "" {
		return msg, nil
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
	if len(modeParams) > 0 {
		chatCtx.SetMode(channel, modeFlags, modeParams...)
	} else {
		chatCtx.SetMode(channel, modeFlags)
	}

	chatCtx.GetLogger().Infow("IRC MODE SET: Set modes", "modes", modes, "channel", channel)
	return fmt.Sprintf("Set channel mode %s on %s", modes, channel), nil
}

// IrcModeQueryTool queries current channel modes
type IrcModeQueryTool struct {
	BaseIRCTool
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

func (t *IrcModeQueryTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	chatCtx, err := validateContext(ctx)
	if err != nil {
		return "", err
	}

	channel := chatCtx.GetConfig().Server.Channel
	ch := chatCtx.GetChannel(channel)

	if ch == nil {
		return fmt.Sprintf("Channel %s not found in state (not joined yet?)", channel), nil
	}

	// Get modes as string from girc's CModes type
	modeStr := ch.Modes

	if modeStr == "" {
		return fmt.Sprintf("Channel %s has no modes set", channel), nil
	}

	chatCtx.GetLogger().Infow("IRC MODE QUERY: Channel modes", "channel", channel, "modes", modeStr)
	return fmt.Sprintf("Channel %s modes: %s", channel, modeStr), nil
}

// IrcInviteTool invites users to the channel
type IrcInviteTool struct {
	BaseIRCTool
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

func (t *IrcInviteTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	chatCtx, msg, err := validateAdminOp(ctx)
	if err != nil {
		return "", err
	}
	if msg != "" {
		return msg, nil
	}

	users, err := parseUsersArg(args)
	if err != nil {
		return "", err
	}

	// Execute the IRC INVITE command
	channel := chatCtx.GetConfig().Server.Channel
	for _, user := range users {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		chatCtx.Invite(channel, user)
	}

	usersStr := strings.Join(users, ", ")
	chatCtx.GetLogger().Infow("IRC INVITE: Invited users", "users", usersStr, "channel", channel)
	return fmt.Sprintf("Invited %s to %s", usersStr, channel), nil
}

// IrcNamesTool lists all users in the channel
type IrcNamesTool struct {
	BaseIRCTool
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

func (t *IrcNamesTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	chatCtx, err := validateContext(ctx)
	if err != nil {
		return "", err
	}

	channel := chatCtx.GetConfig().Server.Channel
	users := chatCtx.GetChannelUsers(channel)

	if users == nil {
		return fmt.Sprintf("Channel %s not found in state (not joined yet?)", channel), nil
	}

	if len(users) == 0 {
		return fmt.Sprintf("No users found in %s", channel), nil
	}

	// Build list of nicks with their prefixes (@, +, etc.)
	var nicks []string
	for _, user := range users {
		prefix := ""
		if user.IsOp {
			prefix = "@"
		} else if user.IsVoice {
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
	BaseIRCTool
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

func (t *IrcWhoisTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	chatCtx, err := validateContext(ctx)
	if err != nil {
		return "", err
	}

	nick, ok := args["nick"].(string)
	if !ok {
		return "", fmt.Errorf("nick must be a string")
	}

	user := chatCtx.GetUser(nick)

	if user == nil {
		return fmt.Sprintf("User %s not found in cached state", nick), nil
	}

	// Build detailed info from cached state
	var info strings.Builder
	info.WriteString(fmt.Sprintf("User: %s\n", user.Nick))
	info.WriteString(fmt.Sprintf("Hostmask: %s!%s@%s\n", user.Nick, user.Ident, user.Host))

	// Add extras if available
	if user.RealName != "" {
		info.WriteString(fmt.Sprintf("Real name: %s\n", user.RealName))
	}
	if user.Account != "" {
		info.WriteString(fmt.Sprintf("Account: %s (authenticated)\n", user.Account))
	}
	if user.Away != "" {
		info.WriteString(fmt.Sprintf("Away: %s\n", user.Away))
	}

	// List channels
	channels := user.Channels
	if len(channels) > 0 {
		info.WriteString(fmt.Sprintf("Channels (%d): %s\n", len(channels), strings.Join(channels, ", ")))
	}

	chatCtx.GetLogger().Infow("IRC WHOIS result", "nick", nick)
	return strings.TrimSpace(info.String()), nil
}
