package database

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// Open connects to the SQLite database at the given path.
// Enables WAL mode for better concurrent read performance.
func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path+"?_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	return db, nil
}
