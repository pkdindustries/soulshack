package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	ai "github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"
)

type SoulShackTool interface {
	GetTool() (ai.Tool, error)
	Execute(ctx ChatContext, tool ai.ToolCall) (ai.ChatCompletionMessage, error)
}

type ToolRegistry struct {
	Tools map[string]SoulShackTool
}

func NewToolRegistry(toolsDir string) (*ToolRegistry, error) {
	toolRegistry := &ToolRegistry{
		Tools: make(map[string]SoulShackTool),
	}

	log.Println("loading tools from:", toolsDir)
	files, err := os.ReadDir(toolsDir)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		log.Println("found:", file.Name())
		if file.IsDir() {
			continue
		}

		toolPath := toolsDir + "/" + file.Name()
		shellTool := &ShellTool{
			Command: toolPath,
		}

		// Load metadata (name, description, schema)
		if err := shellTool.LoadMetadata(); err != nil {
			log.Printf("failed to load metadata for tool %s: %v", toolPath, err)
			continue
		}

		log.Println("registered tool:", shellTool.Name)
		toolRegistry.Tools[shellTool.Name] = shellTool
	}

	return toolRegistry, nil
}

func (r *ToolRegistry) RegisterTool(name string, tool SoulShackTool) {
	log.Println("registering tool:", name)
	// validate the tool by calling GetTool
	_, err := tool.GetTool()
	if err != nil {
		log.Printf("failed to validate tool %s: %v", name, err)
		return
	}
	r.Tools[name] = tool
}

func (r *ToolRegistry) GetToolByName(name string) (SoulShackTool, error) {
	tool, ok := r.Tools[name]
	if !ok {
		return nil, fmt.Errorf("tool not found in registry: %s", name)
	}
	return tool, nil
}

func (r *ToolRegistry) GetToolDefinitions() []ai.Tool {
	tools := make([]ai.Tool, 0, len(r.Tools))
	for _, tool := range r.Tools {
		definition, err := tool.GetTool()
		if err != nil {
			log.Printf("failed to get tool definition: %v", err)
			continue
		}
		tools = append(tools, definition)
	}
	return tools
}

// generic tool that can be configured to execute binaries or scripts.
type ShellTool struct {
	Command     string
	Name        string
	Description string
	Properties  jsonschema.Definition
}

// loads schema for a ShellTool.
//
//	{
//		"name": "get_current_date_with_format",
//		"description": "provides the current time and date in the specified unix date command format",
//		"type": "object",
//		"properties": {
//		  "format": {
//			"type": "string",
//			"description": "The format for the date. use unix date command format (e.g., +%Y-%m-%d %H:%M:%S). always include the leading + sign."
//		  }
//		},
//		"required": ["format"],
//		"additionalProperties": false
//	  }
//

func (s *ShellTool) LoadMetadata() error {

	schemaOutput, err := s.runCommand("--schema")
	if err != nil {
		return fmt.Errorf("failed to get schema: %v", err)
	}

	err = json.Unmarshal([]byte(schemaOutput), &s)
	if err != nil {
		return fmt.Errorf("failed to unmarshal schema: %v", err)
	}

	// i probably don't understand the go-openai library parser
	err = json.Unmarshal([]byte(schemaOutput), &s.Properties)
	if err != nil {
		return fmt.Errorf("failed to unmarshal schema: %v", err)
	}

	tool := ai.Tool{}
	err = json.Unmarshal([]byte(schemaOutput), &tool)
	if err != nil {
		return fmt.Errorf("failed to unmarshal schema: %v", err)
	}

	return nil
}

func (s *ShellTool) runCommand(arg string) (string, error) {
	cmd := exec.Command(s.Command, arg)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func (s *ShellTool) GetTool() (ai.Tool, error) {
	return ai.Tool{
		Type: ai.ToolTypeFunction,
		Function: &ai.FunctionDefinition{
			Name:        s.Name,
			Description: s.Description,
			Parameters:  s.Properties,
			Strict:      true,
		},
	}, nil
}

func (s *ShellTool) Execute(ctx ChatContext, tool ai.ToolCall) (ai.ChatCompletionMessage, error) {
	log.Printf("executing shell tool: %s", s.Command)

	// arguments are passed as a JSON string, parse it
	var args json.RawMessage
	err := json.Unmarshal([]byte(tool.Function.Arguments), &args)
	if err != nil {
		return ai.ChatCompletionMessage{Role: ai.ChatMessageRoleTool, ToolCallID: tool.ID, Name: s.Name}, err
	}

	cmd := exec.Command(s.Command, "--execute", string(args))
	output, err := cmd.CombinedOutput()

	if err != nil {
		log.Println("error executing tool:", err)
		log.Println("tool output:", string(output))
	}

	outputStr := strings.TrimSpace(string(output))
	msg := ai.ChatCompletionMessage{ToolCallID: tool.ID, Name: s.Name, Role: ai.ChatMessageRoleTool, Content: outputStr}
	return msg, err
}
