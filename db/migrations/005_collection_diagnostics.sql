ALTER TABLE metric_collection_status
    ADD COLUMN IF NOT EXISTS fetched_point_count BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS inserted_point_count BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS last_run_status TEXT NOT NULL DEFAULT 'unknown';

ALTER TABLE metric_collection_status
    DROP CONSTRAINT IF EXISTS metric_collection_status_run_status_valid;

ALTER TABLE metric_collection_status
    ADD CONSTRAINT metric_collection_status_run_status_valid CHECK (
        last_run_status IN ('unknown', 'success', 'failure')
    );

CREATE INDEX IF NOT EXISTS idx_metric_collection_status_run_status
ON metric_collection_status (last_run_status);
