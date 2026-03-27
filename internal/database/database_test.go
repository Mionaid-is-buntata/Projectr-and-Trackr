package database_test

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/yourname/projctr/internal/database"
)

func TestMigrate_Idempotent(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := database.Migrate(db); err != nil {
		t.Fatal("first migration:", err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatal("second migration (idempotent):", err)
	}
}

func TestMigrate_AllTablesExist(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}

	tables := []string{
		"descriptions", "pain_points", "technologies",
		"pain_point_technologies", "clusters", "cluster_members",
		"briefs", "projects", "settings",
	}
	for _, table := range tables {
		var name string
		err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&name)
		if err != nil {
			t.Errorf("table %q missing: %v", table, err)
		}
	}
}
