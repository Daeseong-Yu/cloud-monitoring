package discovery

import "testing"

func TestResourcesFromMetricsBuildsDisabledCandidates(t *testing.T) {
	resources := ResourcesFromMetrics("us-east-1", []Metric{
		{
			Namespace:  "AWS/Lambda",
			MetricName: "Errors",
			Dimensions: []Dimension{
				{Name: "FunctionName", Value: "orders-api"},
			},
		},
		{
			Namespace:  "AWS/Lambda",
			MetricName: "Duration",
			Dimensions: []Dimension{
				{Name: "FunctionName", Value: "orders-api"},
			},
		},
	}, map[string]TagInfo{
		"orders-api": {
			DisplayName: "Orders API",
			Tags: map[string]string{
				"Name": "Orders API",
			},
		},
	})

	if got, want := len(resources), 1; got != want {
		t.Fatalf("resource count = %d, want %d", got, want)
	}
	resource := resources[0]
	if resource.ServiceName != "lambda" {
		t.Fatalf("service name = %q, want lambda", resource.ServiceName)
	}
	if resource.DisplayName != "Orders API" {
		t.Fatalf("display name = %q, want Orders API", resource.DisplayName)
	}
	if got, want := len(resource.Metrics), 2; got != want {
		t.Fatalf("metric count = %d, want %d", got, want)
	}
	if resource.Metrics[0].MetricName == "Errors" && resource.Metrics[0].Statistic != "Sum" {
		t.Fatalf("Errors statistic = %q, want Sum", resource.Metrics[0].Statistic)
	}
}

func TestResourceIDFromDimensionsRejectsAmbiguousGenericMetric(t *testing.T) {
	id := ResourceIDFromDimensions([]Dimension{
		{Name: "Operation", Value: "GetObject"},
		{Name: "StorageType", Value: "StandardStorage"},
	})
	if id != "" {
		t.Fatalf("resource id = %q, want empty", id)
	}
}
