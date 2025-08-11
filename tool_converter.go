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

// convertSchemaToOpenAIDefinition recursively converts an MCP schema to an OpenAI Definition
func convertSchemaToOpenAIDefinition(schema *mcpjsonschema.Schema) jsonschema.Definition {
	if schema == nil {
		return jsonschema.Definition{}
	}

	def := jsonschema.Definition{
		Type:        jsonschema.DataType(schema.Type),
		Description: schema.Description,
	}

	// Handle different types
	switch schema.Type {
	case "array":
		// Handle array items - this is the critical fix
		if schema.Items != nil {
			items := convertSchemaToOpenAIDefinition(schema.Items)
			def.Items = &items
		}
	case "object":
		// Handle nested object properties recursively
		if schema.Properties != nil {
			props := make(map[string]jsonschema.Definition)
			for name, prop := range schema.Properties {
				if prop != nil {
					props[name] = convertSchemaToOpenAIDefinition(prop)
				}
			}
			def.Properties = props
		}
		if len(schema.Required) > 0 {
			def.Required = schema.Required
		}
	}

	// Handle enums if present
	if len(schema.Enum) > 0 {
		// Convert any type enums to string enums for OpenAI
		enumStrs := make([]string, 0, len(schema.Enum))
		for _, e := range schema.Enum {
			if s, ok := e.(string); ok {
				enumStrs = append(enumStrs, s)
			}
		}
		if len(enumStrs) > 0 {
			def.Enum = enumStrs
		}
	}

	return def
}

// ConvertToOpenAI converts a generic tool schema to OpenAI format
func ConvertToOpenAI(schema *mcpjsonschema.Schema) ai.Tool {
	// Convert properties to OpenAI jsonschema.Definition using recursive conversion
	props := make(map[string]jsonschema.Definition)
	if schema != nil && schema.Properties != nil {
		for k, v := range schema.Properties {
			if v != nil {
				props[k] = convertSchemaToOpenAIDefinition(v)
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

// convertSchemaToAnthropicMap recursively converts an MCP schema to Anthropic format map
func convertSchemaToAnthropicMap(schema *mcpjsonschema.Schema) map[string]interface{} {
	if schema == nil {
		return nil
	}

	propMap := make(map[string]interface{})

	// Always set type, default to string if empty
	if schema.Type != "" {
		propMap["type"] = schema.Type
	} else {
		propMap["type"] = "string"
	}

	// Only add description if non-empty
	if schema.Description != "" {
		propMap["description"] = schema.Description
	}

	// Handle different types
	switch schema.Type {
	case "array":
		// Handle array items - this is the critical fix
		if schema.Items != nil {
			propMap["items"] = convertSchemaToAnthropicMap(schema.Items)
		} else {
			// Array must have items defined for JSON Schema 2020-12
			propMap["items"] = map[string]interface{}{
				"type": "string",
			}
		}
	case "object":
		// Handle nested object properties recursively
		if len(schema.Properties) > 0 {
			props := make(map[string]interface{})
			for name, prop := range schema.Properties {
				if prop != nil {
					props[name] = convertSchemaToAnthropicMap(prop)
				}
			}
			propMap["properties"] = props
		} else {
			// Object should have properties defined
			propMap["properties"] = make(map[string]interface{})
		}
		if len(schema.Required) > 0 {
			propMap["required"] = schema.Required
		}
	}

	// Handle enums if present
	if len(schema.Enum) > 0 {
		propMap["enum"] = schema.Enum
	}

	return propMap
}

// ConvertToAnthropic converts a generic tool schema to Anthropic format
func ConvertToAnthropic(schema *mcpjsonschema.Schema) anthropic.ToolUnionParam {
	// Convert properties to Anthropic format using recursive conversion
	properties := make(map[string]interface{})
	if schema != nil && schema.Properties != nil {
		for k, v := range schema.Properties {
			if v != nil {
				properties[k] = convertSchemaToAnthropicMap(v)
			}
		}
	}

	name := ""
	description := ""
	var required []string

	if schema != nil {
		name = schema.Title
		description = schema.Description
		// Only set required if it's not empty
		if len(schema.Required) > 0 {
			required = schema.Required
		}
	}

	// Build InputSchema with proper JSON Schema 2020-12 format
	inputSchema := anthropic.ToolInputSchemaParam{
		Type:       "object",
		Properties: properties,
	}

	// Only add required field if it's not empty
	if len(required) > 0 {
		inputSchema.Required = required
	}

	tool := anthropic.ToolParam{
		Name:        name,
		Description: anthropic.String(description),
		InputSchema: inputSchema,
	}

	// Wrap in ToolUnionParam
	return anthropic.ToolUnionParam{
		OfTool: &tool,
	}
}

// convertSchemaToGeminiSchema recursively converts an MCP schema to a Gemini schema
func convertSchemaToGeminiSchema(schema *mcpjsonschema.Schema) *genai.Schema {
	if schema == nil {
		return nil
	}

	geminiSchema := &genai.Schema{
		Description: schema.Description,
	}

	// Map type
	switch schema.Type {
	case "string":
		geminiSchema.Type = genai.TypeString
	case "number":
		geminiSchema.Type = genai.TypeNumber
	case "boolean":
		geminiSchema.Type = genai.TypeBoolean
	case "array":
		geminiSchema.Type = genai.TypeArray
		// Handle array items - this is the critical fix
		if schema.Items != nil {
			geminiSchema.Items = convertSchemaToGeminiSchema(schema.Items)
		}
	case "object":
		geminiSchema.Type = genai.TypeObject
		// Handle nested object properties recursively
		if schema.Properties != nil {
			props := make(map[string]*genai.Schema)
			for name, prop := range schema.Properties {
				if prop != nil {
					props[name] = convertSchemaToGeminiSchema(prop)
				}
			}
			geminiSchema.Properties = props
		}
		if len(schema.Required) > 0 {
			geminiSchema.Required = schema.Required
		}
	default:
		// Default to string for unknown types
		geminiSchema.Type = genai.TypeString
	}

	return geminiSchema
}

// ConvertToGemini converts a generic tool schema to Gemini format
func ConvertToGemini(schema *mcpjsonschema.Schema) *genai.Tool {
	// Convert properties to Gemini schema format using recursive conversion
	props := make(map[string]*genai.Schema)

	if schema != nil && schema.Properties != nil {
		for name, prop := range schema.Properties {
			if prop != nil {
				props[name] = convertSchemaToGeminiSchema(prop)
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

// convertSchemaToOllamaProperty recursively converts an MCP schema to an Ollama ToolProperty
func convertSchemaToOllamaProperty(schema *mcpjsonschema.Schema) ollamaapi.ToolProperty {
	if schema == nil {
		return ollamaapi.ToolProperty{
			Type: ollamaapi.PropertyType{"string"},
		}
	}

	ollamaProp := ollamaapi.ToolProperty{
		Type:        ollamaapi.PropertyType{schema.Type},
		Description: schema.Description,
	}

	// Handle different types
	switch schema.Type {
	case "array":
		// Handle array items - this is the critical fix
		if schema.Items != nil {
			itemsProp := convertSchemaToOllamaProperty(schema.Items)
			ollamaProp.Items = itemsProp
		} else {
			// Default to string items if not specified
			ollamaProp.Items = ollamaapi.ToolProperty{
				Type: ollamaapi.PropertyType{"string"},
			}
		}
	case "object":
		// For nested objects, we need to handle properties recursively
		// Note: Ollama's ToolProperty doesn't have a Properties field for nested objects
		// So we'll need to handle this differently or flatten the structure
		if len(schema.Properties) > 0 {
			// We can't directly set nested properties in ToolProperty
			// This is a limitation of the Ollama API structure
			// For now, we'll just mark it as object type
		}
	}

	// Handle enums if present
	if len(schema.Enum) > 0 {
		ollamaProp.Enum = schema.Enum
	}

	return ollamaProp
}

// convertPropertiesToOllamaFromSchema converts schema properties to Ollama format
func convertPropertiesToOllamaFromSchema(schema *mcpjsonschema.Schema) map[string]ollamaapi.ToolProperty {
	result := make(map[string]ollamaapi.ToolProperty)

	if schema != nil && schema.Properties != nil {
		for name, prop := range schema.Properties {
			if prop != nil {
				result[name] = convertSchemaToOllamaProperty(prop)
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
