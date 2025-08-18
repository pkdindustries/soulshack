package main

import (
	"log"
	
	"github.com/alexschlessinger/pollytool/tools"
)

// LoadMCPTools loads tools from multiple MCP servers using pollytool
func LoadMCPTools(serverPaths []string) ([]tools.Tool, error) {
	var allTools []tools.Tool
	
	for _, server := range serverPaths {
		mcpClient, err := tools.NewMCPClient(server)
		if err != nil {
			log.Printf("warning connecting to MCP server %s: %v", server, err)
			continue
		}
		
		mcpTools, err := mcpClient.ListTools()
		if err != nil {
			log.Printf("warning listing tools from MCP server %s: %v", server, err)
			continue
		}
		
		allTools = append(allTools, mcpTools...)
	}
	
	return allTools, nil
}

// LoadTools is an alias for pollytool's LoadShellTools for compatibility
func LoadTools(paths []string) ([]tools.Tool, error) {
	return tools.LoadShellTools(paths)
}