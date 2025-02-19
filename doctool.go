package main

import (
	ai "github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"
)

var _ SoulShackTool = (*DocTool)(nil)

// document
type DocTool struct {
	DocRoot string
}

func (fs *DocTool) Execute(ctx ChatContextInterface, tool ai.ToolCall) (ai.ChatCompletionMessage, error) {
	panic("implement me")
}

func (fs *DocTool) GetTool() (ai.Tool, error) {
	return ai.Tool{
		Type: ai.ToolTypeFunction,
		Function: &ai.FunctionDefinition{
			Name:        "document_handler",
			Description: "handles a persistant document that can be referenced later",
			Parameters: jsonschema.Definition{
				Type: jsonschema.Object,
				Properties: map[string]jsonschema.Definition{
					"owner": {
						Type:        jsonschema.String,
						Description: "the user making the requestt",
					},
					"id": {
						Type:        jsonschema.String,
						Description: "the id of the document",
					},
					"diff": {
						Type:        jsonschema.String,
						Description: "the content to store in the document",
					},
				},
				Required: []string{"name", "id", "diff"},
			},
		}}, nil
}
