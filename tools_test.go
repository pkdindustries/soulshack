package main

import (
	"testing"

	ai "github.com/sashabaranov/go-openai"
	"github.com/stretchr/testify/assert"
)

func TestNewToolRegistry(t *testing.T) {

	// Create the tool registry
	registry, err := NewToolRegistry("examples/tools")
	assert.NoError(t, err)
	assert.NotNil(t, registry)

	// check the currentdate tool
	tool, ok := registry.Tools["get_current_date_with_format"]
	assert.True(t, ok)
	assert.NotNil(t, tool)
}

func TestGetToolDefinitions(t *testing.T) {
	// Create the tool registry
	registry, err := NewToolRegistry("examples/tools")
	assert.NoError(t, err)
	assert.NotNil(t, registry)

	// Get tool definitions
	toolDefinitions := registry.GetToolDefinitions()
	assert.NotEmpty(t, toolDefinitions)

	// Check if the tool definitions contain the expected tool
	expectedToolName := "get_current_date_with_format"
	found := false
	for _, tool := range toolDefinitions {
		if tool.Function != nil && tool.Function.Name == expectedToolName {
			found = true
			break
		}
	}
	assert.True(t, found, "Expected tool definition not found")
}
func TestShellTool_Execute(t *testing.T) {

	registry, err := NewToolRegistry("examples/tools")
	assert.NoError(t, err)
	assert.NotNil(t, registry)

	ctx := ChatContext{}
	tool, err := registry.GetToolByName("get_current_date_with_format")
	assert.NoError(t, err)
	assert.NotNil(t, tool)

	// create a toolcall
	toolCall := ai.ToolCall{
		Function: ai.FunctionCall{
			Name:      "get_current_date_with_format",
			Arguments: `{"format": "+%A %B %d %T %Y"}`,
		},
		ID: "12354",
	}

	// Execute the tool
	toolmsg, err := tool.Execute(ctx, toolCall)
	assert.NoError(t, err)
	assert.NotNil(t, toolmsg)

	// show the result
	t.Log(toolmsg.Content)
}
