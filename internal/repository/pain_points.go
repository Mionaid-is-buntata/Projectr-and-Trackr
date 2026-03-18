package repository

import (
	"database/sql"
	"time"

	"github.com/yourname/projctr/internal/models"
)

// PainPointStore persists and queries pain points and their technology links.
type PainPointStore struct {
	db *sql.DB
}

// NewPainPointStore creates a store for the pain_points table.
func NewPainPointStore(db *sql.DB) *PainPointStore {
	return &PainPointStore{db: db}
}

// Insert saves a pain point and returns its new ID.
func (s *PainPointStore) Insert(p *models.PainPoint) (int64, error) {
	res, err := s.db.Exec(`
		INSERT INTO pain_points (
			description_id, challenge_text, domain,
			outcome_text, confidence, qdrant_point_id, date_extracted
		) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		p.DescriptionID, p.ChallengeText, p.Domain,
		p.OutcomeText, p.Confidence, nullableString(p.QdrantPointID),
		p.DateExtracted,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// InsertTechnology upserts a technology by name and returns its ID.
func (s *PainPointStore) InsertTechnology(t *models.Technology) (int64, error) {
	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO technologies (name, category) VALUES (?, ?)`,
		t.Name, t.Category,
	)
	if err != nil {
		return 0, err
	}
	var id int64
	err = s.db.QueryRow(`SELECT id FROM technologies WHERE name = ?`, t.Name).Scan(&id)
	return id, err
}

// LinkTechnology links a pain point to a technology.
func (s *PainPointStore) LinkTechnology(painPointID, techID int64) error {
	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO pain_point_technologies (pain_point_id, technology_id) VALUES (?, ?)`,
		painPointID, techID,
	)
	return err
}

// ListAll returns all pain points.
func (s *PainPointStore) ListAll() ([]*models.PainPoint, error) {
	rows, err := s.db.Query(`
		SELECT id, description_id, challenge_text, domain,
			outcome_text, confidence, COALESCE(qdrant_point_id,''), date_extracted
		FROM pain_points ORDER BY date_extracted DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPainPoints(rows)
}

// ListByDescriptionID returns pain points for a given description.
func (s *PainPointStore) ListByDescriptionID(descID int64) ([]*models.PainPoint, error) {
	rows, err := s.db.Query(`
		SELECT id, description_id, challenge_text, domain,
			outcome_text, confidence, COALESCE(qdrant_point_id,''), date_extracted
		FROM pain_points WHERE description_id = ?`, descID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPainPoints(rows)
}

// ListUnassigned returns pain points not yet assigned to any cluster.
func (s *PainPointStore) ListUnassigned() ([]*models.PainPoint, error) {
	rows, err := s.db.Query(`
		SELECT p.id, p.description_id, p.challenge_text, p.domain,
			p.outcome_text, p.confidence, COALESCE(p.qdrant_point_id,''), p.date_extracted
		FROM pain_points p
		LEFT JOIN cluster_members cm ON cm.pain_point_id = p.id
		WHERE cm.pain_point_id IS NULL
		ORDER BY p.date_extracted DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPainPoints(rows)
}

// Count returns the total number of pain points.
func (s *PainPointStore) Count() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM pain_points`).Scan(&n)
	return n, err
}

// Clear deletes all pain_point_technologies links and pain points.
func (s *PainPointStore) Clear() error {
	if _, err := s.db.Exec(`DELETE FROM pain_point_technologies`); err != nil {
		return err
	}
	_, err := s.db.Exec(`DELETE FROM pain_points`)
	return err
}

func scanPainPoints(rows *sql.Rows) ([]*models.PainPoint, error) {
	var out []*models.PainPoint
	for rows.Next() {
		var p models.PainPoint
		var dateExtracted time.Time
		if err := rows.Scan(
			&p.ID, &p.DescriptionID, &p.ChallengeText, &p.Domain,
			&p.OutcomeText, &p.Confidence, &p.QdrantPointID, &dateExtracted,
		); err != nil {
			return nil, err
		}
		p.DateExtracted = dateExtracted
		out = append(out, &p)
	}
	return out, rows.Err()
}

func nullableString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
