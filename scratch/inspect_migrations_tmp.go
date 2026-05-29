package main
import (
  "fmt"
  "log"
  "thanawy-backend/internal/config"
  "thanawy-backend/internal/db"
  "github.com/joho/godotenv"
)
type row struct{ ID string; AppliedAt string }
func main(){
  _ = godotenv.Load()
  cfg := config.Load()
  database, err := db.Connect(cfg.DatabaseURL)
  if err != nil { log.Fatal(err) }
  var rows []row
  if err := database.Raw(`SELECT id, "appliedAt"::text AS applied_at FROM schema_migrations ORDER BY id`).Scan(&rows).Error; err != nil { log.Fatal(err) }
  for _, r := range rows { fmt.Println(r.ID, r.AppliedAt) }
}
