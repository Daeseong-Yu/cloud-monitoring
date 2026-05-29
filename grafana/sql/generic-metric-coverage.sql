SELECT
    md.region,
    md.namespace,
    COALESCE(NULLIF(r.display_name, ''), md.resource_id) AS resource,
    md.resource_id,
    md.metric_name,
    md.statistic,
    md.period_seconds,
    md.enabled,
    count(mp.id) AS points,
    max(mp.timestamp) AS latest_point
FROM metric_definitions md
JOIN resources r
    ON r.service_name = md.service_name
    AND r.resource_id = md.resource_id
    AND r.region = md.region
LEFT JOIN metric_points mp
    ON mp.metric_definition_id = md.id
    AND $__timeFilter(mp.timestamp)
WHERE
    md.region = ${region:sqlstring}
    AND md.namespace = ${namespace:sqlstring}
    AND md.resource_id IN (${resource_id:sqlstring})
GROUP BY
    md.region,
    md.namespace,
    r.display_name,
    md.resource_id,
    md.metric_name,
    md.statistic,
    md.period_seconds,
    md.enabled
ORDER BY resource, md.metric_name;
