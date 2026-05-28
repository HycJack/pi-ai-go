// Package validation provides tool call argument validation.
package validation

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

// ValidationError represents a tool call validation error.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("validation error on %s: %s", e.Field, e.Message)
}

// ValidationResult contains the result of validating tool arguments.
type ValidationResult struct {
	Valid  bool              `json:"valid"`
	Errors []ValidationError `json:"errors,omitempty"`
}

// ValidateToolCall validates a tool call against its tool definition.
func ValidateToolCall(tools []ToolDef, call ToolCall) (*ToolDef, ValidationResult) {
	// Find the tool by name
	var tool *ToolDef
	for i := range tools {
		if tools[i].Name == call.Name {
			tool = &tools[i]
			break
		}
	}

	if tool == nil {
		return nil, ValidationResult{
			Valid: false,
			Errors: []ValidationError{{
				Field:   "name",
				Message: fmt.Sprintf("unknown tool: %s", call.Name),
			}},
		}
	}

	result := ValidateArguments(tool, call.Arguments)
	return tool, result
}

// ToolDef represents a tool definition for validation.
type ToolDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

// ToolCall represents a tool call for validation.
type ToolCall struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// ValidateArguments validates arguments against a tool's parameter schema.
func ValidateArguments(tool *ToolDef, arguments json.RawMessage) ValidationResult {
	if len(tool.Parameters) == 0 || string(tool.Parameters) == "null" {
		return ValidationResult{Valid: true}
	}

	// Parse the schema
	var schemaMap map[string]any
	if err := json.Unmarshal(tool.Parameters, &schemaMap); err != nil {
		return ValidationResult{
			Valid: false,
			Errors: []ValidationError{{
				Field:   "parameters",
				Message: "invalid JSON schema: " + err.Error(),
			}},
		}
	}

	// Parse the arguments
	var argsMap map[string]any
	if err := json.Unmarshal(arguments, &argsMap); err != nil {
		return ValidationResult{
			Valid: false,
			Errors: []ValidationError{{
				Field:   "arguments",
				Message: "invalid JSON: " + err.Error(),
			}},
		}
	}

	// Try to coerce types
	coerceTypes(argsMap, schemaMap)

	// Validate using JSON Schema
	c := jsonschema.NewCompiler()
	if err := c.AddResource("schema.json", schemaMap); err != nil {
		return validateSimple(argsMap, schemaMap)
	}
	schema, err := c.Compile("schema.json")
	if err != nil {
		return validateSimple(argsMap, schemaMap)
	}

	if err := schema.Validate(argsMap); err != nil {
		return ValidationResult{
			Valid: false,
			Errors: []ValidationError{{
				Field:   "arguments",
				Message: err.Error(),
			}},
		}
	}

	return ValidationResult{Valid: true}
}

// coerceTypes attempts to coerce argument values to match expected types.
func coerceTypes(args map[string]any, schema map[string]any) {
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		return
	}

	for key, val := range args {
		propSchema, ok := props[key].(map[string]any)
		if !ok {
			continue
		}
		expectedType, _ := propSchema["type"].(string)
		args[key] = coerceValue(val, expectedType)
	}
}

func coerceValue(val any, expectedType string) any {
	if val == nil {
		return val
	}

	switch expectedType {
	case "string":
		switch v := val.(type) {
		case float64:
			if v == float64(int64(v)) {
				return strconv.FormatInt(int64(v), 10)
			}
			return strconv.FormatFloat(v, 'f', -1, 64)
		case bool:
			return strconv.FormatBool(v)
		}
	case "number", "integer":
		switch v := val.(type) {
		case string:
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				if expectedType == "integer" {
					return int64(f)
				}
				return f
			}
		case bool:
			if v {
				return int64(1)
			}
			return int64(0)
		}
	case "boolean":
		switch v := val.(type) {
		case string:
			lower := strings.ToLower(v)
			if lower == "true" || lower == "1" {
				return true
			}
			if lower == "false" || lower == "0" {
				return false
			}
		case float64:
			return v != 0
		}
	}
	return val
}

// validateSimple performs basic validation when JSON Schema compilation fails.
func validateSimple(args map[string]any, schema map[string]any) ValidationResult {
	required, ok := schema["required"].([]any)
	if !ok {
		return ValidationResult{Valid: true}
	}

	var errors []ValidationError
	for _, req := range required {
		name, ok := req.(string)
		if !ok {
			continue
		}
		if _, exists := args[name]; !exists {
			errors = append(errors, ValidationError{
				Field:   name,
				Message: "required field missing",
			})
		}
	}

	if len(errors) > 0 {
		return ValidationResult{Valid: false, Errors: errors}
	}
	return ValidationResult{Valid: true}
}
