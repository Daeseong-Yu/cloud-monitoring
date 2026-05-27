SELECT
    md.region,
    md.namespace,
    md.resource_id,
    md.metric_name,
    md.statistic,
    md.period_seconds,
    md.enabled,
    count(mp.id) AS points,
    max(mp.timestamp) AS latest_point
FROM metric_definitions md
LEFT JOIN metric_points mp
    ON mp.metric_definition_id = md.id
    AND $__timeFilter(mp.timestamp)
WHERE
    md.region = ${region:sqlstring}
    AND md.namespace = ${namespace:sqlstring}
    AND md.resource_id = ${resource_id:sqlstring}
GROUP BY
    md.region,
    md.namespace,
    md.resource_id,
    md.metric_name,
    md.statistic,
    md.period_seconds,
    md.enabled
ORDER BY md.metric_name;
