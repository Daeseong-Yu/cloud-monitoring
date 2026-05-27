SELECT
    now() AS "time",
    pg_database_size(current_database())::double precision AS value,
    current_database() AS metric;
