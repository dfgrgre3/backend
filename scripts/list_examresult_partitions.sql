SELECT inhrelid::regclass::text AS partition,
       pg_get_expr(c.relpartbound, c.oid) AS bound
FROM pg_inherits i
JOIN pg_class c ON c.oid = i.inhrelid
WHERE inhparent = 'public."ExamResult"'::regclass
ORDER BY 1;