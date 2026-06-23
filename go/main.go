package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

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

	ctx := context.Background()

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer func() { _ = db.Close() }()

	if err := migrate(db); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		return fmt.Errorf("ping: %w", err)
	}

	return service.New(db).Run(ctx)
}

func migrate(db *sql.DB) error {
	dir := os.Getenv("MIGRATIONS_DIR")
	if dir == "" {
		dir = "/migrations"
	}

	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	return goose.Up(db, dir)
}
