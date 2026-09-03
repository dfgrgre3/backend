//go:build ignore

package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	dsn := os.Args[1]
	sqldb, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer sqldb.Close()

	dump := func(title, query string) {
		rows, err := sqldb.Query(query)
		if err != nil {
			fmt.Printf("%s: ERROR %v\n", title, err)
			return
		}
		defer rows.Close()
		cols, _ := rows.Columns()
		fmt.Printf("%s (%s):\n", title, fmt.Sprint(cols))
		vals := make([]any, len(cols))
		for i := range vals {
			var s sql.NullString
			vals[i] = &s
		}
		for rows.Next() {
			if err := rows.Scan(vals...); err != nil {
				fmt.Printf("  scan err: %v\n", err)
				continue
			}
			out := ""
			for i, v := range vals {
				s := v.(*sql.NullString)
				if i > 0 {
					out += " | "
				}
				out += s.String
			}
			fmt.Printf("  %s\n", out)
		}
	}

	dump("current_user/database", "SELECT current_user, current_database()")
	if os.Getenv("TEST_ROLE") != "" {
		if _, err := sqldb.Exec("SET ROLE app_user"); err != nil {
			log.Fatalf("SET ROLE: %v", err)
		}
		dump("as app_user: current_user", "SELECT current_user")
		dump("as app_user: REFRESH CONCURRENTLY", "REFRESH MATERIALIZED VIEW CONCURRENTLY mv_user_progress_summary")
		return
	}
	dump("materialized views", `SELECT matviewname, matviewowner FROM pg_matviews WHERE schemaname = 'public' ORDER BY matviewname`)
	dump("roles", `SELECT rolname, rolsuper::text, rolcanlogin::text FROM pg_roles WHERE rolname IN ('app_user','migration_user','thanawy','postgres') ORDER BY rolname`)
	dump("role memberships", `SELECT m.rolname, g.rolname FROM pg_auth_members am JOIN pg_roles m ON m.oid = am.member JOIN pg_roles g ON g.oid = am.roleid`)
	dump("session tables / deleted_at", `SELECT table_name, column_name FROM information_schema.columns WHERE table_name ILIKE '%session%' AND column_name IN ('deleted_at','is_active','status') ORDER BY table_name`)
	dump("tables with deleted_at (affected by 0176)", `SELECT table_name FROM information_schema.columns WHERE table_schema='public' AND column_name = 'deleted_at' ORDER BY table_name`)
}
