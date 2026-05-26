package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type MetricDefinition struct {
	ID            int64
	ServiceName   string
	Namespace     string
	MetricName    string
	ResourceID    string
	Region        string
	Statistic     string
	PeriodSeconds int32
	Unit          string
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
