package main

import (
	"encoding/json"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/google/generative-ai-go/genai"
	mcpjsonschema "github.com/modelcontextprotocol/go-sdk/jsonschema"
	ollamaapi "github.com/ollama/ollama/api"
	ai "github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"
)

// ConvertToOpenAI converts a generic tool schema to OpenAI format
func ConvertToOpenAI(schema *mcpjsonschema.Schema) ai.Tool {
	// Convert properties to OpenAI jsonschema.Definition
	props := make(map[string]jsonschema.Definition)
	if schema != nil && schema.Properties != nil {
		for k, v := range schema.Properties {
			if v != nil {
				def := jsonschema.Definition{
					Type:        jsonschema.DataType(v.Type),
					Description: v.Description,
				}
				props[k] = def
			}
		}
	}

	name := ""
	description := ""
	var required []string
	
	if schema != nil {
		name = schema.Title
		description = schema.Description
		required = schema.Required
	}
	
	return ai.Tool{
		Type: ai.ToolTypeFunction,
		Function: &ai.FunctionDefinition{
			Name:        name,
			Description: description,
			Parameters: jsonschema.Definition{
				Type:       jsonschema.Object,
				Properties: props,
				Required:   required,
			},
		},
	}
}

// ConvertToAnthropic converts a generic tool schema to Anthropic format
func ConvertToAnthropic(schema *mcpjsonschema.Schema) anthropic.ToolUnionParam {
	// Convert properties to Anthropic format
	properties := make(map[string]interface{})
	if schema != nil && schema.Properties != nil {
		for k, v := range schema.Properties {
			if v != nil {
				// Convert Schema to map for Anthropic
				propMap := map[string]interface{}{
					"type":        v.Type,
					"description": v.Description,
				}
				properties[k] = propMap
			}
		}
	}

	name := ""
	description := ""
	var required []string
	
	if schema != nil {
		name = schema.Title
		description = schema.Description
		required = schema.Required
	}
	
	tool := anthropic.ToolParam{
		Name:        name,
		Description: anthropic.String(description),
		InputSchema: anthropic.ToolInputSchemaParam{
			Type:       "object",
			Properties: properties,
			Required:   required,
		},
	}

	// Wrap in ToolUnionParam
	return anthropic.ToolUnionParam{
		OfTool: &tool,
	}
}

// ConvertToGemini converts a generic tool schema to Gemini format
func ConvertToGemini(schema *mcpjsonschema.Schema) *genai.Tool {
	// Convert properties to Gemini schema format
	props := make(map[string]*genai.Schema)

	if schema != nil && schema.Properties != nil {
		for name, prop := range schema.Properties {
			if prop != nil {
				geminiSchema := &genai.Schema{
					Description: prop.Description,
				}
				// Map type
				switch prop.Type {
				case "string":
					geminiSchema.Type = genai.TypeString
				case "number":
					geminiSchema.Type = genai.TypeNumber
				case "boolean":
					geminiSchema.Type = genai.TypeBoolean
				case "array":
					geminiSchema.Type = genai.TypeArray
				case "object":
					geminiSchema.Type = genai.TypeObject
				default:
					geminiSchema.Type = genai.TypeString
				}
				props[name] = geminiSchema
			}
		}
	}

	name := ""
	description := ""
	var required []string
	
	if schema != nil {
		name = schema.Title
		description = schema.Description
		required = schema.Required
	}
	
	return &genai.Tool{
		FunctionDeclarations: []*genai.FunctionDeclaration{{
			Name:        name,
			Description: description,
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: props,
				Required:   required,
			},
		}},
	}
}


// ConvertToOllama converts a generic tool schema to Ollama native format
func ConvertToOllama(schema *mcpjsonschema.Schema) ollamaapi.Tool {
	name := ""
	description := ""
	typeStr := "object"
	var required []string
	
	if schema != nil {
		name = schema.Title
		description = schema.Description
		if schema.Type != "" {
			typeStr = schema.Type
		}
		required = schema.Required
	}
	
	// Create the tool function
	toolFunc := ollamaapi.ToolFunction{
		Name:        name,
		Description: description,
	}

	// Set parameters
	toolFunc.Parameters.Type = typeStr
	toolFunc.Parameters.Required = required
	toolFunc.Parameters.Properties = convertPropertiesToOllamaFromSchema(schema)

	return ollamaapi.Tool{
		Type:     "function",
		Function: toolFunc,
	}
}

// convertPropertiesToOllamaFromSchema converts schema properties to Ollama format
func convertPropertiesToOllamaFromSchema(schema *mcpjsonschema.Schema) map[string]ollamaapi.ToolProperty {
	result := make(map[string]ollamaapi.ToolProperty)

	if schema != nil && schema.Properties != nil {
		for name, prop := range schema.Properties {
			if prop != nil {
				ollamaProp := ollamaapi.ToolProperty{
					Type:        ollamaapi.PropertyType{prop.Type},
					Description: prop.Description,
				}
				// Note: prop.Enum would need conversion if used
				result[name] = ollamaProp
			}
		}
	}

	return result
}

// ParseOpenAIToolCall parses an OpenAI tool call response
func ParseOpenAIToolCall(toolCall ai.ToolCall) (*ToolCall, error) {
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
		return nil, err
	}

	return &ToolCall{
		ID:   toolCall.ID,
		Name: toolCall.Function.Name,
		Args: args,
	}, nil
}

// ParseAnthropicToolCall parses an Anthropic tool use block
func ParseAnthropicToolCall(toolUse anthropic.ToolUseBlock) *ToolCall {
	// Convert input to map
	args := make(map[string]interface{})

	// The Input field is a json.RawMessage, we need to unmarshal it
	if len(toolUse.Input) > 0 {
		json.Unmarshal(toolUse.Input, &args)
	}

	return &ToolCall{
		ID:   toolUse.ID,
		Name: toolUse.Name,
		Args: args,
	}
}

// ParseGeminiToolCall parses a Gemini function call
func ParseGeminiToolCall(funcCall genai.FunctionCall) *ToolCall {
	// Convert args to map
	args := make(map[string]interface{})
	for k, v := range funcCall.Args {
		args[k] = v
	}

	return &ToolCall{
		ID:   "", // Gemini doesn't use IDs
		Name: funcCall.Name,
		Args: args,
	}
}

// ParseOllamaToolCall parses an Ollama tool call
func ParseOllamaToolCall(toolCall ollamaapi.ToolCall) *ToolCall {
	// Ollama's tool call already has parsed arguments
	args := make(map[string]interface{})
	for k, v := range toolCall.Function.Arguments {
		args[k] = v
	}

	return &ToolCall{
		ID:   "", // Ollama native doesn't have tool call IDs
		Name: toolCall.Function.Name,
		Args: args,
	}
}
