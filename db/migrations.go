package db

import (
	"embed"
	"io/fs"
)

//go:embed migrations/*.sql
var migrations embed.FS

var Migrations = func() fs.FS {
	m, err := fs.Sub(migrations, "migrations")
	if err != nil {
		panic(err.Error())
	}
	return m
}()
