package migrations

import (
	"database/sql"
	"embed"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

//go:embed *.sql
var Files embed.FS

func Up(db *sql.DB) error {
	if err := goose.Up(db, "."); err != nil {
		return fmt.Errorf("failed to apply migrations: %v", err)
	}

	return nil
}

func Reset(db *sql.DB) error {
	if err := goose.Reset(db, "."); err != nil {
		return fmt.Errorf("failed to reset migrations: %v", err)
	}

	return nil
}
