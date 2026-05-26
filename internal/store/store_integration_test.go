package store

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestStoreIntegrationInsertsMetricPointsIdempotently(t *testing.T) {
	databaseURL := os.Getenv("INTEGRATION_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("INTEGRATION_DATABASE_URL is not set")
	}

	ctx := context.Background()
	db, err := Connect(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect database: %v", err)
	}
	defer db.Close()

	if _, err := db.pool.Exec(ctx, "TRUNCATE metric_points, metric_definitions RESTART IDENTITY CASCADE"); err != nil {
		t.Fatalf("truncate test tables: %v", err)
	}

	var definitionID int64
	err = db.pool.QueryRow(ctx, `
INSERT INTO metric_definitions (service_name, namespace, metric_name, resource_id, region, statistic, period_seconds, unit)
VALUES ('ec2', 'AWS/EC2', 'CPUUtilization', 'REPLACE_WITH_INSTANCE_ID', 'us-east-1', 'Average', 300, 'Percent')
RETURNING id`).Scan(&definitionID)
	if err != nil {
		t.Fatalf("insert metric definition: %v", err)
	}

	point := MetricPoint{
		MetricDefinitionID: definitionID,
		Timestamp:          time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC),
		Value:              12.5,
	}

	inserted, err := db.InsertMetricPoints(ctx, []MetricPoint{point})
	if err != nil {
		t.Fatalf("insert metric point: %v", err)
	}
	if inserted != 1 {
		t.Fatalf("inserted = %d, want 1", inserted)
	}

	inserted, err = db.InsertMetricPoints(ctx, []MetricPoint{point})
	if err != nil {
		t.Fatalf("insert duplicate metric point: %v", err)
	}
	if inserted != 0 {
		t.Fatalf("duplicate inserted = %d, want 0", inserted)
	}

	var count int
	if err := db.pool.QueryRow(ctx, "SELECT count(*) FROM metric_points").Scan(&count); err != nil {
		t.Fatalf("count metric points: %v", err)
	}
	if count != 1 {
		t.Fatalf("metric point count = %d, want 1", count)
	}
}
