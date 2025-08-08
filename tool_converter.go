package main

import (
	"encoding/json"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/google/generative-ai-go/genai"
	ollamaapi "github.com/ollama/ollama/api"
	ai "github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"
)

// ConvertToOpenAI converts a generic tool schema to OpenAI format
func ConvertToOpenAI(schema ToolSchema) ai.Tool {
	// Convert generic properties to jsonschema.Definition
	props := make(map[string]jsonschema.Definition)
	for k, v := range schema.Properties {
		if propMap, ok := v.(map[string]interface{}); ok {
			def := jsonschema.Definition{
				Type: jsonschema.DataType(propMap["type"].(string)),
			}
			if desc, ok := propMap["description"].(string); ok {
				def.Description = desc
			}
			props[k] = def
		}
	}

	return ai.Tool{
		Type: ai.ToolTypeFunction,
		Function: &ai.FunctionDefinition{
			Name:        schema.Name,
			Description: schema.Description,
			Parameters: jsonschema.Definition{
				Type:       jsonschema.Object,
				Properties: props,
				Required:   schema.Required,
			},
		},
	}
}

// ConvertToAnthropic converts a generic tool schema to Anthropic format
func ConvertToAnthropic(schema ToolSchema) anthropic.ToolUnionParam {
	// Convert properties to Anthropic format
	properties := make(map[string]interface{})
	for k, v := range schema.Properties {
		properties[k] = v
	}

	tool := anthropic.ToolParam{
		Name:        schema.Name,
		Description: anthropic.String(schema.Description),
		InputSchema: anthropic.ToolInputSchemaParam{
			Type:       "object",
			Properties: properties,
			Required:   schema.Required,
		},
	}

	// Wrap in ToolUnionParam
	return anthropic.ToolUnionParam{
		OfTool: &tool,
	}
}

// ConvertToGemini converts a generic tool schema to Gemini format
func ConvertToGemini(schema ToolSchema) *genai.Tool {
	// Convert properties to Gemini schema format
	props := make(map[string]*genai.Schema)

	for name, prop := range schema.Properties {
		props[name] = parseGeminiProperty(prop)
	}

	return &genai.Tool{
		FunctionDeclarations: []*genai.FunctionDeclaration{{
			Name:        schema.Name,
			Description: schema.Description,
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: props,
				Required:   schema.Required,
			},
		}},
	}
}

// parseGeminiProperty converts a generic property to Gemini schema
func parseGeminiProperty(prop interface{}) *genai.Schema {
	schema := &genai.Schema{}

	// Try to parse as a map
	if propMap, ok := prop.(map[string]interface{}); ok {
		// Get type
		if typeStr, ok := propMap["type"].(string); ok {
			switch typeStr {
			case "string":
				schema.Type = genai.TypeString
			case "number":
				schema.Type = genai.TypeNumber
			case "boolean":
				schema.Type = genai.TypeBoolean
			case "array":
				schema.Type = genai.TypeArray
				// Handle array items if present
				if items, ok := propMap["items"]; ok {
					schema.Items = parseGeminiProperty(items)
				}
			case "object":
				schema.Type = genai.TypeObject
				// Handle nested properties if present
				if props, ok := propMap["properties"].(map[string]interface{}); ok {
					schema.Properties = make(map[string]*genai.Schema)
					for k, v := range props {
						schema.Properties[k] = parseGeminiProperty(v)
					}
				}
			default:
				schema.Type = genai.TypeString // Default to string
			}
		}

		// Get description
		if desc, ok := propMap["description"].(string); ok {
			schema.Description = desc
		}

		// Get enum values if present
		if enum, ok := propMap["enum"].([]interface{}); ok {
			enumStrs := make([]string, len(enum))
			for i, e := range enum {
				if s, ok := e.(string); ok {
					enumStrs[i] = s
				}
			}
			schema.Enum = enumStrs
		}
	}

	return schema
}

// ConvertToOllama converts a generic tool schema to Ollama native format
func ConvertToOllama(schema ToolSchema) ollamaapi.Tool {
	// Create the tool function
	toolFunc := ollamaapi.ToolFunction{
		Name:        schema.Name,
		Description: schema.Description,
	}

	// Set parameters
	toolFunc.Parameters.Type = schema.Type
	toolFunc.Parameters.Required = schema.Required
	toolFunc.Parameters.Properties = convertPropertiesToOllama(schema.Properties)

	return ollamaapi.Tool{
		Type:     "function",
		Function: toolFunc,
	}
}

// convertPropertiesToOllama converts generic properties to Ollama format
func convertPropertiesToOllama(props map[string]interface{}) map[string]ollamaapi.ToolProperty {
	result := make(map[string]ollamaapi.ToolProperty)

	for name, prop := range props {
		if propMap, ok := prop.(map[string]interface{}); ok {
			ollamaProp := ollamaapi.ToolProperty{}

			if typeStr, ok := propMap["type"].(string); ok {
				// PropertyType is a []string
				ollamaProp.Type = ollamaapi.PropertyType{typeStr}
			}

			if desc, ok := propMap["description"].(string); ok {
				ollamaProp.Description = desc
			}

			if enum, ok := propMap["enum"].([]interface{}); ok {
				// Enum is []any
				ollamaProp.Enum = enum
			}

			result[name] = ollamaProp
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
