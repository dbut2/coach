package main

import (
	"context"
	"database/sql"
	"os"
	"time"

	"github.com/lib/pq"
	"github.com/pressly/goose/v3"

	migrations "dbut.dev/web-template/db"
	"dbut.dev/web-template/go/database"
	"dbut.dev/web-template/go/service"
)

func main() {
	dbConnString := os.Getenv("DATABASE_DSN")

	conn, err := pq.NewConnector(dbConnString)
	if err != nil {
		panic(err.Error())
	}
	db := sql.OpenDB(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = db.PingContext(ctx)
	if err != nil {
		panic(err.Error())
	}

	err = runMigrations(db)
	if err != nil {
		panic(err.Error())
	}

	svc := service.New(database.New(db))
	err = svc.Run(context.Background())
	if err != nil {
		panic(err.Error())
	}
}

func runMigrations(db *sql.DB) error {
	goose.SetBaseFS(migrations.Migrations)
	goose.SetTableName("myapp.goose_db_version")
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	return goose.Up(db, "")
}
