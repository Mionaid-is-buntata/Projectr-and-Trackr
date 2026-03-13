package repository

import (
	"database/sql"
	"time"

	"github.com/yourname/projctr/internal/models"
)

// BriefStore persists and queries briefs.
type BriefStore struct {
	db *sql.DB
}

// NewBriefStore creates a store for the briefs table.
func NewBriefStore(db *sql.DB) *BriefStore {
	return &BriefStore{db: db}
}

// Insert inserts a brief and returns the new ID.
func (s *BriefStore) Insert(b *models.Brief) (int64, error) {
	res, err := s.db.Exec(`
		INSERT INTO briefs (
			cluster_id, source_company, source_role,
			title, problem_statement, suggested_approach,
			technology_stack, project_layout, complexity, impact_score,
			linkedin_angle, is_edited, date_generated, date_modified
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		b.ClusterID, b.SourceCompany, b.SourceRole,
		b.Title, b.ProblemStatement, b.SuggestedApproach,
		b.TechnologyStack, b.ProjectLayout, b.Complexity, b.ImpactScore,
		b.LinkedInAngle, boolToInt(b.IsEdited), b.DateGenerated, timeToNullPtr(b.DateModified),
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// List returns all briefs ordered by date_generated desc.
func (s *BriefStore) List() ([]*models.Brief, error) {
	rows, err := s.db.Query(`
		SELECT id, cluster_id, source_company, source_role,
			title, problem_statement, suggested_approach,
			technology_stack, project_layout, complexity, impact_score,
			linkedin_angle, is_edited, date_generated, date_modified
		FROM briefs ORDER BY date_generated DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*models.Brief, 0)
	for rows.Next() {
		var b models.Brief
		var edited int
		var dateMod sql.NullTime
		if err := rows.Scan(
			&b.ID, &b.ClusterID, &b.SourceCompany, &b.SourceRole,
			&b.Title, &b.ProblemStatement, &b.SuggestedApproach,
			&b.TechnologyStack, &b.ProjectLayout, &b.Complexity, &b.ImpactScore,
			&b.LinkedInAngle, &edited, &b.DateGenerated, &dateMod,
		); err != nil {
			return nil, err
		}
		b.IsEdited = edited != 0
		if dateMod.Valid {
			b.DateModified = &dateMod.Time
		}
		out = append(out, &b)
	}
	return out, rows.Err()
}

// GetByID returns a brief by ID.
func (s *BriefStore) GetByID(id int64) (*models.Brief, error) {
	var b models.Brief
	var edited int
	var dateMod sql.NullTime
	err := s.db.QueryRow(`
		SELECT id, cluster_id, source_company, source_role,
			title, problem_statement, suggested_approach,
			technology_stack, project_layout, complexity, impact_score,
			linkedin_angle, is_edited, date_generated, date_modified
		FROM briefs WHERE id = ?`, id,
	).Scan(
		&b.ID, &b.ClusterID, &b.SourceCompany, &b.SourceRole,
		&b.Title, &b.ProblemStatement, &b.SuggestedApproach,
		&b.TechnologyStack, &b.ProjectLayout, &b.Complexity, &b.ImpactScore,
		&b.LinkedInAngle, &edited, &b.DateGenerated, &dateMod,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	b.IsEdited = edited != 0
	if dateMod.Valid {
		b.DateModified = &dateMod.Time
	}
	return &b, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func timeToNullPtr(t *time.Time) interface{} {
	if t == nil {
		return nil
	}
	return *t
}
