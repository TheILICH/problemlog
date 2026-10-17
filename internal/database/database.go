// db pool and migrations
package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib" // registers "pgx" driver, only goose uses it
	"github.com/pressly/goose/v3"

	"problemlog/migrations"
)

const (
	connectAttempts = 30
	connectDelay    = 2 * time.Second
)

func Connect(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	var lastErr error
	// compose already waits for healthy db, this retry is just a backup
	for attempt := 1; attempt <= connectAttempts; attempt++ {
		pool, err := pgxpool.New(ctx, dsn)
		if err == nil {
			if err = pool.Ping(ctx); err == nil {
				return pool, nil
			}
			pool.Close()
		}
		lastErr = err
		log.Printf("database not ready (attempt %d/%d): %v", attempt, connectAttempts, err)

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(connectDelay):
		}
	}
	return nil, fmt.Errorf("database unreachable after %d attempts: %w", connectAttempts, lastErr)
}

func Migrate(dsn string) error {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open database for migrations: %w", err)
	}
	defer db.Close()

	// sql files are embedded in the binary
	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("goose dialect: %w", err)
	}
	// goose saves applied versions in goose_db_version (inside the volume)
	// so on restart it skips them and seed is not inserted twice
	if err := goose.Up(db, "."); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	return nil
}
