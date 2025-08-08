package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
)

// Tool is the generic interface for all tools
type Tool interface {
	GetSchema() ToolSchema
	Execute(args map[string]interface{}) (string, error)
}

// ToolSchema represents a tool's definition in a provider-agnostic way
type ToolSchema struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Type        string                 `json:"type"` // Usually "object"
	Properties  map[string]interface{} `json:"properties"`
	Required    []string               `json:"required,omitempty"`
}

// ToolCall represents a request to execute a tool
type ToolCall struct {
	ID   string                 // Provider-specific ID (if any)
	Name string                 // Tool name
	Args map[string]interface{} // Parsed arguments
}

// ToolRegistry manages available tools
type ToolRegistry struct {
	tools map[string]Tool
}

// NewToolRegistry creates a new tool registry
func NewToolRegistry(toolsDir string) (*ToolRegistry, error) {
	registry := &ToolRegistry{
		tools: make(map[string]Tool),
	}

	// Load shell tools from directory if provided
	if toolsDir != "" {
		log.Println("loading tools from:", toolsDir)
		files, err := os.ReadDir(toolsDir)
		if err != nil {
			return nil, err
		}

		for _, file := range files {
			if file.IsDir() {
				continue
			}

			toolPath := toolsDir + "/" + file.Name()
			shellTool, err := NewShellTool(toolPath)
			if err != nil {
				log.Printf("failed to load tool %s: %v", toolPath, err)
				continue
			}

			schema := shellTool.GetSchema()
			log.Println("registered tool:", schema.Name)
			registry.tools[schema.Name] = shellTool
		}
	}

	return registry, nil
}

// Register adds a tool to the registry
func (r *ToolRegistry) Register(tool Tool) {
	schema := tool.GetSchema()
	log.Printf("registered tool: %s", schema.Name)
	r.tools[schema.Name] = tool
}

// Get retrieves a tool by name
func (r *ToolRegistry) Get(name string) (Tool, bool) {
	tool, ok := r.tools[name]
	return tool, ok
}

// All returns all registered tools
func (r *ToolRegistry) All() []Tool {
	tools := make([]Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, tool)
	}
	return tools
}

// GetSchemas returns schemas for all registered tools
func (r *ToolRegistry) GetSchemas() []ToolSchema {
	schemas := make([]ToolSchema, 0, len(r.tools))
	for _, tool := range r.tools {
		schemas = append(schemas, tool.GetSchema())
	}
	return schemas
}

// ShellTool wraps external commands/scripts as tools
type ShellTool struct {
	Command string
	schema  ToolSchema
}

// NewShellTool creates a new shell tool from a command
func NewShellTool(command string) (*ShellTool, error) {
	tool := &ShellTool{Command: command}

	// Load schema from the tool
	schemaJSON, err := tool.runCommand("--schema")
	if err != nil {
		return nil, fmt.Errorf("failed to get schema from %s: %v", command, err)
	}

	// Parse the schema
	err = json.Unmarshal([]byte(schemaJSON), &tool.schema)
	if err != nil {
		return nil, fmt.Errorf("failed to parse schema from %s: %v", command, err)
	}

	return tool, nil
}

// GetSchema returns the tool's schema
func (s *ShellTool) GetSchema() ToolSchema {
	return s.schema
}

// Execute runs the tool with the given arguments
func (s *ShellTool) Execute(args map[string]interface{}) (string, error) {
	// Convert args to JSON
	argsJSON, err := json.Marshal(args)
	if err != nil {
		return "", fmt.Errorf("failed to marshal arguments: %v", err)
	}

	// Run command with --execute
	cmd := exec.Command(s.Command, "--execute", string(argsJSON))
	output, err := cmd.CombinedOutput()

	// Log execution details
	if cmd.ProcessState != nil {
		log.Printf("shelltool %s: usr=%v sys=%v rc=%d",
			s.schema.Name,
			cmd.ProcessState.UserTime(),
			cmd.ProcessState.SystemTime(),
			cmd.ProcessState.ExitCode())
	}

	result := strings.TrimSpace(string(output))
	if err != nil {
		return result, fmt.Errorf("tool execution failed: %v (output: %s)", err, result)
	}

	return result, nil
}

// runCommand executes the shell tool with a single argument
func (s *ShellTool) runCommand(arg string) (string, error) {
	cmd := exec.Command(s.Command, arg)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}
