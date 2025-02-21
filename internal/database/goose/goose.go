package goose

import (
	"database/sql"
	"embed"
	"fmt"

	"github.com/pressly/goose/v3"
)

type DatabaseType string

const POSTGRES DatabaseType = "postgres"

//go:embed migrations/*.sql
var embedMigrations embed.FS //

func RunMigration(db *sql.DB, databaseType DatabaseType) error {

	goose.SetBaseFS(embedMigrations)
	if err := goose.SetDialect(string(databaseType)); err != nil {
		return fmt.Errorf(`goose.SetDialect(string(databaseType)). %w`, err)
	}

	if err := goose.Up(db, "migrations"); err != nil {
		return fmt.Errorf(`goose.Up(db, "migrations"). %w`, err)
	}

	return nil
}
