DELETE FROM metric_points
WHERE timestamp < now() - interval '30 days';
