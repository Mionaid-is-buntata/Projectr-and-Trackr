package repository

import (
	"database/sql"
	"time"

	"github.com/yourname/projctr/internal/models"
)

// ProjectStore persists and queries projects.
type ProjectStore struct {
	db *sql.DB
}

// NewProjectStore creates a store for the projects table.
func NewProjectStore(db *sql.DB) *ProjectStore {
	return &ProjectStore{db: db}
}

// Insert inserts a project and returns the new ID.
func (s *ProjectStore) Insert(p *models.Project) (int64, error) {
	res, err := s.db.Exec(`
		INSERT INTO projects (
			brief_id, stage, repository_url, linkedin_url,
			gitea_url, live_url, linkedin_draft, notes,
			date_created, date_selected, date_started,
			date_completed, date_published, date_parked
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.BriefID, p.Stage, p.RepositoryURL, p.LinkedInURL,
		p.GiteaURL, p.LiveURL, p.LinkedInDraft, p.Notes,
		p.DateCreated, timeToNullPtr(p.DateSelected), timeToNullPtr(p.DateStarted),
		timeToNullPtr(p.DateCompleted), timeToNullPtr(p.DatePublished), timeToNullPtr(p.DateParked),
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// GetByID returns a project by ID.
func (s *ProjectStore) GetByID(id int64) (*models.Project, error) {
	var p models.Project
	var dateSelected, dateStarted, dateCompleted, datePublished, dateParked sql.NullTime
	var giteaURL, liveURL, linkedInDraft sql.NullString
	err := s.db.QueryRow(`
		SELECT id, brief_id, stage, repository_url, linkedin_url,
			gitea_url, live_url, linkedin_draft, notes,
			date_created, date_selected, date_started,
			date_completed, date_published, date_parked
		FROM projects WHERE id = ?`, id,
	).Scan(
		&p.ID, &p.BriefID, &p.Stage, &p.RepositoryURL, &p.LinkedInURL,
		&giteaURL, &liveURL, &linkedInDraft, &p.Notes,
		&p.DateCreated, &dateSelected, &dateStarted,
		&dateCompleted, &datePublished, &dateParked,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	scanNullTime(&p.DateSelected, dateSelected)
	scanNullTime(&p.DateStarted, dateStarted)
	scanNullTime(&p.DateCompleted, dateCompleted)
	scanNullTime(&p.DatePublished, datePublished)
	scanNullTime(&p.DateParked, dateParked)
	scanNullString(&p.GiteaURL, giteaURL)
	scanNullString(&p.LiveURL, liveURL)
	scanNullString(&p.LinkedInDraft, linkedInDraft)
	return &p, nil
}

// GetByBriefID returns a project by its brief_id, or nil if none exists.
func (s *ProjectStore) GetByBriefID(briefID int64) (*models.Project, error) {
	var p models.Project
	var dateSelected, dateStarted, dateCompleted, datePublished, dateParked sql.NullTime
	var giteaURL, liveURL, linkedInDraft sql.NullString
	err := s.db.QueryRow(`
		SELECT id, brief_id, stage, repository_url, linkedin_url,
			gitea_url, live_url, linkedin_draft, notes,
			date_created, date_selected, date_started,
			date_completed, date_published, date_parked
		FROM projects WHERE brief_id = ?`, briefID,
	).Scan(
		&p.ID, &p.BriefID, &p.Stage, &p.RepositoryURL, &p.LinkedInURL,
		&giteaURL, &liveURL, &linkedInDraft, &p.Notes,
		&p.DateCreated, &dateSelected, &dateStarted,
		&dateCompleted, &datePublished, &dateParked,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	scanNullTime(&p.DateSelected, dateSelected)
	scanNullTime(&p.DateStarted, dateStarted)
	scanNullTime(&p.DateCompleted, dateCompleted)
	scanNullTime(&p.DatePublished, datePublished)
	scanNullTime(&p.DateParked, dateParked)
	scanNullString(&p.GiteaURL, giteaURL)
	scanNullString(&p.LiveURL, liveURL)
	scanNullString(&p.LinkedInDraft, linkedInDraft)
	return &p, nil
}

// List returns all projects joined with brief title and complexity, ordered by date_created desc.
func (s *ProjectStore) List() ([]*models.ProjectWithBrief, error) {
	rows, err := s.db.Query(`
		SELECT p.id, p.brief_id, p.stage, p.repository_url, p.linkedin_url,
			p.gitea_url, p.live_url, p.linkedin_draft, p.notes,
			p.date_created, p.date_selected, p.date_started,
			p.date_completed, p.date_published, p.date_parked,
			b.title AS brief_title, b.complexity AS brief_complexity
		FROM projects p
		JOIN briefs b ON p.brief_id = b.id
		ORDER BY p.date_created DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*models.ProjectWithBrief, 0)
	for rows.Next() {
		var pw models.ProjectWithBrief
		var dateSelected, dateStarted, dateCompleted, datePublished, dateParked sql.NullTime
		var giteaURL, liveURL, linkedInDraft sql.NullString
		var briefComplexity sql.NullString
		if err := rows.Scan(
			&pw.ID, &pw.BriefID, &pw.Stage, &pw.RepositoryURL, &pw.LinkedInURL,
			&giteaURL, &liveURL, &linkedInDraft, &pw.Notes,
			&pw.DateCreated, &dateSelected, &dateStarted,
			&dateCompleted, &datePublished, &dateParked,
			&pw.BriefTitle, &briefComplexity,
		); err != nil {
			return nil, err
		}
		scanNullTime(&pw.DateSelected, dateSelected)
		scanNullTime(&pw.DateStarted, dateStarted)
		scanNullTime(&pw.DateCompleted, dateCompleted)
		scanNullTime(&pw.DatePublished, datePublished)
		scanNullTime(&pw.DateParked, dateParked)
		scanNullString(&pw.GiteaURL, giteaURL)
		scanNullString(&pw.LiveURL, liveURL)
		scanNullString(&pw.LinkedInDraft, linkedInDraft)
		if briefComplexity.Valid {
			pw.BriefComplexity = briefComplexity.String
		}
		out = append(out, &pw)
	}
	return out, rows.Err()
}

// UpdateStage sets the stage and corresponding timestamp fields.
func (s *ProjectStore) UpdateStage(id int64, stage string, timestamps map[string]*time.Time) error {
	query := `UPDATE projects SET stage = ?`
	args := []interface{}{stage}
	for col, t := range timestamps {
		query += `, ` + col + ` = ?`
		if t != nil {
			args = append(args, *t)
		} else {
			args = append(args, nil)
		}
	}
	query += ` WHERE id = ?`
	args = append(args, id)
	_, err := s.db.Exec(query, args...)
	return err
}

// Update persists mutable fields on a project.
func (s *ProjectStore) Update(p *models.Project) error {
	_, err := s.db.Exec(`
		UPDATE projects SET
			gitea_url = ?, live_url = ?, linkedin_draft = ?, notes = ?
		WHERE id = ?`,
		p.GiteaURL, p.LiveURL, p.LinkedInDraft, p.Notes, p.ID,
	)
	return err
}

func scanNullTime(dst **time.Time, src sql.NullTime) {
	if src.Valid {
		*dst = &src.Time
	}
}

func scanNullString(dst *string, src sql.NullString) {
	if src.Valid {
		*dst = src.String
	}
}
