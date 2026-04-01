package trackr

import (
	"errors"
	"strings"
	"time"

	"github.com/campbell/projctr/internal/models"
	"github.com/campbell/projctr/internal/repository"
)

// ErrInvalidTransition is returned when a stage transition is not allowed.
var ErrInvalidTransition = errors.New("invalid stage transition")

// validTransitions defines the allowed stage transitions.
var validTransitions = map[string][]string{
	"candidate":   {"in_progress", "parked", "archived"},
	"in_progress": {"published", "parked", "candidate"},
	"parked":      {"candidate", "archived"},
	"published":   {"archived"},
	"archived":    {},
}

// Service provides business logic for project tracking.
type Service struct {
	store      *repository.ProjectStore
	briefStore *repository.BriefStore
}

// NewService creates a Trackr service.
func NewService(store *repository.ProjectStore, briefStore *repository.BriefStore) *Service {
	return &Service{store: store, briefStore: briefStore}
}

// CreateManualProject creates a project not linked to any brief.
func (s *Service) CreateManualProject(title, complexity, notes, giteaURL, liveURL string) (*models.Project, error) {
	if strings.TrimSpace(title) == "" {
		return nil, errors.New("title is required")
	}
	now := time.Now()
	p := &models.Project{
		BriefID:     0,
		Stage:       "candidate",
		Title:       title,
		Complexity:  complexity,
		Notes:       notes,
		GiteaURL:    giteaURL,
		LiveURL:     liveURL,
		DateCreated: now,
	}
	id, err := s.store.Insert(p)
	if err != nil {
		return nil, err
	}
	p.ID = id
	return p, nil
}

// EnsureProject returns the existing project for a brief, or creates a new candidate.
func (s *Service) EnsureProject(brief *models.Brief) (*models.Project, error) {
	existing, err := s.store.GetByBriefID(brief.ID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}
	now := time.Now()
	p := &models.Project{
		BriefID:     brief.ID,
		Stage:       "candidate",
		DateCreated: now,
	}
	id, err := s.store.Insert(p)
	if err != nil {
		return nil, err
	}
	p.ID = id
	return p, nil
}

// TransitionStage validates and applies a stage transition.
func (s *Service) TransitionStage(projectID int64, toStage string) (*models.Project, error) {
	p, err := s.store.GetByID(projectID)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, errors.New("project not found")
	}

	allowed := validTransitions[p.Stage]
	valid := false
	for _, s := range allowed {
		if s == toStage {
			valid = true
			break
		}
	}
	if !valid {
		return nil, ErrInvalidTransition
	}

	now := time.Now()
	timestamps := make(map[string]*time.Time)

	switch toStage {
	case "in_progress":
		timestamps["date_started"] = &now
	case "published":
		timestamps["date_published"] = &now
	case "parked":
		timestamps["date_parked"] = &now
	}

	if err := s.store.UpdateStage(projectID, toStage, timestamps); err != nil {
		return nil, err
	}

	return s.store.GetByID(projectID)
}

// ListAll returns all projects with brief info joined.
func (s *Service) ListAll() ([]*models.ProjectWithBrief, error) {
	return s.store.List()
}

// GetForBrief returns the project associated with a brief.
func (s *Service) GetForBrief(briefID int64) (*models.Project, error) {
	return s.store.GetByBriefID(briefID)
}

// GetByID returns a project by ID.
func (s *Service) GetByID(projectID int64) (*models.Project, error) {
	return s.store.GetByID(projectID)
}

// UpdateMetadata updates a project's mutable fields.
func (s *Service) UpdateMetadata(projectID int64, giteaURL, liveURL, notes, title, complexity string) error {
	p, err := s.store.GetByID(projectID)
	if err != nil {
		return err
	}
	if p == nil {
		return errors.New("project not found")
	}
	p.GiteaURL = giteaURL
	p.LiveURL = liveURL
	p.Notes = notes
	p.Title = title
	p.Complexity = complexity
	return s.store.Update(p)
}

// SaveDraft persists a LinkedIn post draft for a project.
func (s *Service) SaveDraft(projectID int64, draft string) error {
	p, err := s.store.GetByID(projectID)
	if err != nil {
		return err
	}
	if p == nil {
		return errors.New("project not found")
	}
	p.LinkedInDraft = draft
	return s.store.Update(p)
}
