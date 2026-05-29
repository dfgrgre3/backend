package main
import (
  "fmt"
  "log"
  "thanawy-backend/internal/config"
  "thanawy-backend/internal/db"
  "github.com/joho/godotenv"
)
type lockRow struct{ PID int; Granted bool; State string; Query string }
func main(){
  _ = godotenv.Load()
  cfg := config.Load()
  database, err := db.Connect(cfg.DatabaseURL)
  if err != nil { log.Fatal(err) }
  var rows []lockRow
  q := `SELECT a.pid, l.granted, COALESCE(a.state,'') state, COALESCE(a.query,'') query
        FROM pg_locks l
        LEFT JOIN pg_stat_activity a ON a.pid = l.pid
        WHERE l.locktype = 'advisory'
        ORDER BY l.granted DESC, a.pid`
  if err := database.Raw(q).Scan(&rows).Error; err != nil { log.Fatal(err) }
  for _, r := range rows { fmt.Printf("pid=%d granted=%v state=%s query=%s\n", r.PID, r.Granted, r.State, r.Query) }
}
