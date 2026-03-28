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
			linkedin_angle, is_edited, generation_source, date_generated, date_modified
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		b.ClusterID, b.SourceCompany, b.SourceRole,
		b.Title, b.ProblemStatement, b.SuggestedApproach,
		b.TechnologyStack, b.ProjectLayout, b.Complexity, b.ImpactScore,
		b.LinkedInAngle, boolToInt(b.IsEdited), b.GenerationSource, b.DateGenerated, timeToNullPtr(b.DateModified),
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
			linkedin_angle, is_edited, generation_source, date_generated, date_modified
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
			&b.LinkedInAngle, &edited, &b.GenerationSource, &b.DateGenerated, &dateMod,
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

// Clear deletes all briefs.
func (s *BriefStore) Clear() error {
	_, err := s.db.Exec(`DELETE FROM briefs`)
	return err
}

// UpdateTitle updates a brief's title and sets is_edited and date_modified.
func (s *BriefStore) UpdateTitle(id int64, title string) error {
	_, err := s.db.Exec(`
		UPDATE briefs SET title = ?, is_edited = 1, date_modified = ?
		WHERE id = ?`, title, time.Now(), id,
	)
	return err
}

// SetGeneratedTitle updates title after insert (e.g. "Brief N: …") without marking the brief user-edited.
func (s *BriefStore) SetGeneratedTitle(id int64, title string) error {
	_, err := s.db.Exec(`UPDATE briefs SET title = ? WHERE id = ?`, title, id)
	return err
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
			linkedin_angle, is_edited, generation_source, date_generated, date_modified
		FROM briefs WHERE id = ?`, id,
	).Scan(
		&b.ID, &b.ClusterID, &b.SourceCompany, &b.SourceRole,
		&b.Title, &b.ProblemStatement, &b.SuggestedApproach,
		&b.TechnologyStack, &b.ProjectLayout, &b.Complexity, &b.ImpactScore,
		&b.LinkedInAngle, &edited, &b.GenerationSource, &b.DateGenerated, &dateMod,
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

// UpdateFromFrancis overwrites the LLM-generated fields of a brief and records
// the generation source. is_edited is not set — this is a system update, not a
// user edit. complexity and impactScore are only updated when non-empty / non-nil.
func (s *BriefStore) UpdateFromFrancis(id int64, title, problemStatement, suggestedApproach, linkedInAngle, complexity, source string, impactScore *float64) error {
	now := time.Now()
	if complexity != "" && impactScore != nil {
		_, err := s.db.Exec(`
			UPDATE briefs SET
				title = ?, problem_statement = ?, suggested_approach = ?,
				linkedin_angle = ?, complexity = ?, impact_score = ?,
				generation_source = ?, date_modified = ?
			WHERE id = ?`,
			title, problemStatement, suggestedApproach, linkedInAngle, complexity, *impactScore,
			source, now, id,
		)
		return err
	}
	if complexity != "" {
		_, err := s.db.Exec(`
			UPDATE briefs SET
				title = ?, problem_statement = ?, suggested_approach = ?,
				linkedin_angle = ?, complexity = ?,
				generation_source = ?, date_modified = ?
			WHERE id = ?`,
			title, problemStatement, suggestedApproach, linkedInAngle, complexity,
			source, now, id,
		)
		return err
	}
	if impactScore != nil {
		_, err := s.db.Exec(`
			UPDATE briefs SET
				title = ?, problem_statement = ?, suggested_approach = ?,
				linkedin_angle = ?, impact_score = ?,
				generation_source = ?, date_modified = ?
			WHERE id = ?`,
			title, problemStatement, suggestedApproach, linkedInAngle, *impactScore,
			source, now, id,
		)
		return err
	}
	_, err := s.db.Exec(`
		UPDATE briefs SET
			title = ?, problem_statement = ?, suggested_approach = ?,
			linkedin_angle = ?, generation_source = ?, date_modified = ?
		WHERE id = ?`,
		title, problemStatement, suggestedApproach, linkedInAngle, source, now, id,
	)
	return err
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
