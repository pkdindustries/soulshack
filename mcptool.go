package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/modelcontextprotocol/go-sdk/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MCPTool wraps an MCP tool to implement the soulshack Tool interface
type MCPTool struct {
	session *mcp.ClientSession
	tool    *mcp.Tool
}

// NewMCPTool creates a new MCP tool wrapper
func NewMCPTool(session *mcp.ClientSession, tool *mcp.Tool) *MCPTool {
	return &MCPTool{
		session: session,
		tool:    tool,
	}
}

// GetSchema returns the tool's schema
func (m *MCPTool) GetSchema() *jsonschema.Schema {
	// MCP tools already use jsonschema.Schema, so we can return it directly
	if m.tool.InputSchema != nil {
		// Set the title from the tool name if not already set
		if m.tool.InputSchema.Title == "" {
			m.tool.InputSchema.Title = m.tool.Name
		}
		// Set the description if not already set
		if m.tool.InputSchema.Description == "" {
			m.tool.InputSchema.Description = m.tool.Description
		}
		return m.tool.InputSchema
	}
	
	// If no input schema, create a basic one
	return &jsonschema.Schema{
		Title:       m.tool.Name,
		Description: m.tool.Description,
		Type:        "object",
		Properties:  make(map[string]*jsonschema.Schema),
	}
}

// Execute runs the MCP tool with the given arguments
func (m *MCPTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	// Log the tool execution for debugging
	log.Printf("Executing MCP tool %s with args: %v", m.tool.Name, args)
	
	// Create the call parameters
	params := &mcp.CallToolParams{
		Name:      m.tool.Name,
		Arguments: args,
	}

	// Call the tool via MCP
	result, err := m.session.CallTool(ctx, params)
	if err != nil {
		return "", fmt.Errorf("MCP tool execution failed: %v", err)
	}

	// Handle the result
	if result.IsError {
		// If it's an error, return the error content
		if len(result.Content) > 0 {
			content, _ := json.Marshal(result.Content)
			return "", fmt.Errorf("tool returned error: %s", string(content))
		}
		return "", fmt.Errorf("tool returned error without content")
	}

	// Convert the result content to a string
	if len(result.Content) == 0 {
		return "", nil
	}

	// Marshal the content as JSON for consistent output
	output, err := json.Marshal(result.Content)
	if err != nil {
		return "", fmt.Errorf("failed to marshal tool result: %v", err)
	}

	return string(output), nil
}