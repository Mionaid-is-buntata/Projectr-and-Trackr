package repository

import (
	"database/sql"
	"time"

	"github.com/yourname/projctr/internal/models"
)

// DescriptionStore persists and queries job descriptions.
type DescriptionStore struct {
	db *sql.DB
}

// NewDescriptionStore creates a store for the descriptions table.
func NewDescriptionStore(db *sql.DB) *DescriptionStore {
	return &DescriptionStore{db: db}
}

// HasContentHash returns true if a description with this content hash exists.
func (s *DescriptionStore) HasContentHash(hash string) (bool, error) {
	var exists int
	err := s.db.QueryRow(
		`SELECT 1 FROM descriptions WHERE content_hash = ? LIMIT 1`,
		hash,
	).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// Insert inserts a description. Returns the new ID or error.
func (s *DescriptionStore) Insert(d *models.Description) (int64, error) {
	res, err := s.db.Exec(`
		INSERT INTO descriptions (
			huntr_id, role_title, sector, salary_min, salary_max,
			location, source_board, huntr_score, raw_text,
			date_scraped, date_ingested, content_hash
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		d.HuntrID, d.RoleTitle, d.Sector, d.SalaryMin, d.SalaryMax,
		d.Location, d.SourceBoard, d.HuntrScore, d.RawText,
		timeToNull(d.DateScraped), d.DateIngested, d.ContentHash,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func timeToNull(t time.Time) interface{} {
	if t.IsZero() {
		return nil
	}
	return t
}

// Count returns the total number of descriptions.
func (s *DescriptionStore) Count() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM descriptions`).Scan(&n)
	return n, err
}

// ListUnextracted returns descriptions that have no pain points extracted yet.
func (s *DescriptionStore) ListUnextracted() ([]*models.Description, error) {
	rows, err := s.db.Query(`
		SELECT d.id, d.huntr_id, d.role_title, d.sector, d.salary_min, d.salary_max,
			d.location, d.source_board, d.huntr_score, d.raw_text,
			d.date_scraped, d.date_ingested, d.content_hash
		FROM descriptions d
		LEFT JOIN pain_points p ON p.description_id = d.id
		WHERE p.id IS NULL
		ORDER BY d.date_ingested DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*models.Description
	for rows.Next() {
		var d models.Description
		var dateScraped sql.NullTime
		if err := rows.Scan(
			&d.ID, &d.HuntrID, &d.RoleTitle, &d.Sector, &d.SalaryMin, &d.SalaryMax,
			&d.Location, &d.SourceBoard, &d.HuntrScore, &d.RawText,
			&dateScraped, &d.DateIngested, &d.ContentHash,
		); err != nil {
			return nil, err
		}
		if dateScraped.Valid {
			d.DateScraped = dateScraped.Time
		}
		out = append(out, &d)
	}
	return out, rows.Err()
}

// GetByID returns a description by ID.
func (s *DescriptionStore) GetByID(id int64) (*models.Description, error) {
	var d models.Description
	var dateScraped sql.NullTime
	err := s.db.QueryRow(`
		SELECT id, huntr_id, role_title, sector, salary_min, salary_max,
			location, source_board, huntr_score, raw_text,
			date_scraped, date_ingested, content_hash
		FROM descriptions WHERE id = ?`, id,
	).Scan(
		&d.ID, &d.HuntrID, &d.RoleTitle, &d.Sector, &d.SalaryMin, &d.SalaryMax,
		&d.Location, &d.SourceBoard, &d.HuntrScore, &d.RawText,
		&dateScraped, &d.DateIngested, &d.ContentHash,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if dateScraped.Valid {
		d.DateScraped = dateScraped.Time
	}
	return &d, nil
}
