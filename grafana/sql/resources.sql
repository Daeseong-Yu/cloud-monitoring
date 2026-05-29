SELECT DISTINCT
    COALESCE(NULLIF(r.display_name, ''), md.resource_id) AS __text,
    md.resource_id AS __value
FROM metric_definitions md
JOIN resources r
    ON r.service_name = md.service_name
    AND r.resource_id = md.resource_id
    AND r.region = md.region
WHERE
    md.enabled = TRUE
    AND md.region = ${region:sqlstring}
ORDER BY __text;
