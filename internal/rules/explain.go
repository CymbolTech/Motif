package rules

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// Explanation reports how one condition expression evaluated against a
// payload. It follows MatchConditions exactly, including short-circuiting, so
// the root's Matched and Error are what the worker would compute. Expressions
// skipped by short-circuiting are listed with Evaluated set to false.
type Explanation struct {
	Path      string        `json:"path"`
	Operator  string        `json:"operator"`
	Evaluated bool          `json:"evaluated"`
	Matched   bool          `json:"matched"`
	Detail    string        `json:"detail"`
	Error     string        `json:"error,omitempty"`
	Children  []Explanation `json:"children,omitempty"`
}

// ExplainConditions validates the conditions and explains their evaluation
// against one JSON object payload.
func ExplainConditions(conditions, payload json.RawMessage) (Explanation, error) {
	canonical, err := ValidateConditions(conditions)
	if err != nil {
		return Explanation{}, err
	}
	var expression map[string]any
	if err := json.Unmarshal(canonical, &expression); err != nil {
		return Explanation{}, err
	}
	var document any
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	if err := decoder.Decode(&document); err != nil {
		return Explanation{}, fmt.Errorf("payload must be valid JSON: %w", err)
	}
	if _, ok := document.(map[string]any); !ok {
		return Explanation{}, fmt.Errorf("payload must be a JSON object")
	}
	return explainExpression(expression, document, "conditions"), nil
}

// Reason summarises the explanation in one line: the error that would stop
// processing, the expression that decided a non-match, or "matched".
func (e Explanation) Reason() string {
	if e.Error != "" {
		if failing := e.firstError(); failing != nil {
			return failing.Path + ": " + failing.Error
		}
		return e.Error
	}
	if e.Matched {
		return "matched"
	}
	decisive := e.decisive()
	return decisive.Path + ": " + decisive.Detail
}

func (e Explanation) firstError() *Explanation {
	for index := range e.Children {
		if e.Children[index].Error != "" {
			return e.Children[index].firstError()
		}
	}
	if e.Error != "" {
		return &e
	}
	return nil
}

// decisive walks down to the expression that made an all() fail; an any()
// that matched nothing is itself the reason.
func (e Explanation) decisive() Explanation {
	if e.Operator == "all" {
		for _, child := range e.Children {
			if child.Evaluated && !child.Matched {
				return child.decisive()
			}
		}
	}
	return e
}

func explainExpression(expression map[string]any, payload any, path string) Explanation {
	if len(expression) == 0 {
		return Explanation{Path: path, Operator: "always", Evaluated: true, Matched: true, Detail: "an empty condition always matches"}
	}
	for operator, operand := range expression {
		switch operator {
		case "all", "any":
			return explainGroup(operator, operand.([]any), payload, path)
		case "exists":
			field := operand.(string)
			_, found := resolvePath(payload, field)
			detail := field + " is present"
			if !found {
				detail = field + " is missing"
			}
			return Explanation{Path: path, Operator: operator, Evaluated: true, Matched: found, Detail: detail}
		case "eq", "neq", "gt", "gte", "lt", "lte":
			return explainComparison(operator, operand.([]any), payload, path)
		}
	}
	return Explanation{Path: path, Evaluated: true, Detail: "unsupported expression"}
}

func explainGroup(operator string, items []any, payload any, path string) Explanation {
	node := Explanation{Path: path, Operator: operator, Evaluated: true, Matched: operator == "all"}
	decided := false
	decidedBy := -1
	for index, item := range items {
		childPath := fmt.Sprintf("%s.%s[%d]", path, operator, index)
		child := item.(map[string]any)
		if decided {
			node.Children = append(node.Children, Explanation{Path: childPath, Operator: operatorOf(child), Detail: "not evaluated: an earlier expression decided the result"})
			continue
		}
		explained := explainExpression(child, payload, childPath)
		node.Children = append(node.Children, explained)
		switch {
		case explained.Error != "":
			node.Matched, node.Error, decided, decidedBy = false, explained.Error, true, index
		case operator == "all" && !explained.Matched:
			node.Matched, decided, decidedBy = false, true, index
		case operator == "any" && explained.Matched:
			node.Matched, decided, decidedBy = true, true, index
		}
	}
	switch {
	case node.Error != "":
		node.Detail = fmt.Sprintf("%s[%d] could not be evaluated", operator, decidedBy)
	case operator == "all" && node.Matched:
		node.Detail = fmt.Sprintf("all %d expressions matched", len(items))
	case operator == "all":
		node.Detail = fmt.Sprintf("all[%d] did not match", decidedBy)
	case node.Matched:
		node.Detail = fmt.Sprintf("any[%d] matched", decidedBy)
	default:
		node.Detail = fmt.Sprintf("none of %d expressions matched", len(items))
	}
	return node
}

func explainComparison(operator string, items []any, payload any, path string) Explanation {
	left, leftFound := resolveOperand(payload, items[0])
	right, rightFound := resolveOperand(payload, items[1])
	expression := describeOperand(items[0], left, leftFound) + " " + comparisonSymbol(operator) + " " + describeOperand(items[1], right, rightFound)
	node := Explanation{Path: path, Operator: operator, Evaluated: true}
	if !leftFound || !rightFound {
		node.Matched = operator == "neq"
		node.Detail = fmt.Sprintf("%s: a missing path makes %s %t", expression, operator, node.Matched)
		return node
	}
	comparison, err := compare(left, right)
	if err != nil {
		node.Error = err.Error()
		node.Detail = expression + ": " + err.Error()
		return node
	}
	switch operator {
	case "eq":
		node.Matched = comparison == 0
	case "neq":
		node.Matched = comparison != 0
	case "gt":
		node.Matched = comparison > 0
	case "gte":
		node.Matched = comparison >= 0
	case "lt":
		node.Matched = comparison < 0
	case "lte":
		node.Matched = comparison <= 0
	}
	node.Detail = fmt.Sprintf("%s is %t", expression, node.Matched)
	return node
}

func describeOperand(operand, resolved any, found bool) string {
	if path, ok := operand.(string); ok && strings.HasPrefix(path, "$") {
		if !found {
			return path + " (missing)"
		}
		return path + " (" + formatValue(resolved) + ")"
	}
	return formatValue(operand)
}

func formatValue(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprint(value)
	}
	return string(encoded)
}

func comparisonSymbol(operator string) string {
	switch operator {
	case "eq":
		return "=="
	case "neq":
		return "!="
	case "gt":
		return ">"
	case "gte":
		return ">="
	case "lt":
		return "<"
	default:
		return "<="
	}
}

func operatorOf(expression map[string]any) string {
	if len(expression) == 0 {
		return "always"
	}
	for operator := range expression {
		return operator
	}
	return ""
}
