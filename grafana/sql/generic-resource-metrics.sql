SELECT
    mp.timestamp AS "time",
    mp.value AS value,
    COALESCE(NULLIF(r.display_name, ''), md.resource_id) AS metric
FROM metric_points mp
JOIN metric_definitions md
    ON md.id = mp.metric_definition_id
JOIN resources r
    ON r.service_name = md.service_name
    AND r.resource_id = md.resource_id
    AND r.region = md.region
WHERE
    md.enabled = TRUE
    AND md.region = ${region:sqlstring}
    AND md.namespace = ${namespace:sqlstring}
    AND md.resource_id IN (${resource_id:sqlstring})
    AND md.metric_name = ${metric_name:sqlstring}
    AND $__timeFilter(mp.timestamp)
ORDER BY mp.timestamp;
