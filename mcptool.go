package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

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

// GetSchema returns the tool's schema in soulshack format
func (m *MCPTool) GetSchema() ToolSchema {
	// Convert MCP tool to soulshack ToolSchema
	schema := ToolSchema{
		Name:        m.tool.Name,
		Description: m.tool.Description,
		Type:        "object",
		Properties:  make(map[string]interface{}),
	}

	// If the MCP tool has input schema, convert it
	if m.tool.InputSchema != nil {
		// The InputSchema is already a jsonschema.Schema object
		// We need to extract its properties
		// For now, we'll just set basic properties
		// In a real implementation, we'd need to properly convert the schema
		schemaBytes, err := json.Marshal(m.tool.InputSchema)
		if err == nil {
			var inputSchema map[string]interface{}
			if err := json.Unmarshal(schemaBytes, &inputSchema); err == nil {
				if props, ok := inputSchema["properties"].(map[string]interface{}); ok {
					schema.Properties = props
				}
				if required, ok := inputSchema["required"].([]interface{}); ok {
					schema.Required = make([]string, len(required))
					for i, r := range required {
						if s, ok := r.(string); ok {
							schema.Required[i] = s
						}
					}
				}
			}
		}
	}

	return schema
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