package rules

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strings"
)

const (
	maxDepth      = 8
	maxNodes      = 64
	maxArrayItems = 32
)

// ValidateConditions validates and canonicalizes the restricted condition AST.
// The AST is intentionally data-only: it cannot execute customer-provided code.
func ValidateConditions(raw json.RawMessage) (json.RawMessage, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		raw = json.RawMessage(`{}`)
	}
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return nil, fmt.Errorf("conditions must be valid JSON: %w", err)
	}
	if value == nil {
		return nil, fmt.Errorf("conditions must be a JSON object")
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("conditions must be a JSON object")
	}
	state := validationState{}
	if err := validateExpression(object, 0, &state); err != nil {
		return nil, err
	}
	canonical, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("conditions could not be normalized: %w", err)
	}
	return canonical, nil
}

// MatchConditions evaluates a validated or raw condition AST against one JSON payload.
func MatchConditions(conditions, payload json.RawMessage) (bool, error) {
	canonical, err := ValidateConditions(conditions)
	if err != nil {
		return false, err
	}
	var expression map[string]any
	if err := json.Unmarshal(canonical, &expression); err != nil {
		return false, err
	}
	var document any
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	if err := decoder.Decode(&document); err != nil {
		return false, fmt.Errorf("payload must be valid JSON: %w", err)
	}
	if _, ok := document.(map[string]any); !ok {
		return false, fmt.Errorf("payload must be a JSON object")
	}
	return matchExpression(expression, document)
}

type validationState struct{ nodes int }

func validateExpression(expression map[string]any, depth int, state *validationState) error {
	if depth > maxDepth {
		return fmt.Errorf("conditions exceed maximum nesting depth of %d", maxDepth)
	}
	if len(expression) == 0 {
		return nil
	}
	if len(expression) != 1 {
		return fmt.Errorf("condition expressions must contain exactly one operator")
	}
	state.nodes++
	if state.nodes > maxNodes {
		return fmt.Errorf("conditions exceed maximum of %d expressions", maxNodes)
	}
	for operator, operand := range expression {
		switch operator {
		case "all", "any":
			items, ok := operand.([]any)
			if !ok {
				return fmt.Errorf("%s expects an array of expressions", operator)
			}
			if len(items) > maxArrayItems {
				return fmt.Errorf("%s exceeds maximum of %d expressions", operator, maxArrayItems)
			}
			for _, item := range items {
				child, ok := item.(map[string]any)
				if !ok {
					return fmt.Errorf("%s items must be expression objects", operator)
				}
				if err := validateExpression(child, depth+1, state); err != nil {
					return err
				}
			}
		case "eq", "neq", "gt", "gte", "lt", "lte":
			items, ok := operand.([]any)
			if !ok || len(items) != 2 {
				return fmt.Errorf("%s expects exactly two operands", operator)
			}
			for _, item := range items {
				if err := validateOperand(item); err != nil {
					return fmt.Errorf("%s: %w", operator, err)
				}
			}
		case "exists":
			path, ok := operand.(string)
			if !ok {
				return fmt.Errorf("exists expects a payload path")
			}
			if err := validatePath(path); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported condition operator %q", operator)
		}
	}
	return nil
}

func validateOperand(value any) error {
	if path, ok := value.(string); ok && strings.HasPrefix(path, "$") {
		return validatePath(path)
	}
	if _, ok := value.(map[string]any); ok {
		return fmt.Errorf("nested objects are not valid operands")
	}
	if _, ok := value.([]any); ok {
		return fmt.Errorf("arrays are not valid operands")
	}
	return nil
}

func validatePath(path string) error {
	if path == "$" || !strings.HasPrefix(path, "$.") {
		return fmt.Errorf("payload path %q must start with $.", path)
	}
	segments := strings.Split(path[2:], ".")
	if len(segments) == 0 {
		return fmt.Errorf("payload path %q is empty", path)
	}
	for _, segment := range segments {
		if segment == "" || strings.ContainsAny(segment, "[]\\\"") {
			return fmt.Errorf("payload path %q is malformed", path)
		}
	}
	return nil
}

func matchExpression(expression map[string]any, payload any) (bool, error) {
	if len(expression) == 0 {
		return true, nil
	}
	for operator, operand := range expression {
		switch operator {
		case "all", "any":
			items := operand.([]any)
			matched := operator == "all"
			for _, item := range items {
				childMatched, err := matchExpression(item.(map[string]any), payload)
				if err != nil {
					return false, err
				}
				if operator == "all" && !childMatched {
					return false, nil
				}
				if operator == "any" && childMatched {
					return true, nil
				}
			}
			return matched, nil
		case "exists":
			_, found := resolvePath(payload, operand.(string))
			return found, nil
		case "eq", "neq", "gt", "gte", "lt", "lte":
			items := operand.([]any)
			left, leftFound := resolveOperand(payload, items[0])
			right, rightFound := resolveOperand(payload, items[1])
			if !leftFound || !rightFound {
				return operator == "neq", nil
			}
			comparison, err := compare(left, right)
			if err != nil {
				return false, err
			}
			switch operator {
			case "eq":
				return comparison == 0, nil
			case "neq":
				return comparison != 0, nil
			case "gt":
				return comparison > 0, nil
			case "gte":
				return comparison >= 0, nil
			case "lt":
				return comparison < 0, nil
			case "lte":
				return comparison <= 0, nil
			}
		}
	}
	return false, nil
}

func resolveOperand(payload any, operand any) (any, bool) {
	if path, ok := operand.(string); ok && strings.HasPrefix(path, "$") {
		return resolvePath(payload, path)
	}
	return operand, true
}

func resolvePath(payload any, path string) (any, bool) {
	if path == "$" {
		return payload, true
	}
	current := payload
	for _, segment := range strings.Split(path[2:], ".") {
		object, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = object[segment]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func compare(left, right any) (int, error) {
	if leftNumber, ok := numberValue(left); ok {
		rightNumber, rightOK := numberValue(right)
		if !rightOK || math.IsNaN(leftNumber) || math.IsNaN(rightNumber) {
			return 0, fmt.Errorf("numeric comparison requires two numbers")
		}
		if leftNumber < rightNumber {
			return -1, nil
		}
		if leftNumber > rightNumber {
			return 1, nil
		}
		return 0, nil
	}
	if leftString, ok := left.(string); ok {
		rightString, rightOK := right.(string)
		if !rightOK {
			return 0, fmt.Errorf("comparison operands must have matching scalar types")
		}
		return strings.Compare(leftString, rightString), nil
	}
	if reflect.TypeOf(left) != reflect.TypeOf(right) {
		return 0, fmt.Errorf("comparison operands must have matching scalar types")
	}
	if reflect.DeepEqual(left, right) {
		return 0, nil
	}
	return 1, nil
}

func numberValue(value any) (float64, bool) {
	switch number := value.(type) {
	case json.Number:
		parsed, err := number.Float64()
		return parsed, err == nil
	case float64:
		return number, true
	default:
		return 0, false
	}
}
