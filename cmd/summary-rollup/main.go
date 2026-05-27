package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"cloud-monitor/internal/sanitize"
)

func main() {
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		fmt.Fprintln(os.Stderr, "summary rollup configuration error: DATABASE_URL is required")
		os.Exit(2)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "summary rollup database error: %s\n", sanitize.Message(err.Error(), databaseURL))
		os.Exit(1)
	}
	defer pool.Close()

	hourly, err := rollupHourly(ctx, pool)
	if err != nil {
		fmt.Fprintf(os.Stderr, "summary hourly rollup error: %s\n", sanitize.Message(err.Error(), databaseURL))
		os.Exit(1)
	}
	daily, err := rollupDaily(ctx, pool)
	if err != nil {
		fmt.Fprintf(os.Stderr, "summary daily rollup error: %s\n", sanitize.Message(err.Error(), databaseURL))
		os.Exit(1)
	}

	fmt.Printf("summary rollup completed: hourly=%d daily=%d\n", hourly, daily)
}

func rollupHourly(ctx context.Context, pool *pgxpool.Pool) (int64, error) {
	tag, err := pool.Exec(ctx, `
INSERT INTO metric_hourly_summary (
    metric_definition_id,
    bucket,
    min_value,
    max_value,
    avg_value,
    sample_count
)
SELECT
    metric_definition_id,
    date_trunc('hour', timestamp) AS bucket,
    min(value) AS min_value,
    max(value) AS max_value,
    avg(value) AS avg_value,
    count(*) AS sample_count
FROM metric_points
GROUP BY metric_definition_id, date_trunc('hour', timestamp)
ON CONFLICT ON CONSTRAINT metric_hourly_summary_unique DO UPDATE SET
    min_value = EXCLUDED.min_value,
    max_value = EXCLUDED.max_value,
    avg_value = EXCLUDED.avg_value,
    sample_count = EXCLUDED.sample_count,
    updated_at = now()`)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func rollupDaily(ctx context.Context, pool *pgxpool.Pool) (int64, error) {
	tag, err := pool.Exec(ctx, `
INSERT INTO metric_daily_summary (
    metric_definition_id,
    bucket,
    min_value,
    max_value,
    avg_value,
    sample_count
)
SELECT
    metric_definition_id,
    timestamp::date AS bucket,
    min(value) AS min_value,
    max(value) AS max_value,
    avg(value) AS avg_value,
    count(*) AS sample_count
FROM metric_points
GROUP BY metric_definition_id, timestamp::date
ON CONFLICT ON CONSTRAINT metric_daily_summary_unique DO UPDATE SET
    min_value = EXCLUDED.min_value,
    max_value = EXCLUDED.max_value,
    avg_value = EXCLUDED.avg_value,
    sample_count = EXCLUDED.sample_count,
    updated_at = now()`)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
