package events

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

type eventSchema struct {
	Type                 string                 `json:"type"`
	Required             []string               `json:"required"`
	Properties           map[string]eventSchema `json:"properties"`
	Enum                 []any                  `json:"enum"`
	Minimum              *float64               `json:"minimum"`
	Maximum              *float64               `json:"maximum"`
	AdditionalProperties *bool                  `json:"additionalProperties"`
}

func ValidateSchema(schema, payload json.RawMessage) error {
	var definition eventSchema
	if err := json.Unmarshal(schema, &definition); err != nil {
		return fmt.Errorf("event schema must be valid JSON: %w", err)
	}
	var value any
	if err := json.Unmarshal(payload, &value); err != nil {
		return fmt.Errorf("payload must be valid JSON: %w", err)
	}
	if object, ok := value.(map[string]any); !ok || object == nil {
		return fmt.Errorf("payload must be a JSON object")
	}
	if err := validateValue(definition, value, "payload", 0); err != nil {
		return err
	}
	return nil
}

func validateValue(schema eventSchema, value any, path string, depth int) error {
	if depth > 16 {
		return fmt.Errorf("%s exceeds maximum schema depth", path)
	}
	if schema.Type != "" && !matchesType(schema.Type, value) {
		return fmt.Errorf("%s must be of type %s", path, schema.Type)
	}
	if len(schema.Enum) > 0 && !containsJSONValue(schema.Enum, value) {
		return fmt.Errorf("%s must match one of the enum values", path)
	}
	if number, ok := numberValue(value); ok {
		if schema.Minimum != nil && number < *schema.Minimum {
			return fmt.Errorf("%s must be greater than or equal to %v", path, *schema.Minimum)
		}
		if schema.Maximum != nil && number > *schema.Maximum {
			return fmt.Errorf("%s must be less than or equal to %v", path, *schema.Maximum)
		}
	}
	object, isObject := value.(map[string]any)
	if !isObject {
		return nil
	}
	for _, field := range schema.Required {
		if _, ok := object[field]; !ok {
			return fmt.Errorf("%s is missing required field %q", path, field)
		}
	}
	for key, child := range object {
		property, declared := schema.Properties[key]
		if !declared {
			if schema.AdditionalProperties != nil && !*schema.AdditionalProperties {
				return fmt.Errorf("%s.%s is not allowed by the schema", path, key)
			}
			continue
		}
		if err := validateValue(property, child, path+"."+key, depth+1); err != nil {
			return err
		}
	}
	return nil
}

func matchesType(kind string, value any) bool {
	switch strings.ToLower(kind) {
	case "object":
		_, ok := value.(map[string]any)
		return ok
	case "array":
		_, ok := value.([]any)
		return ok
	case "string":
		_, ok := value.(string)
		return ok
	case "number":
		_, ok := numberValue(value)
		return ok
	case "integer":
		number, ok := numberValue(value)
		return ok && math.Trunc(number) == number
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "null":
		return value == nil
	default:
		return false
	}
}

func numberValue(value any) (float64, bool) {
	number, ok := value.(float64)
	return number, ok
}

func containsJSONValue(values []any, value any) bool {
	for _, candidate := range values {
		left, _ := json.Marshal(candidate)
		right, _ := json.Marshal(value)
		if string(left) == string(right) {
			return true
		}
	}
	return false
}
