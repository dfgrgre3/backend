package main
import (
  "fmt"; "log"; "thanawy-backend/internal/config"; "thanawy-backend/internal/db"; "github.com/joho/godotenv"
)
type col struct{ TableName string; ColumnName string }
func main(){ _=godotenv.Load(); cfg:=config.Load(); database,err:=db.Connect(cfg.DatabaseURL); if err!=nil{log.Fatal(err)}; var cols []col; database.Raw(`SELECT table_name, column_name FROM information_schema.columns WHERE table_schema='public' AND table_name IN ('User','UserSettings') ORDER BY table_name,column_name`).Scan(&cols); for _,c:= range cols{fmt.Println(c.TableName, c.ColumnName)} }
