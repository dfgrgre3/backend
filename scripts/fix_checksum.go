//go:build ignore

// One-off: align the recorded checksum of an applied migration with the
// current file bytes (used when line-ending normalization changed the file
// after apply). Usage: go run scripts/fix_checksum.go <migration_id>
package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	id := os.Args[1]
	path := filepath.Join("internal", "infrastructure", "database", "migration", "migrations", id+".sql")
	contents, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("read %s: %v", path, err)
	}
	sum := sha256.Sum256(contents)
	checksum := hex.EncodeToString(sum[:])

	dsn := os.Args[2]
	sqldb, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer sqldb.Close()

	res, err := sqldb.Exec(`UPDATE "schema_migrations" SET checksum = $1 WHERE id = $2`, checksum, id)
	if err != nil {
		log.Fatalf("update: %v", err)
	}
	n, _ := res.RowsAffected()
	fmt.Printf("updated %d row(s); id=%s checksum=%s...\n", n, id, checksum[:12])
}
