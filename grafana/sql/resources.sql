SELECT DISTINCT
    resource_id AS __text,
    resource_id AS __value
FROM metric_definitions
WHERE
    enabled = TRUE
    AND region = ${region:sqlstring}
ORDER BY resource_id;
