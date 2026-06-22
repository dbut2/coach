package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/phayes/freeport"
	"github.com/pressly/goose/v3"
)

const (
	pgUser = "postgres"
	pgPass = "postgres"
	pgDB   = "postgres"

	gooseVersionTable = "goose_db_version"
)

// ¯\_(ツ)_/¯
const tablesQuery = `
SELECT format(E'CREATE TABLE %I (\n    %s\n);',
	c.relname,
	string_agg(
		format('%I %s%s%s',
			a.attname,
			pg_catalog.format_type(a.atttypid, a.atttypmod),
			CASE WHEN ad.adbin IS NOT NULL THEN ' DEFAULT ' || pg_get_expr(ad.adbin, ad.adrelid) ELSE '' END,
			CASE WHEN a.attnotnull THEN ' NOT NULL' ELSE '' END),
		E',\n    ' ORDER BY a.attnum))
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
JOIN pg_attribute a ON a.attrelid = c.oid AND a.attnum > 0 AND NOT a.attisdropped
LEFT JOIN pg_attrdef ad ON ad.adrelid = c.oid AND ad.adnum = a.attnum
WHERE n.nspname = 'public' AND c.relkind = 'r' AND c.relname <> $1
GROUP BY c.oid, c.relname
ORDER BY c.relname;`

func main() {
	if err := run(); err != nil {
		log.Fatalf("schemadump: %v", err)
	}
}

func run() error {
	migrationsDir := flag.String("migrations", "../db/migrations", "goose migrations dir")
	outPath := flag.String("out", "../db/schema.sql", `output file ("-" for stdout)`)
	flag.Parse()

	port, err := freeport.GetFreePort()
	if err != nil {
		return err
	}

	runtimeDir, err := os.MkdirTemp("", "coach-schemadump-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(runtimeDir) }()

	cfg := embeddedpostgres.DefaultConfig().
		Version(embeddedpostgres.V18).
		Username(pgUser).
		Password(pgPass).
		Database(pgDB).
		Port(uint32(port)).
		RuntimePath(runtimeDir).
		BinariesPath(runtimeDir).
		DataPath(filepath.Join(runtimeDir, "data")).
		Logger(io.Discard)

	pg := embeddedpostgres.NewDatabase(cfg)
	if err := pg.Start(); err != nil {
		return fmt.Errorf("start postgres: %w", err)
	}
	defer func() {
		if err := pg.Stop(); err != nil {
			log.Printf("warning: stop postgres: %v", err)
		}
	}()

	db, err := sql.Open("pgx", dsn(port))
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if err := applyMigrations(db, *migrationsDir); err != nil {
		return err
	}

	schema, err := dumpSchema(ctx, db)
	if err != nil {
		return err
	}

	if *outPath == "-" {
		_, err := os.Stdout.WriteString(schema)
		return err
	}
	return os.WriteFile(*outPath, []byte(schema), 0o644)
}

func applyMigrations(db *sql.DB, dir string) error {
	goose.SetLogger(goose.NopLogger())
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	if err := goose.Up(db, dir); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}
	return nil
}

func dumpSchema(ctx context.Context, db *sql.DB) (string, error) {
	rows, err := db.QueryContext(ctx, tablesQuery, gooseVersionTable)
	if err != nil {
		return "", err
	}
	defer func() { _ = rows.Close() }()

	var tables []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return "", err
		}
		tables = append(tables, s)
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	return strings.Join(tables, "\n\n") + "\n", nil
}

func dsn(port int) string {
	return fmt.Sprintf("postgres://%s:%s@localhost:%d/%s?sslmode=disable", pgUser, pgPass, port, pgDB)
}
