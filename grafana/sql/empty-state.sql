SELECT
    md.metric_name,
    md.resource_id,
    md.region,
    count(mp.id) AS points
FROM metric_definitions md
LEFT JOIN metric_points mp
    ON mp.metric_definition_id = md.id
    AND $__timeFilter(mp.timestamp)
WHERE
    md.enabled = TRUE
    AND md.resource_id = ${resource_id:sqlstring}
    AND md.region = ${region:sqlstring}
GROUP BY md.metric_name, md.resource_id, md.region
ORDER BY md.metric_name;
