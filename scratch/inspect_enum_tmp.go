package main
import (
  "fmt"; "log"; "thanawy-backend/internal/config"; "thanawy-backend/internal/db"; "github.com/joho/godotenv"
)
func main(){ _=godotenv.Load(); cfg:=config.Load(); database,err:=db.Connect(cfg.DatabaseURL); if err!=nil{log.Fatal(err)}; var vals []string; if err:=database.Raw(`SELECT enumlabel FROM pg_enum JOIN pg_type ON pg_type.oid=pg_enum.enumtypid WHERE typname='WalletTransactionType' ORDER BY enumsortorder`).Scan(&vals).Error; err!=nil{log.Fatal(err)}; for _,v:=range vals{fmt.Println(v)} }
