ALTER TABLE resources
    ADD COLUMN IF NOT EXISTS discovery_source TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS arn TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS account_id TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS internal_region_label TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS public_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS public_display_name TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS public_description TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS public_label TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS public_sort_order INTEGER NOT NULL DEFAULT 0;

ALTER TABLE metric_definitions
    ADD COLUMN IF NOT EXISTS public_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS public_display_name TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS public_description TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS public_label TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS public_sort_order INTEGER NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS metric_collection_status (
    metric_definition_id BIGINT PRIMARY KEY REFERENCES metric_definitions(id) ON DELETE CASCADE,
    last_success_at TIMESTAMPTZ,
    last_failure_at TIMESTAMPTZ,
    latest_point_at TIMESTAMPTZ,
    recent_point_count BIGINT NOT NULL DEFAULT 0,
    sanitized_error TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_resources_public_enabled
ON resources (public_enabled)
WHERE public_enabled = TRUE;

CREATE INDEX IF NOT EXISTS idx_metric_definitions_public_enabled
ON metric_definitions (public_enabled)
WHERE public_enabled = TRUE;

CREATE INDEX IF NOT EXISTS idx_metric_collection_status_success
ON metric_collection_status (last_success_at DESC);

CREATE OR REPLACE VIEW public_portfolio_metric_view AS
SELECT
    r.service_name,
    md.namespace,
    md.metric_name,
    md.statistic,
    md.period_seconds,
    COALESCE(md.unit, '') AS unit,
    r.public_display_name AS resource_display_name,
    r.public_description AS resource_description,
    r.public_label AS resource_label,
    r.public_sort_order AS resource_sort_order,
    md.public_display_name AS metric_display_name,
    md.public_description AS metric_description,
    md.public_label AS metric_label,
    md.public_sort_order AS metric_sort_order,
    mcs.latest_point_at,
    mcs.recent_point_count
FROM resources r
JOIN metric_definitions md
    ON md.resource_id = r.resource_id
    AND md.region = r.region
LEFT JOIN metric_collection_status mcs
    ON mcs.metric_definition_id = md.id
WHERE r.public_enabled = TRUE
  AND md.public_enabled = TRUE
  AND r.public_label <> ''
  AND md.public_label <> '';
