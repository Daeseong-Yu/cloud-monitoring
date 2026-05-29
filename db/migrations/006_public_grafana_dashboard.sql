CREATE OR REPLACE VIEW public_grafana_default_metric_catalog AS
SELECT
    service_name,
    namespace,
    metric_name,
    statistic,
    period_seconds
FROM (VALUES
    ('amplify', 'AWS/AmplifyHosting', '4xxErrors', 'Sum', 300),
    ('amplify', 'AWS/AmplifyHosting', '5xxErrors', 'Sum', 300),
    ('amplify', 'AWS/AmplifyHosting', 'Latency', 'Average', 300),
    ('amplify', 'AWS/AmplifyHosting', 'Requests', 'Sum', 300),
    ('api-gateway', 'AWS/ApiGateway', '4XXError', 'Sum', 300),
    ('api-gateway', 'AWS/ApiGateway', '5XXError', 'Sum', 300),
    ('api-gateway', 'AWS/ApiGateway', 'Count', 'Sum', 300),
    ('api-gateway', 'AWS/ApiGateway', 'Latency', 'Average', 300),
    ('ec2', 'AWS/EC2', 'CPUUtilization', 'Average', 300),
    ('ec2', 'AWS/EC2', 'NetworkIn', 'Sum', 300),
    ('ec2', 'AWS/EC2', 'NetworkOut', 'Sum', 300),
    ('ec2', 'AWS/EC2', 'StatusCheckFailed', 'Maximum', 300),
    ('ec2', 'CWAgent', 'disk_used_percent', 'Average', 300),
    ('ec2', 'CWAgent', 'mem_used_percent', 'Average', 300),
    ('lambda', 'AWS/Lambda', 'Duration', 'Average', 300),
    ('lambda', 'AWS/Lambda', 'Errors', 'Sum', 300),
    ('lambda', 'AWS/Lambda', 'Invocations', 'Sum', 300),
    ('lambda', 'AWS/Lambda', 'Throttles', 'Sum', 300),
    ('s3', 'AWS/S3', '4xxErrors', 'Sum', 300),
    ('s3', 'AWS/S3', '5xxErrors', 'Sum', 300),
    ('s3', 'AWS/S3', 'AllRequests', 'Sum', 300),
    ('s3', 'AWS/S3', 'BucketSizeBytes', 'Average', 86400),
    ('s3', 'AWS/S3', 'NumberOfObjects', 'Average', 86400),
    ('ses', 'AWS/SES', 'Bounce', 'Sum', 300),
    ('ses', 'AWS/SES', 'Complaint', 'Sum', 300),
    ('ses', 'AWS/SES', 'Delivery', 'Sum', 300),
    ('ses', 'AWS/SES', 'Send', 'Sum', 300)
) AS catalog(service_name, namespace, metric_name, statistic, period_seconds);

CREATE OR REPLACE VIEW public_grafana_metric_summary AS
SELECT
    initcap(replace(r.service_name, '-', ' ')) AS service_label,
    COALESCE(NULLIF(r.public_display_name, ''), r.public_label) AS resource_label,
    r.public_description AS resource_description,
    COALESCE(NULLIF(md.public_display_name, ''), md.public_label) AS metric_label,
    md.public_description AS metric_description,
    COALESCE(NULLIF(md.unit, ''), '') AS unit,
    concat(
        COALESCE(NULLIF(r.public_display_name, ''), r.public_label),
        ' / ',
        COALESCE(NULLIF(md.public_display_name, ''), md.public_label)
    ) AS series_label,
    r.public_sort_order AS resource_sort_order,
    md.public_sort_order AS metric_sort_order,
    mcs.latest_point_at,
    COALESCE(mcs.recent_point_count, 0) AS recent_point_count
FROM resources r
JOIN metric_definitions md
    ON md.service_name = r.service_name
    AND md.resource_id = r.resource_id
    AND md.region = r.region
JOIN public_grafana_default_metric_catalog catalog
    ON catalog.service_name = md.service_name
    AND catalog.namespace = md.namespace
    AND catalog.metric_name = md.metric_name
    AND catalog.statistic = md.statistic
    AND catalog.period_seconds = md.period_seconds
LEFT JOIN metric_collection_status mcs
    ON mcs.metric_definition_id = md.id
WHERE r.public_enabled = TRUE
  AND r.enabled = TRUE
  AND md.public_enabled = TRUE
  AND md.enabled = TRUE
  AND r.public_label <> ''
  AND md.public_label <> '';

CREATE OR REPLACE VIEW public_grafana_metric_points AS
SELECT
    mp.timestamp AS "time",
    mp.value AS value,
    initcap(replace(r.service_name, '-', ' ')) AS service_label,
    COALESCE(NULLIF(r.public_display_name, ''), r.public_label) AS resource_label,
    COALESCE(NULLIF(md.public_display_name, ''), md.public_label) AS metric_label,
    COALESCE(NULLIF(md.unit, ''), '') AS unit,
    concat(
        COALESCE(NULLIF(r.public_display_name, ''), r.public_label),
        ' / ',
        COALESCE(NULLIF(md.public_display_name, ''), md.public_label)
    ) AS series_label,
    r.public_sort_order AS resource_sort_order,
    md.public_sort_order AS metric_sort_order
FROM metric_points mp
JOIN metric_definitions md
    ON md.id = mp.metric_definition_id
JOIN resources r
    ON r.service_name = md.service_name
    AND r.resource_id = md.resource_id
    AND r.region = md.region
JOIN public_grafana_default_metric_catalog catalog
    ON catalog.service_name = md.service_name
    AND catalog.namespace = md.namespace
    AND catalog.metric_name = md.metric_name
    AND catalog.statistic = md.statistic
    AND catalog.period_seconds = md.period_seconds
WHERE r.public_enabled = TRUE
  AND r.enabled = TRUE
  AND md.public_enabled = TRUE
  AND md.enabled = TRUE
  AND r.public_label <> ''
  AND md.public_label <> '';
