package database

import (
	"database/sql"
)

// Migrate applies the Projctr schema to the given database connection.
// It is idempotent — safe to call on every startup.
func Migrate(db *sql.DB) error {
	schema := []string{
		`CREATE TABLE IF NOT EXISTS descriptions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			huntr_id TEXT NOT NULL,
			role_title TEXT,
			sector TEXT,
			salary_min INTEGER,
			salary_max INTEGER,
			location TEXT,
			source_board TEXT,
			huntr_score REAL,
			raw_text TEXT NOT NULL,
			date_scraped DATETIME,
			date_ingested DATETIME NOT NULL,
			content_hash TEXT NOT NULL,
			UNIQUE(huntr_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_descriptions_content_hash ON descriptions(content_hash)`,
		`CREATE INDEX IF NOT EXISTS idx_descriptions_date_ingested ON descriptions(date_ingested)`,
		`CREATE TABLE IF NOT EXISTS pain_points (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			description_id INTEGER NOT NULL REFERENCES descriptions(id),
			challenge_text TEXT NOT NULL,
			domain TEXT,
			outcome_text TEXT,
			confidence REAL,
			qdrant_point_id TEXT,
			date_extracted DATETIME NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_pain_points_description_id ON pain_points(description_id)`,
		`CREATE TABLE IF NOT EXISTS technologies (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			category TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS pain_point_technologies (
			pain_point_id INTEGER NOT NULL REFERENCES pain_points(id),
			technology_id INTEGER NOT NULL REFERENCES technologies(id),
			PRIMARY KEY (pain_point_id, technology_id)
		)`,
		`CREATE TABLE IF NOT EXISTS clusters (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			summary TEXT NOT NULL,
			frequency INTEGER NOT NULL DEFAULT 0,
			avg_salary REAL,
			recency_score REAL,
			gap_type TEXT,
			gap_score REAL,
			date_clustered DATETIME NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS cluster_members (
			cluster_id INTEGER NOT NULL REFERENCES clusters(id),
			pain_point_id INTEGER NOT NULL REFERENCES pain_points(id),
			PRIMARY KEY (cluster_id, pain_point_id)
		)`,
		`CREATE TABLE IF NOT EXISTS briefs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			cluster_id INTEGER NOT NULL REFERENCES clusters(id),
			title TEXT NOT NULL,
			problem_statement TEXT,
			suggested_approach TEXT,
			technology_stack TEXT,
			project_layout TEXT,
			complexity TEXT,
			impact_score REAL,
			linkedin_angle TEXT,
			is_edited INTEGER NOT NULL DEFAULT 0,
			date_generated DATETIME NOT NULL,
			date_modified DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS projects (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			brief_id INTEGER NOT NULL REFERENCES briefs(id),
			stage TEXT NOT NULL,
			repository_url TEXT,
			linkedin_url TEXT,
			notes TEXT,
			date_created DATETIME NOT NULL,
			date_selected DATETIME,
			date_started DATETIME,
			date_completed DATETIME,
			date_published DATETIME
		)`,
	}
	for _, stmt := range schema {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	// Add columns for existing databases (idempotent — errors ignored)
	_, _ = db.Exec(`ALTER TABLE briefs ADD COLUMN project_layout TEXT`)
	_, _ = db.Exec(`ALTER TABLE briefs ADD COLUMN source_company TEXT NOT NULL DEFAULT ''`)
	_, _ = db.Exec(`ALTER TABLE briefs ADD COLUMN source_role TEXT NOT NULL DEFAULT ''`)

	// Settings table (idempotent)
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS settings (
		key   TEXT PRIMARY KEY,
		value TEXT NOT NULL
	)`); err != nil {
		return err
	}

	// Trackr: add columns to projects table (idempotent — errors ignored)
	_, _ = db.Exec(`ALTER TABLE projects ADD COLUMN github_url TEXT`)
	_, _ = db.Exec(`ALTER TABLE projects ADD COLUMN live_url TEXT`)
	_, _ = db.Exec(`ALTER TABLE projects ADD COLUMN linkedin_draft TEXT`)
	_, _ = db.Exec(`ALTER TABLE projects ADD COLUMN date_parked DATETIME`)

	// Trackr: rename github_url → gitea_url (idempotent — errors ignored if already renamed or column missing)
	_, _ = db.Exec(`ALTER TABLE projects RENAME COLUMN github_url TO gitea_url`)

	// Trackr: migrate old stage values to canonical set
	_, _ = db.Exec(`UPDATE projects SET stage = 'candidate' WHERE stage = 'selected'`)
	_, _ = db.Exec(`UPDATE projects SET stage = 'published' WHERE stage = 'complete'`)

	return nil
}
