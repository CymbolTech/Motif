package mcpserver

import (
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
)

// inputSchema infers a tool's input schema from its Go type and adds the enum
// values the struct tags cannot express.
func inputSchema[T any](enums map[string][]string) *jsonschema.Schema {
	schema, err := jsonschema.For[T](nil)
	if err != nil {
		panic(fmt.Sprintf("input schema for %T: %v", *new(T), err))
	}
	for property, values := range enums {
		field, ok := schema.Properties[property]
		if !ok {
			panic(fmt.Sprintf("input schema for %T has no property %q", *new(T), property))
		}
		for _, value := range values {
			field.Enum = append(field.Enum, value)
		}
	}
	return schema
}
