package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/zo-king/zoking_blog/apps/api/internal/config"
	"github.com/zo-king/zoking_blog/apps/api/internal/database"
	"github.com/zo-king/zoking_blog/apps/api/internal/maintenance"
	"github.com/zo-king/zoking_blog/apps/api/internal/publisher"
)

func main() {
	cfg := config.Load()
	if err := cfg.ValidateRuntime(); err != nil {
		log.Fatalf("invalid runtime configuration: %v", err)
	}

	db, err := database.Connect(context.Background(), cfg)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	if sqlDB, dbErr := db.DB(); dbErr == nil {
		defer sqlDB.Close()
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	worker := publisher.NewWorker(db, cfg, log.Default())
	go maintenance.RunPreviewCleanup(ctx, db, cfg, log.Default())
	if err := worker.Run(ctx); err != nil {
		log.Fatalf("run publish worker: %v", err)
	}
}
