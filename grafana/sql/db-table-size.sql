SELECT
    schemaname || '.' || relname AS table_name,
    pg_total_relation_size(quote_ident(schemaname) || '.' || quote_ident(relname)) AS total_bytes
FROM pg_stat_user_tables
WHERE schemaname = 'public'
ORDER BY total_bytes DESC;
