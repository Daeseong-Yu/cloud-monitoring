package productcatalog

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestLoadProductMetricCatalog(t *testing.T) {
	catalog, err := LoadFile("../../configs/product-metric-catalog.json")
	if err != nil {
		t.Fatalf("load product metric catalog: %v", err)
	}

	services := map[string]bool{}
	for _, metric := range catalog.Metrics {
		services[metric.ServiceName] = true
	}
	for _, service := range []string{"ec2", "lambda", "api-gateway", "amplify", "ses", "s3"} {
		if !services[service] {
			t.Fatalf("expected catalog service %q", service)
		}
	}
}

func TestRecommendedMetricSetsPreserveExistingEC2AndLambdaMetrics(t *testing.T) {
	catalog, err := LoadFile("../../configs/product-metric-catalog.json")
	if err != nil {
		t.Fatalf("load product metric catalog: %v", err)
	}

	sets := catalog.RecommendedMetricSets()
	byName := map[string]RecommendedMetricSet{}
	for _, set := range sets {
		byName[set.Name] = set
	}

	assertRecommendedNames(t, byName["ec2-default"], []string{
		"CPUUtilization",
		"NetworkIn",
		"NetworkOut",
		"StatusCheckFailed",
	})
	assertRecommendedNames(t, byName["lambda-default"], []string{
		"Duration",
		"Errors",
		"Invocations",
		"Throttles",
	})
}

func TestValidateRejectsDuplicateMetricKey(t *testing.T) {
	input := `{
	  "version": 1,
	  "metrics": [
	    {
	      "key": "duplicate",
	      "serviceName": "ec2",
	      "namespace": "AWS/EC2",
	      "metricName": "CPUUtilization",
	      "statistic": "Average",
	      "periodSeconds": 300,
	      "unit": "Percent",
	      "requiredDimensions": ["InstanceId"],
	      "recommended": true,
	      "axis": {"min": 0, "max": 100},
	      "prerequisite": "",
	      "costWarning": ""
	    },
	    {
	      "key": "duplicate",
	      "serviceName": "lambda",
	      "namespace": "AWS/Lambda",
	      "metricName": "Errors",
	      "statistic": "Sum",
	      "periodSeconds": 300,
	      "unit": "Count",
	      "requiredDimensions": ["FunctionName"],
	      "recommended": true,
	      "axis": {"min": 0},
	      "prerequisite": "",
	      "costWarning": ""
	    }
	  ]
	}`

	if _, err := Load(strings.NewReader(input)); err == nil {
		t.Fatal("expected duplicate key error")
	}
}

func TestValidateRejectsMissingRequiredDimension(t *testing.T) {
	input := `{
	  "version": 1,
	  "metrics": [
	    {
	      "key": "lambda.errors",
	      "serviceName": "lambda",
	      "namespace": "AWS/Lambda",
	      "metricName": "Errors",
	      "statistic": "Sum",
	      "periodSeconds": 300,
	      "unit": "Count",
	      "requiredDimensions": [],
	      "recommended": true,
	      "axis": {"min": 0},
	      "prerequisite": "",
	      "costWarning": ""
	    }
	  ]
	}`

	if _, err := Load(strings.NewReader(input)); err == nil {
		t.Fatal("expected missing required dimension error")
	}
}

func TestValidateRejectsInvalidStatisticAndPeriod(t *testing.T) {
	for name, input := range map[string]string{
		"statistic": `{
		  "version": 1,
		  "metrics": [
		    {
		      "key": "lambda.errors",
		      "serviceName": "lambda",
		      "namespace": "AWS/Lambda",
		      "metricName": "Errors",
		      "statistic": "Median",
		      "periodSeconds": 300,
		      "unit": "Count",
		      "requiredDimensions": ["FunctionName"],
		      "recommended": true,
		      "axis": {"min": 0},
		      "prerequisite": "",
		      "costWarning": ""
		    }
		  ]
		}`,
		"period": `{
		  "version": 1,
		  "metrics": [
		    {
		      "key": "lambda.errors",
		      "serviceName": "lambda",
		      "namespace": "AWS/Lambda",
		      "metricName": "Errors",
		      "statistic": "Sum",
		      "periodSeconds": 17,
		      "unit": "Count",
		      "requiredDimensions": ["FunctionName"],
		      "recommended": true,
		      "axis": {"min": 0},
		      "prerequisite": "",
		      "costWarning": ""
		    }
		  ]
		}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Load(strings.NewReader(input)); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestCatalogFileDoesNotContainRuntimeIdentifiers(t *testing.T) {
	body, err := os.ReadFile("../../configs/product-metric-catalog.json")
	if err != nil {
		t.Fatalf("read product metric catalog: %v", err)
	}
	patterns := []string{
		`arn:aws:`,
		`AKIA[0-9A-Z]{16}`,
		`[0-9]{12}`,
		`i-[0-9a-f]{8,}`,
	}
	for _, pattern := range patterns {
		if regexp.MustCompile(pattern).Match(body) {
			t.Fatalf("catalog contains disallowed runtime identifier pattern %q", pattern)
		}
	}
}

func assertRecommendedNames(t *testing.T, set RecommendedMetricSet, want []string) {
	t.Helper()
	if len(set.Metrics) != len(want) {
		t.Fatalf("%s metric count = %d, want %d", set.Name, len(set.Metrics), len(want))
	}
	for i, metric := range set.Metrics {
		if metric.MetricName != want[i] {
			t.Fatalf("%s metric %d = %q, want %q", set.Name, i, metric.MetricName, want[i])
		}
	}
}
