BEGIN;

CREATE TABLE IF NOT EXISTS metric_definitions (
    id BIGSERIAL PRIMARY KEY,
    service_name TEXT NOT NULL,
    namespace TEXT NOT NULL,
    metric_name TEXT NOT NULL,
    resource_id TEXT NOT NULL,
    region TEXT NOT NULL,
    statistic TEXT NOT NULL,
    period_seconds INTEGER NOT NULL,
    unit TEXT,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT metric_definitions_unique_metric UNIQUE (
        namespace,
        metric_name,
        resource_id,
        region,
        statistic,
        period_seconds
    ),
    CONSTRAINT metric_definitions_period_positive CHECK (period_seconds > 0)
);

CREATE TABLE IF NOT EXISTS metric_points (
    id BIGSERIAL PRIMARY KEY,
    metric_definition_id BIGINT NOT NULL REFERENCES metric_definitions(id) ON DELETE CASCADE,
    timestamp TIMESTAMPTZ NOT NULL,
    value DOUBLE PRECISION NOT NULL,
    collected_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT metric_points_unique_point UNIQUE (
        metric_definition_id,
        timestamp
    )
);

CREATE INDEX IF NOT EXISTS idx_metric_points_definition_time
ON metric_points (metric_definition_id, timestamp DESC);

CREATE INDEX IF NOT EXISTS idx_metric_points_time
ON metric_points (timestamp DESC);

CREATE INDEX IF NOT EXISTS idx_metric_definitions_enabled
ON metric_definitions (enabled);

CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_metric_definitions_updated_at ON metric_definitions;

CREATE TRIGGER trg_metric_definitions_updated_at
BEFORE UPDATE ON metric_definitions
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

COMMIT;
