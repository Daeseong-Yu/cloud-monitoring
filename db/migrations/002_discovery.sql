CREATE TABLE IF NOT EXISTS resources (
    id BIGSERIAL PRIMARY KEY,
    service_name TEXT NOT NULL,
    namespace TEXT NOT NULL,
    resource_id TEXT NOT NULL,
    region TEXT NOT NULL,
    display_name TEXT NOT NULL DEFAULT '',
    tags JSONB NOT NULL DEFAULT '{}'::jsonb,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    discovered_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT resources_unique_resource UNIQUE (
        service_name,
        resource_id,
        region
    )
);

ALTER TABLE metric_definitions
    ADD COLUMN IF NOT EXISTS dimensions JSONB NOT NULL DEFAULT '[]'::jsonb;

ALTER TABLE metric_definitions
    DROP CONSTRAINT IF EXISTS metric_definitions_unique_metric;

ALTER TABLE metric_definitions
    ADD CONSTRAINT metric_definitions_unique_metric UNIQUE (
        namespace,
        metric_name,
        resource_id,
        region,
        dimensions,
        statistic,
        period_seconds
    );

CREATE TABLE IF NOT EXISTS discovered_metrics (
    id BIGSERIAL PRIMARY KEY,
    resource_id BIGINT NOT NULL REFERENCES resources(id) ON DELETE CASCADE,
    namespace TEXT NOT NULL,
    metric_name TEXT NOT NULL,
    dimensions JSONB NOT NULL DEFAULT '[]'::jsonb,
    statistic TEXT NOT NULL DEFAULT 'Average',
    period_seconds INTEGER NOT NULL DEFAULT 300,
    unit TEXT,
    region TEXT NOT NULL,
    selected BOOLEAN NOT NULL DEFAULT FALSE,
    discovered_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT discovered_metrics_unique_metric UNIQUE (
        resource_id,
        namespace,
        metric_name,
        dimensions,
        statistic,
        period_seconds
    )
);

CREATE INDEX IF NOT EXISTS idx_resources_region_service
ON resources (region, service_name);

CREATE INDEX IF NOT EXISTS idx_resources_enabled
ON resources (enabled);

CREATE INDEX IF NOT EXISTS idx_discovered_metrics_selected
ON discovered_metrics (selected);

CREATE INDEX IF NOT EXISTS idx_metric_definitions_dimensions
ON metric_definitions USING GIN (dimensions);
