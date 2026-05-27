package alert

import "testing"

func TestEvaluateThresholdExceeded(t *testing.T) {
	evaluation, err := Evaluate(Rule{Name: "cpu high", Operator: "gt", Threshold: 80}, 90)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if !evaluation.Active {
		t.Fatal("expected active alert")
	}
}

func TestEvaluateRejectsUnknownOperator(t *testing.T) {
	if _, err := Evaluate(Rule{Name: "bad", Operator: "eq", Threshold: 1}, 1); err == nil {
		t.Fatal("expected operator error")
	}
}
