SELECT
    mp.timestamp AS "time",
    mp.value AS value,
    md.metric_name AS metric
FROM metric_points mp
JOIN metric_definitions md
    ON md.id = mp.metric_definition_id
WHERE
    md.metric_name = 'NetworkIn'
    AND md.resource_id = ${resource_id:sqlstring}
    AND md.region = ${region:sqlstring}
    AND $__timeFilter(mp.timestamp)
ORDER BY mp.timestamp;
