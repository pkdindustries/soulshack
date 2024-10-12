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

		// Get the tool name
		cmd := exec.Command(toolPath, "--name")
		nameOutput, err := cmd.Output()
		if err != nil {
			log.Printf("failed to get tool name for %s: %v", toolPath, err)
			continue
		}
		shellTool.Name = strings.Trim(string(nameOutput), "\n")

		// Get the tool description
		cmd = exec.Command(toolPath, "--description")
		descriptionOutput, err := cmd.Output()
		if err != nil {
			log.Printf("failed to get tool description for %s: %v", toolPath, err)
			continue
		}
		shellTool.Description = strings.Trim(string(descriptionOutput), "\n")

		// check if the tool parses out
		_, err = shellTool.GetTool()
		if err != nil {
			log.Printf("failed to get tool definition for %s: %v", toolPath, err)
			continue
		}

		log.Println("registered tool:", shellTool.Name)
		toolRegistry.Tools[shellTool.Name] = shellTool
	}

	return toolRegistry, nil
}

func (r *ToolRegistry) RegisterTool(name string, tool SoulShackTool) {
	log.Println("registering tool:", name)
	// Validate the tool by calling GetTool
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

// ShellTool is a generic tool that can be configured to execute binaries or scripts.
type ShellTool struct {
	Name        string
	Description string
	Command     string
}

// wrapper around an executable shell script / binary to implement the SoulshackTool interface
func (s *ShellTool) GetTool() (ai.Tool, error) {
	// Obtain the JSON schema by executing with the --schema argument
	cmd := exec.Command(s.Command, "--schema")
	schemaOutput, err := cmd.Output()
	if err != nil {
		log.Printf("failed to get schema: %v", err)
		return ai.Tool{}, err
	}

	var schema jsonschema.Definition
	err = json.Unmarshal(schemaOutput, &schema)
	if err != nil {
		log.Printf("failed to unmarshal schema: %v", err)
	}

	return ai.Tool{
		Type: ai.ToolTypeFunction,
		Function: &ai.FunctionDefinition{
			Name:        s.Name,
			Description: s.Description,
			Parameters:  schema,
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

	log.Printf("tool output: %s", output)
	output = []byte(strings.Trim(string(output), "\n"))
	msg := ai.ChatCompletionMessage{ToolCallID: tool.ID, Name: s.Name, Role: ai.ChatMessageRoleTool, Content: string(output)}
	return msg, err
}
