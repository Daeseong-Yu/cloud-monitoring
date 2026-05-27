package alert

import "fmt"

type Rule struct {
	ID         int64
	Name       string
	Region     string
	Namespace  string
	ResourceID string
	MetricName string
	Statistic  string
	Operator   string
	Threshold  float64
}

type Evaluation struct {
	Rule    Rule
	Value   float64
	Active  bool
	Message string
}

func Evaluate(rule Rule, value float64) (Evaluation, error) {
	active, err := compare(rule.Operator, value, rule.Threshold)
	if err != nil {
		return Evaluation{}, err
	}
	status := "normal"
	if active {
		status = "threshold exceeded"
	}
	return Evaluation{
		Rule:    rule,
		Value:   value,
		Active:  active,
		Message: fmt.Sprintf("%s: %s value %.4f threshold %.4f", rule.Name, status, value, rule.Threshold),
	}, nil
}

func compare(operator string, value float64, threshold float64) (bool, error) {
	switch operator {
	case "gt":
		return value > threshold, nil
	case "gte":
		return value >= threshold, nil
	case "lt":
		return value < threshold, nil
	case "lte":
		return value <= threshold, nil
	default:
		return false, fmt.Errorf("unsupported alert operator %q", operator)
	}
}
