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
	err := goose.SetDialect(string(databaseType))
	if err != nil {
		return fmt.Errorf(`goose.SetDialect(string(databaseType)). %w`, err)
	}

	gooseErr := goose.Up(db, "migrations")
	if gooseErr != nil {
		return fmt.Errorf(`goose.Up(db, "migrations"). %w`, err)
	}

	return nil
}
