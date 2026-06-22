package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"naomi.run/service"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	dsn := os.Getenv("DATABASE_DSN")
	if dsn == "" {
		return errors.New("DATABASE_DSN is required")
	}

	if err := migrate(dsn); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer pool.Close()

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		return fmt.Errorf("ping: %w", err)
	}

	return service.New(pool).Run(ctx)
}

func migrate(dsn string) error {
	dir := os.Getenv("MIGRATIONS_DIR")
	if dir == "" {
		dir = "/migrations"
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	return goose.Up(db, dir)
}
