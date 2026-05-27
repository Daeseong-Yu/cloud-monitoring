package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"cloud-monitor/internal/discovery"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type MetricDefinition struct {
	ID             int64
	ServiceName    string
	Namespace      string
	MetricName     string
	ResourceID     string
	Region         string
	DimensionsJSON string
	Statistic      string
	PeriodSeconds  int32
	Unit           string
}

type Dimension struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type MetricPoint struct {
	MetricDefinitionID int64
	Timestamp          time.Time
	Value              float64
}

type Store struct {
	pool *pgxpool.Pool
}

func Connect(ctx context.Context, databaseURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() {
	s.pool.Close()
}

func (s *Store) EnabledMetricDefinitions(ctx context.Context, region string) ([]MetricDefinition, error) {
	const query = `
SELECT
    id,
    service_name,
    namespace,
    metric_name,
    resource_id,
    region,
    COALESCE(dimensions, '[]'::jsonb)::text,
    statistic,
    period_seconds,
    COALESCE(unit, '')
FROM metric_definitions
WHERE enabled = TRUE
  AND region = $1
ORDER BY region, resource_id, namespace, metric_name, statistic, period_seconds`

	rows, err := s.pool.Query(ctx, query, region)
	if err != nil {
		return nil, fmt.Errorf("query enabled metric definitions: %w", err)
	}
	defer rows.Close()

	definitions, err := pgx.CollectRows(rows, pgx.RowToStructByPos[MetricDefinition])
	if err != nil {
		return nil, fmt.Errorf("scan enabled metric definitions: %w", err)
	}
	return definitions, nil
}

func (d MetricDefinition) Dimensions() ([]Dimension, error) {
	var dimensions []Dimension
	if d.DimensionsJSON != "" {
		if err := json.Unmarshal([]byte(d.DimensionsJSON), &dimensions); err != nil {
			return nil, fmt.Errorf("decode metric dimensions: %w", err)
		}
	}
	if len(dimensions) == 0 {
		dimensions = []Dimension{
			{Name: "InstanceId", Value: d.ResourceID},
		}
	}
	return dimensions, nil
}

func (s *Store) InsertMetricPoints(ctx context.Context, points []MetricPoint) (int64, error) {
	if len(points) == 0 {
		return 0, nil
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin metric point insert: %w", err)
	}
	defer tx.Rollback(ctx)

	var inserted int64
	for _, point := range points {
		tag, err := tx.Exec(ctx, `
INSERT INTO metric_points (metric_definition_id, timestamp, value)
VALUES ($1, $2, $3)
ON CONFLICT ON CONSTRAINT metric_points_unique_point DO NOTHING`,
			point.MetricDefinitionID,
			point.Timestamp.UTC(),
			point.Value,
		)
		if err != nil {
			return 0, fmt.Errorf("insert metric point: %w", err)
		}
		inserted += tag.RowsAffected()
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit metric point insert: %w", err)
	}
	return inserted, nil
}

func (s *Store) UpsertDiscoveredResources(ctx context.Context, resources []discovery.Resource) (int64, int64, error) {
	if len(resources) == 0 {
		return 0, 0, nil
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("begin discovery upsert: %w", err)
	}
	defer tx.Rollback(ctx)

	var resourceCount int64
	var metricCount int64
	for _, resource := range resources {
		tagsJSON, err := discovery.TagsJSON(resource.Tags)
		if err != nil {
			return 0, 0, err
		}

		var resourceRowID int64
		err = tx.QueryRow(ctx, `
INSERT INTO resources (service_name, namespace, resource_id, region, display_name, tags, enabled)
VALUES ($1, $2, $3, $4, $5, $6::jsonb, FALSE)
ON CONFLICT ON CONSTRAINT resources_unique_resource DO UPDATE SET
    namespace = EXCLUDED.namespace,
    display_name = EXCLUDED.display_name,
    tags = EXCLUDED.tags,
    updated_at = now()
RETURNING id`,
			resource.ServiceName,
			resource.Namespace,
			resource.ResourceID,
			resource.Region,
			resource.DisplayName,
			tagsJSON,
		).Scan(&resourceRowID)
		if err != nil {
			return 0, 0, fmt.Errorf("upsert discovered resource: %w", err)
		}
		resourceCount++

		for _, metric := range resource.Metrics {
			dimensionsJSON, err := discovery.DimensionsJSON(metric.Dimensions)
			if err != nil {
				return 0, 0, err
			}
			tag, err := tx.Exec(ctx, `
INSERT INTO discovered_metrics (resource_id, namespace, metric_name, dimensions, statistic, period_seconds, unit, region, selected)
VALUES ($1, $2, $3, $4::jsonb, $5, $6, $7, $8, FALSE)
ON CONFLICT ON CONSTRAINT discovered_metrics_unique_metric DO UPDATE SET
    unit = EXCLUDED.unit,
    region = EXCLUDED.region,
    updated_at = now()`,
				resourceRowID,
				metric.Namespace,
				metric.MetricName,
				dimensionsJSON,
				metric.Statistic,
				metric.PeriodSeconds,
				metric.Unit,
				resource.Region,
			)
			if err != nil {
				return 0, 0, fmt.Errorf("upsert discovered metric: %w", err)
			}
			metricCount += tag.RowsAffected()
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, 0, fmt.Errorf("commit discovery upsert: %w", err)
	}
	return resourceCount, metricCount, nil
}
