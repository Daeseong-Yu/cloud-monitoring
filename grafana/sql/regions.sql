SELECT DISTINCT
    region AS __text,
    region AS __value
FROM metric_definitions
WHERE enabled = TRUE
ORDER BY region;
