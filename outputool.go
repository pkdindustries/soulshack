package main

import (
	ai "github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"
)

var _ SoulShackTool = (*CreateOutputTool)(nil)

// filesystem artifact
type CreateOutputTool struct {
	DocRoot string
}

func (fs *CreateOutputTool) Execute(ctx ChatContextInterface, tool ai.ToolCall) (ai.ChatCompletionMessage, error) {
	panic("implement me")
}

func (fs *CreateOutputTool) GetTool() (ai.Tool, error) {
	return ai.Tool{
		Type: ai.ToolTypeFunction,
		Function: &ai.FunctionDefinition{
			Name:        "create_output",
			Description: "creates a persistant output that can be referenced later",
			Parameters: jsonschema.Definition{
				Type: jsonschema.Object,
				Properties: map[string]jsonschema.Definition{
					"owner": {
						Type:        jsonschema.String,
						Description: "the user who created the output",
					},
					"name": {
						Type:        jsonschema.String,
						Description: "the name of the output",
					},
					"content": {
						Type:        jsonschema.String,
						Description: "the content to store in the persistent output",
					},
				},
				Required: []string{"name", "content"},
			},
		}}, nil
}
