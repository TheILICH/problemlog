// server entrypoint: config -> db connect -> migrations -> http server
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

	"problemlog/internal/api"
	"problemlog/internal/config"
	"problemlog/internal/database"
	"problemlog/internal/store"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("fatal: %v", err)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// docker sends sigterm on compose down, ctx gets cancelled then
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	log.Println("connected to PostgreSQL")

	if err := database.Migrate(cfg.DatabaseURL); err != nil {
		return err
	}
	log.Println("migrations applied (schema is up to date)")

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           api.New(store.New(pool)),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("listening on :%s", cfg.Port)
		errCh <- srv.ListenAndServe()
	}()

	// wait for server crash or stop signal
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Println("shutting down...")
	}

	// give running requests 10s to finish
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
