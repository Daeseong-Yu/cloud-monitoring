package store

import (
	"strings"
	"testing"
	"time"
)

func TestMetricPointUsesUTC(t *testing.T) {
	location := time.FixedZone("test", 9*60*60)
	point := MetricPoint{
		MetricDefinitionID: 1,
		Timestamp:          time.Date(2026, 1, 2, 3, 4, 5, 0, location),
		Value:              42,
	}

	if point.Timestamp.UTC().Location() != time.UTC {
		t.Fatal("expected UTC timestamp conversion to be available")
	}
}

func TestPublicMetricIDUsesPublicAliasesOnly(t *testing.T) {
	id := PublicMetricID("orders", "errors")

	if strings.Contains(id, "orders") || strings.Contains(id, "errors") || strings.Contains(id, "/") {
		t.Fatalf("public metric id should be URL-safe and encoded: %s", id)
	}
	resourceAlias, metricAlias, err := publicMetricAliasesFromID(id)
	if err != nil {
		t.Fatalf("decode public metric id: %v", err)
	}
	if resourceAlias != "orders" || metricAlias != "errors" {
		t.Fatalf("aliases = %q/%q, want orders/errors", resourceAlias, metricAlias)
	}
}
