package main

import (
	"testing"

	"github.com/lrstanley/girc"
	"github.com/stretchr/testify/assert"
)

func TestNewToolRegistry(t *testing.T) {
	// Load tools from paths
	tools, err := LoadTools([]string{"examples/tools/datetime.sh"})
	assert.NoError(t, err)
	assert.NotEmpty(t, tools)

	// Create the tool registry
	registry := NewToolRegistry(tools)
	assert.NotNil(t, registry)

	// check the currentdate tool
	tool, ok := registry.Get("get_current_date_with_format")
	assert.True(t, ok)
	assert.NotNil(t, tool)
}

func TestGetToolSchemas(t *testing.T) {
	// Load tools from paths
	tools, err := LoadTools([]string{"examples/tools/datetime.sh", "examples/tools/weather.py"})
	assert.NoError(t, err)
	assert.NotEmpty(t, tools)

	// Create the tool registry
	registry := NewToolRegistry(tools)
	assert.NotNil(t, registry)

	// Get tool schemas
	toolSchemas := registry.GetSchemas()
	assert.NotEmpty(t, toolSchemas)

	// Check if the tool schemas contain the expected tool
	expectedToolName := "get_current_date_with_format"
	found := false
	for _, schema := range toolSchemas {
		if schema.Name == expectedToolName {
			found = true
			break
		}
	}
	assert.True(t, found, "Expected tool schema not found")
}

func TestShellTool_Execute(t *testing.T) {
	// Load tools from paths
	tools, err := LoadTools([]string{"examples/tools/datetime.sh"})
	assert.NoError(t, err)
	assert.NotEmpty(t, tools)

	// Create the tool registry
	registry := NewToolRegistry(tools)
	assert.NotNil(t, registry)

	tool, ok := registry.Get("get_current_date_with_format")
	assert.True(t, ok)
	assert.NotNil(t, tool)

	// Create arguments for the tool
	args := map[string]interface{}{
		"format": "+%A %B %d %T %Y",
	}

	// Execute the tool
	result, err := tool.Execute(args)
	assert.NoError(t, err)
	assert.NotEmpty(t, result)

	// show the result
	t.Log(result)
}

func TestIsAdmin(t *testing.T) {
	tests := []struct {
		name     string
		admins   []string
		hostmask string
		expected bool
	}{
		{
			name:     "No admins configured",
			admins:   []string{},
			hostmask: "user@host",
			expected: true,
		},
		{
			name:     "Hostmask is admin",
			admins:   []string{"~user@host"},
			hostmask: "~user@host",
			expected: true,
		},
		{
			name:     "Hostmask is not admin",
			admins:   []string{"admin@host", "user@host2"},
			hostmask: "user@host",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock Config
			Config := NewConfiguration()
			Config.Bot.Admins = tt.admins

			// Mock Event
			event := &girc.Event{
				Source: &girc.Source{
					Name: tt.hostmask,
				},
			}

			// Create ChatContext
			ctx := &ChatContext{
				event:  event,
				Config: Config,
			}

			// Test IsAdmin
			result := ctx.IsAdmin()
			assert.Equal(t, tt.expected, result)
		})
	}
}
