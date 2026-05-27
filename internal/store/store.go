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

type AdminResource struct {
	ID                int64  `json:"id"`
	ServiceName       string `json:"serviceName"`
	Namespace         string `json:"namespace"`
	ResourceID        string `json:"resourceId"`
	Region            string `json:"region"`
	DisplayName       string `json:"displayName"`
	TagsJSON          string `json:"tags"`
	Enabled           bool   `json:"enabled"`
	DiscoveredMetrics int64  `json:"discoveredMetrics"`
	SelectedMetrics   int64  `json:"selectedMetrics"`
	MetricDefinitions int64  `json:"metricDefinitions"`
}

type AdminMetricDefinition struct {
	ID             int64  `json:"id"`
	ServiceName    string `json:"serviceName"`
	Namespace      string `json:"namespace"`
	MetricName     string `json:"metricName"`
	ResourceID     string `json:"resourceId"`
	Region         string `json:"region"`
	DimensionsJSON string `json:"dimensions"`
	Statistic      string `json:"statistic"`
	PeriodSeconds  int32  `json:"periodSeconds"`
	Unit           string `json:"unit"`
	Enabled        bool   `json:"enabled"`
}

type MetricDefinitionInput struct {
	ID             int64  `json:"id"`
	ServiceName    string `json:"serviceName"`
	Namespace      string `json:"namespace"`
	MetricName     string `json:"metricName"`
	ResourceID     string `json:"resourceId"`
	Region         string `json:"region"`
	DimensionsJSON string `json:"dimensions"`
	Statistic      string `json:"statistic"`
	PeriodSeconds  int32  `json:"periodSeconds"`
	Unit           string `json:"unit"`
	Enabled        bool   `json:"enabled"`
}

type RecommendedMetric struct {
	MetricName    string `json:"metricName"`
	Statistic     string `json:"statistic"`
	PeriodSeconds int32  `json:"periodSeconds"`
	Unit          string `json:"unit"`
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

func (s *Store) ListAdminResources(ctx context.Context, region string) ([]AdminResource, error) {
	const query = `
SELECT
    r.id,
    r.service_name,
    r.namespace,
    r.resource_id,
    r.region,
    r.display_name,
    r.tags::text,
    r.enabled,
    COUNT(DISTINCT dm.id) AS discovered_metrics,
    COUNT(DISTINCT dm.id) FILTER (WHERE dm.selected = TRUE) AS selected_metrics,
    COUNT(DISTINCT md.id) AS metric_definitions
FROM resources r
LEFT JOIN discovered_metrics dm ON dm.resource_id = r.id
LEFT JOIN metric_definitions md
    ON md.resource_id = r.resource_id
    AND md.region = r.region
WHERE ($1 = '' OR r.region = $1)
GROUP BY r.id
ORDER BY r.region, r.service_name, r.display_name, r.resource_id`

	rows, err := s.pool.Query(ctx, query, region)
	if err != nil {
		return nil, fmt.Errorf("query admin resources: %w", err)
	}
	defer rows.Close()

	resources, err := pgx.CollectRows(rows, pgx.RowToStructByPos[AdminResource])
	if err != nil {
		return nil, fmt.Errorf("scan admin resources: %w", err)
	}
	return resources, nil
}

func (s *Store) SetResourceEnabled(ctx context.Context, id int64, enabled bool) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin resource enabled update: %w", err)
	}
	defer tx.Rollback(ctx)

	var resourceID string
	var region string
	err = tx.QueryRow(ctx, `
UPDATE resources
SET enabled = $2, updated_at = now()
WHERE id = $1
RETURNING resource_id, region`, id, enabled).Scan(&resourceID, &region)
	if err == pgx.ErrNoRows {
		return fmt.Errorf("resource not found")
	}
	if err != nil {
		return fmt.Errorf("update resource enabled: %w", err)
	}

	if _, err := tx.Exec(ctx, `
UPDATE metric_definitions
SET enabled = $3, updated_at = now()
WHERE resource_id = $1
  AND region = $2`, resourceID, region, enabled); err != nil {
		return fmt.Errorf("update resource metric definitions enabled: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit resource enabled update: %w", err)
	}
	return nil
}

func (s *Store) ListAdminMetricDefinitions(ctx context.Context, region string) ([]AdminMetricDefinition, error) {
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
    COALESCE(unit, ''),
    enabled
FROM metric_definitions
WHERE ($1 = '' OR region = $1)
ORDER BY region, resource_id, namespace, metric_name, statistic, period_seconds`

	rows, err := s.pool.Query(ctx, query, region)
	if err != nil {
		return nil, fmt.Errorf("query admin metric definitions: %w", err)
	}
	defer rows.Close()

	definitions, err := pgx.CollectRows(rows, pgx.RowToStructByPos[AdminMetricDefinition])
	if err != nil {
		return nil, fmt.Errorf("scan admin metric definitions: %w", err)
	}
	return definitions, nil
}

func (s *Store) UpsertMetricDefinition(ctx context.Context, input MetricDefinitionInput) (int64, error) {
	dimensionsJSON := input.DimensionsJSON
	if dimensionsJSON == "" {
		dimensionsJSON = "[]"
	}

	if input.ID > 0 {
		tag, err := s.pool.Exec(ctx, `
UPDATE metric_definitions
SET service_name = $2,
    namespace = $3,
    metric_name = $4,
    resource_id = $5,
    region = $6,
    dimensions = $7::jsonb,
    statistic = $8,
    period_seconds = $9,
    unit = $10,
    enabled = $11,
    updated_at = now()
WHERE id = $1`,
			input.ID,
			input.ServiceName,
			input.Namespace,
			input.MetricName,
			input.ResourceID,
			input.Region,
			dimensionsJSON,
			input.Statistic,
			input.PeriodSeconds,
			input.Unit,
			input.Enabled,
		)
		if err != nil {
			return 0, fmt.Errorf("update metric definition: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return 0, fmt.Errorf("metric definition not found")
		}
		return input.ID, nil
	}

	var id int64
	err := s.pool.QueryRow(ctx, `
INSERT INTO metric_definitions (service_name, namespace, metric_name, resource_id, region, dimensions, statistic, period_seconds, unit, enabled)
VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7, $8, $9, $10)
ON CONFLICT ON CONSTRAINT metric_definitions_unique_metric DO UPDATE SET
    service_name = EXCLUDED.service_name,
    dimensions = EXCLUDED.dimensions,
    unit = EXCLUDED.unit,
    enabled = EXCLUDED.enabled,
    updated_at = now()
RETURNING id`,
		input.ServiceName,
		input.Namespace,
		input.MetricName,
		input.ResourceID,
		input.Region,
		dimensionsJSON,
		input.Statistic,
		input.PeriodSeconds,
		input.Unit,
		input.Enabled,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("upsert metric definition: %w", err)
	}
	return id, nil
}

func (s *Store) SetMetricDefinitionEnabled(ctx context.Context, id int64, enabled bool) error {
	tag, err := s.pool.Exec(ctx, `
UPDATE metric_definitions
SET enabled = $2, updated_at = now()
WHERE id = $1`, id, enabled)
	if err != nil {
		return fmt.Errorf("update metric definition enabled: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("metric definition not found")
	}
	return nil
}

func (s *Store) DeleteMetricDefinition(ctx context.Context, id int64) error {
	tag, err := s.pool.Exec(ctx, "DELETE FROM metric_definitions WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("delete metric definition: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("metric definition not found")
	}
	return nil
}

func (s *Store) ApplyRecommendedMetricSet(ctx context.Context, resourceRowID int64, metrics []RecommendedMetric) (int64, error) {
	var resource AdminResource
	err := s.pool.QueryRow(ctx, `
SELECT id, service_name, namespace, resource_id, region, display_name, tags::text, enabled, 0::bigint, 0::bigint, 0::bigint
FROM resources
WHERE id = $1`, resourceRowID).Scan(
		&resource.ID,
		&resource.ServiceName,
		&resource.Namespace,
		&resource.ResourceID,
		&resource.Region,
		&resource.DisplayName,
		&resource.TagsJSON,
		&resource.Enabled,
		&resource.DiscoveredMetrics,
		&resource.SelectedMetrics,
		&resource.MetricDefinitions,
	)
	if err != nil {
		return 0, fmt.Errorf("query resource for metric set: %w", err)
	}

	var applied int64
	for _, metric := range metrics {
		dimensionsJSON, err := s.dimensionsForDiscoveredMetric(ctx, resourceRowID, metric.MetricName)
		if err != nil {
			return 0, err
		}
		if dimensionsJSON == "" {
			dimensionsJSON = fallbackDimensionsJSON(resource.ServiceName, resource.ResourceID)
		}
		_, err = s.UpsertMetricDefinition(ctx, MetricDefinitionInput{
			ServiceName:    resource.ServiceName,
			Namespace:      resource.Namespace,
			MetricName:     metric.MetricName,
			ResourceID:     resource.ResourceID,
			Region:         resource.Region,
			DimensionsJSON: dimensionsJSON,
			Statistic:      metric.Statistic,
			PeriodSeconds:  metric.PeriodSeconds,
			Unit:           metric.Unit,
			Enabled:        true,
		})
		if err != nil {
			return 0, err
		}
		if err := s.markDiscoveredMetricSelected(ctx, resourceRowID, metric.MetricName); err != nil {
			return 0, err
		}
		applied++
	}
	return applied, nil
}

func (s *Store) markDiscoveredMetricSelected(ctx context.Context, resourceRowID int64, metricName string) error {
	_, err := s.pool.Exec(ctx, `
UPDATE discovered_metrics
SET selected = TRUE, updated_at = now()
WHERE resource_id = $1
  AND metric_name = $2`, resourceRowID, metricName)
	if err != nil {
		return fmt.Errorf("mark discovered metric selected: %w", err)
	}
	return nil
}

func (s *Store) dimensionsForDiscoveredMetric(ctx context.Context, resourceRowID int64, metricName string) (string, error) {
	var dimensionsJSON string
	err := s.pool.QueryRow(ctx, `
SELECT dimensions::text
FROM discovered_metrics
WHERE resource_id = $1
  AND metric_name = $2
ORDER BY discovered_at DESC
LIMIT 1`, resourceRowID, metricName).Scan(&dimensionsJSON)
	if err == pgx.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("query discovered metric dimensions: %w", err)
	}
	return dimensionsJSON, nil
}

func fallbackDimensionsJSON(serviceName string, resourceID string) string {
	name := "InstanceId"
	if serviceName == "lambda" {
		name = "FunctionName"
	}
	data, _ := json.Marshal([]Dimension{{Name: name, Value: resourceID}})
	return string(data)
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
