package main

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MCPServerManager manages connections to MCP servers
type MCPServerManager struct {
	sessions []*mcp.ClientSession
	servers  []string
}

// NewMCPServerManager creates a new MCP server manager
func NewMCPServerManager(servers []string) *MCPServerManager {
	return &MCPServerManager{
		servers:  servers,
		sessions: make([]*mcp.ClientSession, 0),
	}
}

// LoadMCPTools connects to MCP servers and returns their tools
func LoadMCPTools(serverPaths []string) ([]Tool, error) {
	if len(serverPaths) == 0 {
		return nil, nil
	}

	var allTools []Tool
	manager := NewMCPServerManager(serverPaths)

	// Connect to each server and enumerate tools
	for _, serverPath := range serverPaths {
		tools, session, err := manager.connectAndLoadTools(serverPath)
		if err != nil {
			log.Printf("failed to load MCP server %s: %v", serverPath, err)
			// Continue loading other servers even if one fails
			continue
		}
		if session != nil {
			manager.sessions = append(manager.sessions, session)
		}
		allTools = append(allTools, tools...)
	}

	return allTools, nil
}

// connectAndLoadTools connects to a single MCP server and loads its tools
func (m *MCPServerManager) connectAndLoadTools(serverPath string) ([]Tool, *mcp.ClientSession, error) {
	ctx := context.Background()

	// Parse the server path - it could be a command with arguments
	parts := strings.Fields(serverPath)
	if len(parts) == 0 {
		return nil, nil, fmt.Errorf("empty server path")
	}

	// Create the MCP client
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "soulshack",
		Version: "1.0.0",
	}, nil)

	// Create the command to run the server
	var cmd *exec.Cmd
	if len(parts) == 1 {
		cmd = exec.Command(parts[0])
	} else {
		cmd = exec.Command(parts[0], parts[1:]...)
	}
	
	// Set up stderr to see any error output from the server
	cmd.Stderr = log.Writer()

	// Connect to the server
	log.Printf("connecting to MCP server: %s", serverPath)
	session, err := client.Connect(ctx, mcp.NewCommandTransport(cmd))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to MCP server: %v", err)
	}

	// List available tools
	var soulshackTools []Tool
	for tool, err := range session.Tools(ctx, nil) {
		if err != nil {
			// Close the session if we encounter an error
			session.Close()
			return nil, nil, fmt.Errorf("error listing tools: %v", err)
		}
		if tool != nil {
			log.Printf("loaded MCP tool: %s - %s", tool.Name, tool.Description)
			soulshackTools = append(soulshackTools, NewMCPTool(session, tool))
		}
	}

	log.Printf("loaded %d tools from MCP server: %s", len(soulshackTools), serverPath)
	return soulshackTools, session, nil
}

// Close closes all MCP server connections
func (m *MCPServerManager) Close() {
	for _, session := range m.sessions {
		session.Close()
	}
	m.sessions = nil
}