package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/zo-king/zoking_blog/apps/api/internal/config"
	"github.com/zo-king/zoking_blog/apps/api/internal/database"
	"github.com/zo-king/zoking_blog/apps/api/internal/httpapi"
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

	if cfg.PublishWorkerEnabled {
		worker := publisher.NewWorker(db, cfg, log.Default())
		go func() {
			if err := worker.Run(ctx); err != nil {
				log.Printf("publish worker stopped: %v", err)
			}
		}()
		go maintenance.RunPreviewCleanup(ctx, db, cfg, log.Default())
	}

	router := httpapi.NewRouter(db, cfg)
	server := &http.Server{
		Addr:              ":" + cfg.AppPort,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      3 * time.Minute,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	serverErr := make(chan error, 1)
	go func() { serverErr <- server.ListenAndServe() }()
	select {
	case err := <-serverErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("run api: %v", err)
		}
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("shutdown api: %v", err)
		}
	}
}
