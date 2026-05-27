SELECT
    md.region,
    md.namespace,
    count(DISTINCT md.resource_id) AS resources,
    count(DISTINCT md.id) AS definitions,
    count(mp.id) AS points
FROM metric_definitions md
LEFT JOIN metric_points mp
    ON mp.metric_definition_id = md.id
    AND $__timeFilter(mp.timestamp)
GROUP BY md.region, md.namespace
ORDER BY md.region, md.namespace;
