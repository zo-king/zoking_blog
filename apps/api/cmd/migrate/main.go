package main

import (
	"database/sql"
	"log"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/zo-king/zoking_blog/apps/api/internal/config"
)

func main() {
	cfg := config.Load()
	command, args := migrationCommand(os.Args[1:])

	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()

	if err := goose.SetDialect("postgres"); err != nil {
		log.Fatalf("set goose dialect: %v", err)
	}

	if err := goose.Run(command, db, cfg.MigrationsDir, args...); err != nil {
		log.Fatalf("goose %s: %v", command, err)
	}
}

func migrationCommand(args []string) (string, []string) {
	if len(args) == 0 {
		return "up", nil
	}
	if len(args) == 1 {
		return args[0], nil
	}
	return args[0], args[1:]
}
