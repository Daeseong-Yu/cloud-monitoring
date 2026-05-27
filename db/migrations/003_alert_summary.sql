CREATE TABLE IF NOT EXISTS alert_rules (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    region TEXT NOT NULL,
    namespace TEXT NOT NULL,
    resource_id TEXT NOT NULL,
    metric_name TEXT NOT NULL,
    statistic TEXT NOT NULL,
    operator TEXT NOT NULL,
    threshold DOUBLE PRECISION NOT NULL,
    lookback_minutes INTEGER NOT NULL DEFAULT 15,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT alert_rules_operator_valid CHECK (
        operator IN ('gt', 'gte', 'lt', 'lte')
    ),
    CONSTRAINT alert_rules_lookback_positive CHECK (lookback_minutes > 0)
);

CREATE TABLE IF NOT EXISTS alert_events (
    id BIGSERIAL PRIMARY KEY,
    alert_rule_id BIGINT NOT NULL REFERENCES alert_rules(id) ON DELETE CASCADE,
    status TEXT NOT NULL,
    value DOUBLE PRECISION NOT NULL,
    threshold DOUBLE PRECISION NOT NULL,
    message TEXT NOT NULL,
    opened_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT alert_events_status_valid CHECK (
        status IN ('open', 'resolved')
    )
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_alert_events_one_open
ON alert_events (alert_rule_id)
WHERE status = 'open';

CREATE INDEX IF NOT EXISTS idx_alert_rules_enabled_region
ON alert_rules (enabled, region);

CREATE INDEX IF NOT EXISTS idx_alert_events_rule_status
ON alert_events (alert_rule_id, status);

CREATE TABLE IF NOT EXISTS metric_hourly_summary (
    metric_definition_id BIGINT NOT NULL REFERENCES metric_definitions(id) ON DELETE CASCADE,
    bucket TIMESTAMPTZ NOT NULL,
    min_value DOUBLE PRECISION NOT NULL,
    max_value DOUBLE PRECISION NOT NULL,
    avg_value DOUBLE PRECISION NOT NULL,
    sample_count BIGINT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT metric_hourly_summary_unique UNIQUE (
        metric_definition_id,
        bucket
    )
);

CREATE TABLE IF NOT EXISTS metric_daily_summary (
    metric_definition_id BIGINT NOT NULL REFERENCES metric_definitions(id) ON DELETE CASCADE,
    bucket DATE NOT NULL,
    min_value DOUBLE PRECISION NOT NULL,
    max_value DOUBLE PRECISION NOT NULL,
    avg_value DOUBLE PRECISION NOT NULL,
    sample_count BIGINT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT metric_daily_summary_unique UNIQUE (
        metric_definition_id,
        bucket
    )
);

CREATE INDEX IF NOT EXISTS idx_metric_hourly_summary_bucket
ON metric_hourly_summary (bucket DESC);

CREATE INDEX IF NOT EXISTS idx_metric_daily_summary_bucket
ON metric_daily_summary (bucket DESC);
