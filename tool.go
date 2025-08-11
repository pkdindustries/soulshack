package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/jsonschema"
)

// Tool is the generic interface for all tools
type Tool interface {
	GetSchema() *jsonschema.Schema
	Execute(ctx context.Context, args map[string]interface{}) (string, error)
}

// CloseableTool is a Tool that can be closed to release resources
type CloseableTool interface {
	Tool
	Close() error
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

// NewToolRegistry creates a new tool registry from a list of tools
func NewToolRegistry(tools []Tool) *ToolRegistry {
	registry := &ToolRegistry{
		tools: make(map[string]Tool),
	}

	for _, tool := range tools {
		schema := tool.GetSchema()
		name := ""
		if schema != nil && schema.Title != "" {
			name = schema.Title
		}
		log.Printf("registered tool: %s", name)
		registry.tools[name] = tool
	}

	return registry
}

// LoadTools loads tools from the given file paths
func LoadTools(paths []string) ([]Tool, error) {
	var tools []Tool

	for _, path := range paths {
		log.Printf("loading tool from: %s", path)
		shellTool, err := NewShellTool(path)
		if err != nil {
			log.Printf("failed to load tool %s: %v", path, err)
			// Continue loading other tools even if one fails
			continue
		}
		tools = append(tools, shellTool)
	}

	return tools, nil
}

// Register adds a tool to the registry
func (r *ToolRegistry) Register(tool Tool) {
	schema := tool.GetSchema()
	name := ""
	if schema != nil && schema.Title != "" {
		name = schema.Title
	}
	log.Printf("registered tool: %s", name)
	r.tools[name] = tool
}

// Get retrieves a tool by name
func (r *ToolRegistry) Get(name string) (Tool, bool) {
	tool, ok := r.tools[name]
	return tool, ok
}

// AddTool adds a single tool to the registry
func (r *ToolRegistry) AddTool(tool Tool) {
	schema := tool.GetSchema()
	name := ""
	if schema != nil && schema.Title != "" {
		name = schema.Title
	}
	log.Printf("added tool: %s", name)
	r.tools[name] = tool
}

// RemoveTool removes a tool by name from the registry
func (r *ToolRegistry) RemoveTool(name string) {
	if tool, ok := r.tools[name]; ok {
		// Clean up closeable tools
		if closeable, ok := tool.(CloseableTool); ok {
			if err := closeable.Close(); err != nil {
				log.Printf("error closing tool %s: %v", name, err)
			}
		}
		delete(r.tools, name)
		log.Printf("removed tool: %s", name)
	}
}

// Clear removes all tools from the registry
func (r *ToolRegistry) Clear() {
	r.tools = make(map[string]Tool)
	log.Printf("cleared all tools")
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
func (r *ToolRegistry) GetSchemas() []*jsonschema.Schema {
	schemas := make([]*jsonschema.Schema, 0, len(r.tools))
	for _, tool := range r.tools {
		schemas = append(schemas, tool.GetSchema())
	}
	return schemas
}

// ShellTool wraps external commands/scripts as tools
type ShellTool struct {
	Command string
	schema  *jsonschema.Schema
}

// NewShellTool creates a new shell tool from a command
func NewShellTool(command string) (*ShellTool, error) {
	tool := &ShellTool{Command: command}

	// Load schema from the tool
	schemaJSON, err := tool.runCommand("--schema")
	if err != nil {
		return nil, fmt.Errorf("failed to get schema from %s: %v", command, err)
	}

	// Parse the schema directly - it should unmarshal properly
	tool.schema = &jsonschema.Schema{}
	err = json.Unmarshal([]byte(schemaJSON), tool.schema)
	if err != nil {
		return nil, fmt.Errorf("failed to parse schema from %s: %v", command, err)
	}

	return tool, nil
}

// GetSchema returns the tool's schema
func (s *ShellTool) GetSchema() *jsonschema.Schema {
	return s.schema
}

// Execute runs the tool with the given arguments
func (s *ShellTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	// Convert args to JSON
	argsJSON, err := json.Marshal(args)
	if err != nil {
		return "", fmt.Errorf("failed to marshal arguments: %v", err)
	}

	// Run command with --execute using context for timeout
	cmd := exec.CommandContext(ctx, s.Command, "--execute", string(argsJSON))
	output, err := cmd.CombinedOutput()

	// Log execution details
	if cmd.ProcessState != nil {
		name := ""
		if s.schema != nil && s.schema.Title != "" {
			name = s.schema.Title
		}
		log.Printf("shelltool %s: usr=%v sys=%v rc=%d",
			name,
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
