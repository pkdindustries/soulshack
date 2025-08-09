package main

// ChatMessage represents a provider-agnostic chat message
type ChatMessage struct {
	Role       string
	Content    string
	ToolCalls  []ChatMessageToolCall
	ToolCallID string // For tool response messages
}

// ChatMessageToolCall represents a tool call within a message
type ChatMessageToolCall struct {
	ID       string
	Name     string
	Arguments string // JSON string of arguments
}

// Standard role constants
const (
	MessageRoleSystem    = "system"
	MessageRoleUser      = "user"
	MessageRoleAssistant = "assistant"
	MessageRoleTool      = "tool"
)