package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/lib/pq"
)

var (
	DB  *sql.DB
	Ctx context.Context
)

func ConnectPg() {
	var err error
	dsn := fmt.Sprintf("host=database port=5432 user=%s password=%s dbname=%s sslmode=disable",
		os.Getenv("POSTGRES_USER"), os.Getenv("POSTGRES_PASSWORD"), os.Getenv("POSTGRES_DB"))
	DB, err = sql.Open("postgres", dsn)
	if err != nil {
		log.Panicln(err)
	}
	err = DB.Ping()
	if err != nil {
		log.Panicln(err)
	}
	log.Println("Postgres connection successful")
}
