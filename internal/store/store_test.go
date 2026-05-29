package store

import (
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
