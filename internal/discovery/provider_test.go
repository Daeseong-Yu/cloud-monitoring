package discovery

import (
	"testing"

	"cloud-monitor/internal/productcatalog"
)

func TestDefaultRegistryDiscoversCatalogCandidatesWithAvailability(t *testing.T) {
	catalog, err := productcatalog.LoadFile("../../configs/product-metric-catalog.json")
	if err != nil {
		t.Fatalf("load product catalog: %v", err)
	}

	resources := DefaultRegistry().Discover("us-east-1", []Metric{
		{
			Namespace:  "AWS/Lambda",
			MetricName: "Errors",
			Dimensions: []Dimension{{Name: "FunctionName", Value: "orders-api"}},
		},
	}, map[string]TagInfo{
		"orders-api": {
			DisplayName: "Orders API",
			Tags:        map[string]string{"Name": "Orders API"},
		},
	}, catalog)

	if len(resources) != 1 {
		t.Fatalf("resource count = %d, want 1", len(resources))
	}
	resource := resources[0]
	if resource.ProviderSource != "lambda-provider" {
		t.Fatalf("provider source = %q, want lambda-provider", resource.ProviderSource)
	}
	if len(resource.Metrics) == 0 {
		t.Fatal("expected catalog metric candidates")
	}

	var foundAvailable bool
	var foundNotSeen bool
	for _, metric := range resource.Metrics {
		switch metric.MetricName {
		case "Errors":
			foundAvailable = metric.AvailabilityStatus == AvailabilityAvailable
		case "Duration":
			foundNotSeen = metric.AvailabilityStatus == AvailabilityNotSeen
		}
	}
	if !foundAvailable {
		t.Fatal("expected observed Lambda Errors metric to be available")
	}
	if !foundNotSeen {
		t.Fatal("expected unobserved Lambda Duration metric to be not_seen")
	}
}

func TestProviderMarksPrerequisiteMetricRequiresSetup(t *testing.T) {
	catalog, err := productcatalog.LoadFile("../../configs/product-metric-catalog.json")
	if err != nil {
		t.Fatalf("load product catalog: %v", err)
	}

	resources := DefaultRegistry().Discover("us-east-1", []Metric{
		{
			Namespace:  "AWS/EC2",
			MetricName: "CPUUtilization",
			Dimensions: []Dimension{{Name: "InstanceId", Value: "REPLACE_WITH_INSTANCE_ID"}},
		},
	}, nil, catalog)

	if len(resources) != 1 {
		t.Fatalf("resource count = %d, want 1", len(resources))
	}

	var found bool
	for _, metric := range resources[0].Metrics {
		if metric.MetricName == "mem_used_percent" {
			found = true
			if metric.AvailabilityStatus != AvailabilityRequiresSetup {
				t.Fatalf("mem_used_percent availability = %q, want requires_setup", metric.AvailabilityStatus)
			}
			if metric.Prerequisite == "" || metric.CostWarning == "" {
				t.Fatalf("expected prerequisite and cost warning: %#v", metric)
			}
		}
	}
	if !found {
		t.Fatal("expected CWAgent memory candidate")
	}
}

func TestDefaultRegistryIncludesProductizationServices(t *testing.T) {
	namespaces := map[string]bool{}
	for _, namespace := range DefaultRegistry().Namespaces() {
		namespaces[namespace] = true
	}

	for _, namespace := range []string{
		"AWS/EC2",
		"CWAgent",
		"AWS/Lambda",
		"AWS/ApiGateway",
		"AWS/AmplifyHosting",
		"AWS/SES",
		"AWS/S3",
	} {
		if !namespaces[namespace] {
			t.Fatalf("expected namespace %q in default registry", namespace)
		}
	}
}
