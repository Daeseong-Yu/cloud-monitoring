SELECT
    md.metric_name,
    COALESCE(NULLIF(r.display_name, ''), md.resource_id) AS resource,
    md.resource_id,
    md.region,
    count(mp.id) AS points
FROM metric_definitions md
JOIN resources r
    ON r.service_name = md.service_name
    AND r.resource_id = md.resource_id
    AND r.region = md.region
LEFT JOIN metric_points mp
    ON mp.metric_definition_id = md.id
    AND $__timeFilter(mp.timestamp)
WHERE
    md.enabled = TRUE
    AND md.resource_id IN (${resource_id:sqlstring})
    AND md.region = ${region:sqlstring}
GROUP BY md.metric_name, r.display_name, md.resource_id, md.region
ORDER BY resource, md.metric_name;
