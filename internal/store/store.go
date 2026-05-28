package store

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
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

type MetricCollectionStatusInput struct {
	MetricDefinitionID int64
	LatestPointAt      time.Time
	RecentPointCount   int64
	FetchedPointCount  int64
	InsertedPointCount int64
	SanitizedError     string
	CollectedAt        time.Time
}

type MetricInsertSummary struct {
	Inserted             int64
	InsertedByDefinition map[int64]int64
}

type AdminResource struct {
	ID                int64  `json:"id"`
	ServiceName       string `json:"serviceName"`
	Namespace         string `json:"namespace"`
	ResourceID        string `json:"resourceId"`
	Region            string `json:"region"`
	DisplayName       string `json:"displayName"`
	TagsJSON          string `json:"tags"`
	ProviderSource    string `json:"providerSource"`
	DiscoverySource   string `json:"discoverySource"`
	InternalRegion    string `json:"internalRegion"`
	PublicEnabled     bool   `json:"publicEnabled"`
	PublicDisplayName string `json:"publicDisplayName"`
	PublicDescription string `json:"publicDescription"`
	PublicLabel       string `json:"publicLabel"`
	PublicSortOrder   int32  `json:"publicSortOrder"`
	Enabled           bool   `json:"enabled"`
	DiscoveredMetrics int64  `json:"discoveredMetrics"`
	AvailableMetrics  int64  `json:"availableMetrics"`
	SelectedMetrics   int64  `json:"selectedMetrics"`
	MetricDefinitions int64  `json:"metricDefinitions"`
}

type AdminService struct {
	ServiceName        string `json:"serviceName"`
	Namespace          string `json:"namespace"`
	ResourceCount      int64  `json:"resourceCount"`
	AvailableMetrics   int64  `json:"availableMetrics"`
	RequiresSetup      int64  `json:"requiresSetup"`
	UnsupportedMetrics int64  `json:"unsupportedMetrics"`
	SelectedMetrics    int64  `json:"selectedMetrics"`
}

type AdminMetricCandidate struct {
	ID                 int64  `json:"id"`
	ResourceID         int64  `json:"resourceRowId"`
	ServiceName        string `json:"serviceName"`
	ResourceIdentifier string `json:"resourceId"`
	DisplayName        string `json:"displayName"`
	Namespace          string `json:"namespace"`
	MetricName         string `json:"metricName"`
	DimensionsJSON     string `json:"dimensions"`
	Statistic          string `json:"statistic"`
	PeriodSeconds      int32  `json:"periodSeconds"`
	Unit               string `json:"unit"`
	Region             string `json:"region"`
	Selected           bool   `json:"selected"`
	AvailabilityStatus string `json:"availabilityStatus"`
	AvailabilityReason string `json:"availabilityReason"`
	ProviderSource     string `json:"providerSource"`
	Prerequisite       string `json:"prerequisite"`
	CostWarning        string `json:"costWarning"`
}

type AdminMetricDefinition struct {
	ID                 int64  `json:"id"`
	ServiceName        string `json:"serviceName"`
	Namespace          string `json:"namespace"`
	MetricName         string `json:"metricName"`
	ResourceID         string `json:"resourceId"`
	Region             string `json:"region"`
	DimensionsJSON     string `json:"dimensions"`
	Statistic          string `json:"statistic"`
	PeriodSeconds      int32  `json:"periodSeconds"`
	Unit               string `json:"unit"`
	Enabled            bool   `json:"enabled"`
	PublicEnabled      bool   `json:"publicEnabled"`
	PublicLabel        string `json:"publicLabel"`
	LastRunStatus      string `json:"lastRunStatus"`
	LastSuccessAt      string `json:"lastSuccessAt"`
	LastFailureAt      string `json:"lastFailureAt"`
	LatestPointAt      string `json:"latestPointAt"`
	FetchedPointCount  int64  `json:"fetchedPointCount"`
	InsertedPointCount int64  `json:"insertedPointCount"`
	RecentPointCount   int64  `json:"recentPointCount"`
	SanitizedError     string `json:"sanitizedError"`
}

type CollectionCostEstimate struct {
	EnabledMetricCount             int64   `json:"enabledMetricCount"`
	RegionCount                    int64   `json:"regionCount"`
	CollectorIntervalSeconds       int64   `json:"collectorIntervalSeconds"`
	MonthlyCollectionRunsPerRegion int64   `json:"monthlyCollectionRunsPerRegion"`
	MonthlyMetricRequests          int64   `json:"monthlyMetricRequests"`
	GetMetricDataPricePerThousand  float64 `json:"getMetricDataPricePerThousand"`
	EstimatedMonthlyCostUSD        float64 `json:"estimatedMonthlyCostUsd"`
	CostWarningMetricCount         int64   `json:"costWarningMetricCount"`
	PricingNote                    string  `json:"pricingNote"`
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

type PublicMetadataInput struct {
	PublicEnabled     bool   `json:"publicEnabled"`
	PublicDisplayName string `json:"publicDisplayName"`
	PublicDescription string `json:"publicDescription"`
	PublicLabel       string `json:"publicLabel"`
	PublicSortOrder   int32  `json:"publicSortOrder"`
}

type PublicMetric struct {
	ID                  string `json:"id"`
	ResourceAlias       string `json:"resourceAlias"`
	ResourceLabel       string `json:"resourceLabel"`
	ResourceDescription string `json:"resourceDescription,omitempty"`
	MetricAlias         string `json:"metricAlias"`
	MetricLabel         string `json:"metricLabel"`
	MetricDescription   string `json:"metricDescription,omitempty"`
	Unit                string `json:"unit,omitempty"`
	LatestPointAt       string `json:"latestPointAt,omitempty"`
	RecentPointCount    int64  `json:"recentPointCount"`
}

type PublicMetricSeriesPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
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

const publicMetricIDSeparator = "\x1f"

func PublicMetricID(resourceAlias string, metricAlias string) string {
	payload := strings.TrimSpace(resourceAlias) + publicMetricIDSeparator + strings.TrimSpace(metricAlias)
	return base64.RawURLEncoding.EncodeToString([]byte(payload))
}

func publicMetricAliasesFromID(id string) (string, string, error) {
	data, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(id))
	if err != nil {
		return "", "", fmt.Errorf("invalid public metric id")
	}
	parts := strings.SplitN(string(data), publicMetricIDSeparator, 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return "", "", fmt.Errorf("invalid public metric id")
	}
	return parts[0], parts[1], nil
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

func (s *Store) RecordMetricCollectionSuccess(ctx context.Context, input MetricCollectionStatusInput) error {
	collectedAt := input.CollectedAt.UTC()
	if collectedAt.IsZero() {
		collectedAt = time.Now().UTC()
	}

	var latestPoint any
	if !input.LatestPointAt.IsZero() {
		latestPoint = input.LatestPointAt.UTC()
	}

	_, err := s.pool.Exec(ctx, `
INSERT INTO metric_collection_status (
    metric_definition_id,
    last_success_at,
    latest_point_at,
    recent_point_count,
    fetched_point_count,
    inserted_point_count,
    last_run_status,
    sanitized_error,
    updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, 'success', '', now())
ON CONFLICT (metric_definition_id) DO UPDATE SET
    last_success_at = EXCLUDED.last_success_at,
    latest_point_at = COALESCE(EXCLUDED.latest_point_at, metric_collection_status.latest_point_at),
    recent_point_count = EXCLUDED.recent_point_count,
    fetched_point_count = EXCLUDED.fetched_point_count,
    inserted_point_count = EXCLUDED.inserted_point_count,
    last_run_status = 'success',
    sanitized_error = '',
    updated_at = now()`,
		input.MetricDefinitionID,
		collectedAt,
		latestPoint,
		input.RecentPointCount,
		input.FetchedPointCount,
		input.InsertedPointCount,
	)
	if err != nil {
		return fmt.Errorf("record metric collection success: %w", err)
	}
	return nil
}

func (s *Store) RecordMetricCollectionFailure(ctx context.Context, input MetricCollectionStatusInput) error {
	collectedAt := input.CollectedAt.UTC()
	if collectedAt.IsZero() {
		collectedAt = time.Now().UTC()
	}

	_, err := s.pool.Exec(ctx, `
INSERT INTO metric_collection_status (
    metric_definition_id,
    last_failure_at,
    last_run_status,
    sanitized_error,
    updated_at
)
VALUES ($1, $2, 'failure', $3, now())
ON CONFLICT (metric_definition_id) DO UPDATE SET
    last_failure_at = EXCLUDED.last_failure_at,
    last_run_status = 'failure',
    sanitized_error = EXCLUDED.sanitized_error,
    updated_at = now()`,
		input.MetricDefinitionID,
		collectedAt,
		input.SanitizedError,
	)
	if err != nil {
		return fmt.Errorf("record metric collection failure: %w", err)
	}
	return nil
}

func (s *Store) InsertMetricPoints(ctx context.Context, points []MetricPoint) (int64, error) {
	summary, err := s.InsertMetricPointsDetailed(ctx, points)
	if err != nil {
		return 0, err
	}
	return summary.Inserted, nil
}

func (s *Store) InsertMetricPointsDetailed(ctx context.Context, points []MetricPoint) (MetricInsertSummary, error) {
	if len(points) == 0 {
		return MetricInsertSummary{InsertedByDefinition: map[int64]int64{}}, nil
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return MetricInsertSummary{}, fmt.Errorf("begin metric point insert: %w", err)
	}
	defer tx.Rollback(ctx)

	summary := MetricInsertSummary{InsertedByDefinition: map[int64]int64{}}
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
			return MetricInsertSummary{}, fmt.Errorf("insert metric point: %w", err)
		}
		inserted := tag.RowsAffected()
		summary.Inserted += inserted
		summary.InsertedByDefinition[point.MetricDefinitionID] += inserted
	}

	if err := tx.Commit(ctx); err != nil {
		return MetricInsertSummary{}, fmt.Errorf("commit metric point insert: %w", err)
	}
	return summary, nil
}

func (s *Store) ListAdminServices(ctx context.Context, region string) ([]AdminService, error) {
	const query = `
SELECT
    r.service_name,
    r.namespace,
    COUNT(DISTINCT r.id) AS resource_count,
    COUNT(DISTINCT dm.id) FILTER (WHERE dm.availability_status = 'available') AS available_metrics,
    COUNT(DISTINCT dm.id) FILTER (WHERE dm.availability_status = 'requires_setup') AS requires_setup,
    COUNT(DISTINCT dm.id) FILTER (WHERE dm.availability_status = 'unsupported') AS unsupported_metrics,
    COUNT(DISTINCT dm.id) FILTER (WHERE dm.selected = TRUE) AS selected_metrics
FROM resources r
LEFT JOIN discovered_metrics dm ON dm.resource_id = r.id
WHERE ($1 = '' OR r.region = $1)
GROUP BY r.service_name, r.namespace
ORDER BY r.service_name, r.namespace`

	rows, err := s.pool.Query(ctx, query, region)
	if err != nil {
		return nil, fmt.Errorf("query admin services: %w", err)
	}
	defer rows.Close()

	services, err := pgx.CollectRows(rows, pgx.RowToStructByPos[AdminService])
	if err != nil {
		return nil, fmt.Errorf("scan admin services: %w", err)
	}
	return services, nil
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
    COALESCE(r.provider_source, ''),
    COALESCE(r.discovery_source, ''),
    COALESCE(NULLIF(r.internal_region_label, ''), r.region),
    r.public_enabled,
    r.public_display_name,
    r.public_description,
    r.public_label,
    r.public_sort_order,
    r.enabled,
    COUNT(DISTINCT dm.id) AS discovered_metrics,
    COUNT(DISTINCT dm.id) FILTER (WHERE dm.availability_status = 'available') AS available_metrics,
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

func (s *Store) ListAdminMetricCandidates(ctx context.Context, region string) ([]AdminMetricCandidate, error) {
	const query = `
SELECT
    dm.id,
    dm.resource_id,
    r.service_name,
    r.resource_id,
    r.display_name,
    dm.namespace,
    dm.metric_name,
    dm.dimensions::text,
    dm.statistic,
    dm.period_seconds,
    COALESCE(dm.unit, ''),
    dm.region,
    dm.selected,
    dm.availability_status,
    dm.availability_reason,
    dm.provider_source,
    dm.prerequisite,
    dm.cost_warning
FROM discovered_metrics dm
JOIN resources r ON r.id = dm.resource_id
WHERE ($1 = '' OR dm.region = $1)
ORDER BY dm.region, r.service_name, r.display_name, dm.metric_name, dm.statistic, dm.period_seconds`

	rows, err := s.pool.Query(ctx, query, region)
	if err != nil {
		return nil, fmt.Errorf("query admin metric candidates: %w", err)
	}
	defer rows.Close()

	candidates, err := pgx.CollectRows(rows, pgx.RowToStructByPos[AdminMetricCandidate])
	if err != nil {
		return nil, fmt.Errorf("scan admin metric candidates: %w", err)
	}
	return candidates, nil
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

func (s *Store) UpdateResourcePublicMetadata(ctx context.Context, id int64, input PublicMetadataInput) error {
	if input.PublicEnabled && strings.TrimSpace(input.PublicLabel) == "" {
		return fmt.Errorf("public_label is required when public_enabled is true")
	}
	tag, err := s.pool.Exec(ctx, `
UPDATE resources
SET public_enabled = $2,
    public_display_name = $3,
    public_description = $4,
    public_label = $5,
    public_sort_order = $6,
    updated_at = now()
WHERE id = $1`,
		id,
		input.PublicEnabled,
		strings.TrimSpace(input.PublicDisplayName),
		strings.TrimSpace(input.PublicDescription),
		strings.TrimSpace(input.PublicLabel),
		input.PublicSortOrder,
	)
	if err != nil {
		return fmt.Errorf("update resource public metadata: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("resource not found")
	}
	return nil
}

func (s *Store) ListPublicMetrics(ctx context.Context) ([]PublicMetric, error) {
	const query = `
SELECT
    r.public_label AS resource_alias,
    COALESCE(NULLIF(r.public_display_name, ''), r.public_label) AS resource_label,
    r.public_description AS resource_description,
    md.public_label AS metric_alias,
    COALESCE(NULLIF(md.public_display_name, ''), md.public_label) AS metric_label,
    md.public_description AS metric_description,
    COALESCE(md.unit, '') AS unit,
    COALESCE(mcs.latest_point_at::text, '') AS latest_point_at,
    COALESCE(mcs.recent_point_count, 0) AS recent_point_count
FROM resources r
JOIN metric_definitions md
    ON md.resource_id = r.resource_id
    AND md.region = r.region
LEFT JOIN metric_collection_status mcs
    ON mcs.metric_definition_id = md.id
WHERE r.public_enabled = TRUE
  AND md.public_enabled = TRUE
  AND md.enabled = TRUE
  AND r.public_label <> ''
  AND md.public_label <> ''
ORDER BY r.public_sort_order, r.public_label, md.public_sort_order, md.public_label`

	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query public metrics: %w", err)
	}
	defer rows.Close()

	metrics := []PublicMetric{}
	for rows.Next() {
		var metric PublicMetric
		if err := rows.Scan(
			&metric.ResourceAlias,
			&metric.ResourceLabel,
			&metric.ResourceDescription,
			&metric.MetricAlias,
			&metric.MetricLabel,
			&metric.MetricDescription,
			&metric.Unit,
			&metric.LatestPointAt,
			&metric.RecentPointCount,
		); err != nil {
			return nil, fmt.Errorf("scan public metric: %w", err)
		}
		metric.ID = PublicMetricID(metric.ResourceAlias, metric.MetricAlias)
		metrics = append(metrics, metric)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate public metrics: %w", err)
	}
	return metrics, nil
}

func (s *Store) ListPublicMetricSeries(ctx context.Context, publicMetricID string, limit int32) ([]PublicMetricSeriesPoint, error) {
	resourceAlias, metricAlias, err := publicMetricAliasesFromID(publicMetricID)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 288
	}
	if limit > 1000 {
		limit = 1000
	}

	const query = `
SELECT mp.timestamp, mp.value
FROM metric_points mp
JOIN metric_definitions md ON md.id = mp.metric_definition_id
JOIN resources r
    ON r.resource_id = md.resource_id
    AND r.region = md.region
WHERE r.public_enabled = TRUE
  AND md.public_enabled = TRUE
  AND md.enabled = TRUE
  AND r.public_label = $1
  AND md.public_label = $2
ORDER BY mp.timestamp DESC
LIMIT $3`

	rows, err := s.pool.Query(ctx, query, resourceAlias, metricAlias, limit)
	if err != nil {
		return nil, fmt.Errorf("query public metric series: %w", err)
	}
	defer rows.Close()

	points, err := pgx.CollectRows(rows, pgx.RowToStructByPos[PublicMetricSeriesPoint])
	if err != nil {
		return nil, fmt.Errorf("scan public metric series: %w", err)
	}
	return points, nil
}

func (s *Store) ListAdminMetricDefinitions(ctx context.Context, region string) ([]AdminMetricDefinition, error) {
	const query = `
SELECT
    md.id,
    md.service_name,
    md.namespace,
    md.metric_name,
    md.resource_id,
    md.region,
    COALESCE(md.dimensions, '[]'::jsonb)::text,
    md.statistic,
    md.period_seconds,
    COALESCE(md.unit, ''),
    md.enabled,
    md.public_enabled,
    md.public_label,
    COALESCE(mcs.last_run_status, 'unknown'),
    COALESCE(mcs.last_success_at::text, ''),
    COALESCE(mcs.last_failure_at::text, ''),
    COALESCE(mcs.latest_point_at::text, ''),
    COALESCE(mcs.fetched_point_count, 0),
    COALESCE(mcs.inserted_point_count, 0),
    COALESCE(mcs.recent_point_count, 0),
    COALESCE(mcs.sanitized_error, '')
FROM metric_definitions md
LEFT JOIN metric_collection_status mcs ON mcs.metric_definition_id = md.id
WHERE ($1 = '' OR md.region = $1)
ORDER BY md.region, md.resource_id, md.namespace, md.metric_name, md.statistic, md.period_seconds`

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

func (s *Store) CollectionCostEstimate(ctx context.Context, region string, intervalSeconds int64) (CollectionCostEstimate, error) {
	if intervalSeconds <= 0 {
		intervalSeconds = 60
	}

	var enabledMetricCount int64
	var regionCount int64
	var costWarningMetricCount int64
	err := s.pool.QueryRow(ctx, `
SELECT
    COUNT(*) FILTER (WHERE md.enabled = TRUE) AS enabled_metric_count,
    COUNT(DISTINCT md.region) FILTER (WHERE md.enabled = TRUE) AS region_count,
    COUNT(*) FILTER (
        WHERE md.enabled = TRUE
          AND EXISTS (
              SELECT 1
              FROM discovered_metrics dm
              JOIN resources r ON r.id = dm.resource_id
              WHERE r.resource_id = md.resource_id
                AND dm.region = md.region
                AND dm.namespace = md.namespace
                AND dm.metric_name = md.metric_name
                AND dm.statistic = md.statistic
                AND dm.period_seconds = md.period_seconds
                AND dm.cost_warning <> ''
          )
    ) AS cost_warning_metric_count
FROM metric_definitions md
WHERE ($1 = '' OR md.region = $1)`, region).Scan(
		&enabledMetricCount,
		&regionCount,
		&costWarningMetricCount,
	)
	if err != nil {
		return CollectionCostEstimate{}, fmt.Errorf("query collection cost estimate: %w", err)
	}

	monthlyRuns := int64(30*24*60*60) / intervalSeconds
	monthlyMetricRequests := enabledMetricCount * monthlyRuns
	pricePerThousand := 0.01
	return CollectionCostEstimate{
		EnabledMetricCount:             enabledMetricCount,
		RegionCount:                    regionCount,
		CollectorIntervalSeconds:       intervalSeconds,
		MonthlyCollectionRunsPerRegion: monthlyRuns,
		MonthlyMetricRequests:          monthlyMetricRequests,
		GetMetricDataPricePerThousand:  pricePerThousand,
		EstimatedMonthlyCostUSD:        float64(monthlyMetricRequests) * pricePerThousand / 1000,
		CostWarningMetricCount:         costWarningMetricCount,
		PricingNote:                    "Estimate uses CloudWatch GetMetricData USD 0.01 per 1,000 metrics requested; pricing checked 2026-05-28.",
	}, nil
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

func (s *Store) UpdateMetricDefinitionPublicMetadata(ctx context.Context, id int64, input PublicMetadataInput) error {
	if input.PublicEnabled && strings.TrimSpace(input.PublicLabel) == "" {
		return fmt.Errorf("public_label is required when public_enabled is true")
	}
	tag, err := s.pool.Exec(ctx, `
UPDATE metric_definitions
SET public_enabled = $2,
    public_display_name = $3,
    public_description = $4,
    public_label = $5,
    public_sort_order = $6,
    updated_at = now()
WHERE id = $1`,
		id,
		input.PublicEnabled,
		strings.TrimSpace(input.PublicDisplayName),
		strings.TrimSpace(input.PublicDescription),
		strings.TrimSpace(input.PublicLabel),
		input.PublicSortOrder,
	)
	if err != nil {
		return fmt.Errorf("update metric definition public metadata: %w", err)
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

func (s *Store) SelectMetricCandidate(ctx context.Context, candidateID int64) (int64, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin metric candidate selection: %w", err)
	}
	defer tx.Rollback(ctx)

	var candidate AdminMetricCandidate
	err = tx.QueryRow(ctx, `
SELECT
    dm.id,
    dm.resource_id,
    r.service_name,
    r.resource_id,
    r.display_name,
    dm.namespace,
    dm.metric_name,
    dm.dimensions::text,
    dm.statistic,
    dm.period_seconds,
    COALESCE(dm.unit, ''),
    dm.region,
    dm.selected,
    dm.availability_status,
    dm.availability_reason,
    dm.provider_source,
    dm.prerequisite,
    dm.cost_warning
FROM discovered_metrics dm
JOIN resources r ON r.id = dm.resource_id
WHERE dm.id = $1`, candidateID).Scan(
		&candidate.ID,
		&candidate.ResourceID,
		&candidate.ServiceName,
		&candidate.ResourceIdentifier,
		&candidate.DisplayName,
		&candidate.Namespace,
		&candidate.MetricName,
		&candidate.DimensionsJSON,
		&candidate.Statistic,
		&candidate.PeriodSeconds,
		&candidate.Unit,
		&candidate.Region,
		&candidate.Selected,
		&candidate.AvailabilityStatus,
		&candidate.AvailabilityReason,
		&candidate.ProviderSource,
		&candidate.Prerequisite,
		&candidate.CostWarning,
	)
	if err == pgx.ErrNoRows {
		return 0, fmt.Errorf("metric candidate not found")
	}
	if err != nil {
		return 0, fmt.Errorf("query metric candidate: %w", err)
	}
	if candidate.AvailabilityStatus != discovery.AvailabilityAvailable {
		return 0, fmt.Errorf("metric candidate is not available: %s", candidate.AvailabilityStatus)
	}

	var definitionID int64
	err = tx.QueryRow(ctx, `
INSERT INTO metric_definitions (service_name, namespace, metric_name, resource_id, region, dimensions, statistic, period_seconds, unit, enabled)
VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7, $8, $9, TRUE)
ON CONFLICT ON CONSTRAINT metric_definitions_unique_metric DO UPDATE SET
    service_name = EXCLUDED.service_name,
    dimensions = EXCLUDED.dimensions,
    unit = EXCLUDED.unit,
    enabled = TRUE,
    updated_at = now()
RETURNING id`,
		candidate.ServiceName,
		candidate.Namespace,
		candidate.MetricName,
		candidate.ResourceIdentifier,
		candidate.Region,
		candidate.DimensionsJSON,
		candidate.Statistic,
		candidate.PeriodSeconds,
		candidate.Unit,
	).Scan(&definitionID)
	if err != nil {
		return 0, fmt.Errorf("upsert metric definition from candidate: %w", err)
	}

	if _, err := tx.Exec(ctx, `
UPDATE discovered_metrics
SET selected = TRUE, updated_at = now()
WHERE id = $1`, candidateID); err != nil {
		return 0, fmt.Errorf("mark metric candidate selected: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit metric candidate selection: %w", err)
	}
	return definitionID, nil
}

func (s *Store) ApplyRecommendedMetricSet(ctx context.Context, resourceRowID int64, metrics []RecommendedMetric) (int64, error) {
	var resource AdminResource
	err := s.pool.QueryRow(ctx, `
SELECT
    id,
    service_name,
    namespace,
    resource_id,
    region,
    display_name,
    tags::text,
    COALESCE(provider_source, ''),
    COALESCE(discovery_source, ''),
    COALESCE(NULLIF(internal_region_label, ''), region),
    public_enabled,
    public_display_name,
    public_description,
    public_label,
    public_sort_order,
    enabled,
    0::bigint,
    0::bigint,
    0::bigint,
    0::bigint
FROM resources
WHERE id = $1`, resourceRowID).Scan(
		&resource.ID,
		&resource.ServiceName,
		&resource.Namespace,
		&resource.ResourceID,
		&resource.Region,
		&resource.DisplayName,
		&resource.TagsJSON,
		&resource.ProviderSource,
		&resource.DiscoverySource,
		&resource.InternalRegion,
		&resource.PublicEnabled,
		&resource.PublicDisplayName,
		&resource.PublicDescription,
		&resource.PublicLabel,
		&resource.PublicSortOrder,
		&resource.Enabled,
		&resource.DiscoveredMetrics,
		&resource.AvailableMetrics,
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
INSERT INTO resources (
    service_name,
    namespace,
    resource_id,
    region,
    display_name,
    tags,
    provider_source,
    discovery_source,
    internal_region_label,
    enabled
)
VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7, $8, $9, FALSE)
ON CONFLICT ON CONSTRAINT resources_unique_resource DO UPDATE SET
    namespace = EXCLUDED.namespace,
    display_name = EXCLUDED.display_name,
    tags = EXCLUDED.tags,
    provider_source = EXCLUDED.provider_source,
    discovery_source = EXCLUDED.discovery_source,
    internal_region_label = EXCLUDED.internal_region_label,
    updated_at = now()
RETURNING id`,
			resource.ServiceName,
			resource.Namespace,
			resource.ResourceID,
			resource.Region,
			resource.DisplayName,
			tagsJSON,
			resource.ProviderSource,
			defaultString(resource.ProviderSource, "cloudwatch-listmetrics"),
			resource.Region,
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
INSERT INTO discovered_metrics (
    resource_id,
    namespace,
    metric_name,
    dimensions,
    statistic,
    period_seconds,
    unit,
    region,
    selected,
    availability_status,
    availability_reason,
    provider_source,
    prerequisite,
    cost_warning
)
VALUES ($1, $2, $3, $4::jsonb, $5, $6, $7, $8, FALSE, $9, $10, $11, $12, $13)
ON CONFLICT ON CONSTRAINT discovered_metrics_unique_metric DO UPDATE SET
    unit = EXCLUDED.unit,
    region = EXCLUDED.region,
    availability_status = EXCLUDED.availability_status,
    availability_reason = EXCLUDED.availability_reason,
    provider_source = EXCLUDED.provider_source,
    prerequisite = EXCLUDED.prerequisite,
    cost_warning = EXCLUDED.cost_warning,
    updated_at = now()`,
				resourceRowID,
				metric.Namespace,
				metric.MetricName,
				dimensionsJSON,
				metric.Statistic,
				metric.PeriodSeconds,
				metric.Unit,
				resource.Region,
				defaultString(metric.AvailabilityStatus, discovery.AvailabilityAvailable),
				metric.AvailabilityReason,
				defaultString(metric.ProviderSource, resource.ProviderSource),
				metric.Prerequisite,
				metric.CostWarning,
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

func defaultString(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
