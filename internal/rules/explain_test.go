package rules

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestExplainConditionsAgreesWithMatchConditions(t *testing.T) {
	payload := json.RawMessage(`{"total":129.9,"status":"paid","plan":"pro","order":{"id":"o-1"},"label":"99"}`)
	cases := []string{
		`{}`,
		`{"gte":["$.total",120]}`,
		`{"gte":["$.total",200]}`,
		`{"all":[{"eq":["$.status","paid"]},{"gte":["$.total",120]}]}`,
		`{"all":[{"eq":["$.status","refunded"]},{"gte":["$.label",1]}]}`,
		`{"any":[{"eq":["$.plan","free"]},{"exists":"$.order.id"}]}`,
		`{"any":[{"eq":["$.plan","free"]},{"exists":"$.order.missing"}]}`,
		`{"neq":["$.coupon","SPRING"]}`,
		`{"eq":["$.coupon","SPRING"]}`,
		`{"all":[{"eq":["$.status","paid"]},{"gte":["$.label",1]}]}`,
		`{"any":[{"eq":["$.status","paid"]},{"gte":["$.label",1]}]}`,
	}
	for _, conditions := range cases {
		matched, matchErr := MatchConditions(json.RawMessage(conditions), payload)
		explanation, err := ExplainConditions(json.RawMessage(conditions), payload)
		if err != nil {
			t.Fatalf("ExplainConditions(%s) error = %v", conditions, err)
		}
		if explanation.Matched != matched || (explanation.Error != "") != (matchErr != nil) {
			t.Fatalf("ExplainConditions(%s) = matched %t error %q, MatchConditions = %t, %v", conditions, explanation.Matched, explanation.Error, matched, matchErr)
		}
	}
}

func TestExplainConditionsNamesTheFailingExpression(t *testing.T) {
	explanation, err := ExplainConditions(json.RawMessage(`{"all":[{"eq":["$.status","paid"]},{"gte":["$.total",120]}]}`), json.RawMessage(`{"status":"paid","total":99}`))
	if err != nil {
		t.Fatal(err)
	}
	reason := explanation.Reason()
	if !strings.Contains(reason, "conditions.all[1]") || !strings.Contains(reason, "$.total (99) >= 120 is false") {
		t.Fatalf("Reason() = %q, want the failing gte expression", reason)
	}
}

func TestExplainConditionsMarksShortCircuitedExpressions(t *testing.T) {
	explanation, err := ExplainConditions(json.RawMessage(`{"any":[{"exists":"$.order"},{"gte":["$.total",1]}]}`), json.RawMessage(`{"order":{}}`))
	if err != nil {
		t.Fatal(err)
	}
	if !explanation.Matched || explanation.Children[1].Evaluated {
		t.Fatalf("explanation = %+v, want any[0] to decide and any[1] unevaluated", explanation)
	}
}

func TestExplainConditionsSurfacesProcessingErrors(t *testing.T) {
	explanation, err := ExplainConditions(json.RawMessage(`{"all":[{"gte":["$.total",100]}]}`), json.RawMessage(`{"total":"100"}`))
	if err != nil {
		t.Fatal(err)
	}
	if explanation.Error == "" || !strings.Contains(explanation.Reason(), "conditions.all[0]") {
		t.Fatalf("explanation = %+v, want the comparison error reported on all[0]", explanation)
	}
}
