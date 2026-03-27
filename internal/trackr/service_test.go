package trackr_test

import (
	"testing"

	"github.com/yourname/projctr/internal/models"
	"github.com/yourname/projctr/internal/repository"
	"github.com/yourname/projctr/internal/testutil"
	"github.com/yourname/projctr/internal/trackr"
)

func setupService(t *testing.T) *trackr.Service {
	t.Helper()
	db := testutil.NewTestDB(t)
	return trackr.NewService(repository.NewProjectStore(db), repository.NewBriefStore(db))
}

func TestCreateManualProject(t *testing.T) {
	svc := setupService(t)
	p, err := svc.CreateManualProject("My Project", "medium", "notes", "https://gitea.local/repo", "https://live.local")
	if err != nil {
		t.Fatal(err)
	}
	if p.Title != "My Project" {
		t.Errorf("title = %q", p.Title)
	}
	if p.Stage != "candidate" {
		t.Errorf("stage = %q", p.Stage)
	}
	if p.ID == 0 {
		t.Error("ID should be set")
	}
}

func TestCreateManualProject_EmptyTitle(t *testing.T) {
	svc := setupService(t)
	_, err := svc.CreateManualProject("", "small", "", "", "")
	if err == nil {
		t.Fatal("expected error for empty title")
	}
}

func TestEnsureProject_Idempotent(t *testing.T) {
	db := testutil.NewTestDB(t)
	briefStore := repository.NewBriefStore(db)
	projectStore := repository.NewProjectStore(db)
	svc := trackr.NewService(projectStore, briefStore)

	brief := testutil.MustSeedBrief(t, db)

	p1, err := svc.EnsureProject(brief)
	if err != nil {
		t.Fatal(err)
	}
	p2, err := svc.EnsureProject(brief)
	if err != nil {
		t.Fatal(err)
	}
	if p1.ID != p2.ID {
		t.Errorf("expected same ID, got %d and %d", p1.ID, p2.ID)
	}
}

func TestTransitionStage_AllValid(t *testing.T) {
	transitions := []struct {
		from, to string
	}{
		{"candidate", "in_progress"},
		{"candidate", "parked"},
		{"candidate", "archived"},
		{"in_progress", "published"},
		{"in_progress", "parked"},
		{"in_progress", "candidate"},
		{"parked", "candidate"},
		{"parked", "archived"},
		{"published", "archived"},
	}

	for _, tt := range transitions {
		t.Run(tt.from+"→"+tt.to, func(t *testing.T) {
			db := testutil.NewTestDB(t)
			projectStore := repository.NewProjectStore(db)
			briefStore := repository.NewBriefStore(db)
			svc := trackr.NewService(projectStore, briefStore)

			p, _ := svc.CreateManualProject("Test", "small", "", "", "")

			// Get to the starting state
			if tt.from != "candidate" {
				// First transition to in_progress if needed
				switch tt.from {
				case "in_progress":
					svc.TransitionStage(p.ID, "in_progress")
				case "parked":
					svc.TransitionStage(p.ID, "parked")
				case "published":
					svc.TransitionStage(p.ID, "in_progress")
					svc.TransitionStage(p.ID, "published")
				}
			}

			result, err := svc.TransitionStage(p.ID, tt.to)
			if err != nil {
				t.Fatalf("transition %s→%s failed: %v", tt.from, tt.to, err)
			}
			if result.Stage != tt.to {
				t.Errorf("stage = %q, want %q", result.Stage, tt.to)
			}
		})
	}
}

func TestTransitionStage_Invalid(t *testing.T) {
	invalid := []struct {
		from, to string
	}{
		{"candidate", "published"},
		{"parked", "in_progress"},
		{"parked", "published"},
		{"published", "candidate"},
		{"published", "in_progress"},
		{"archived", "candidate"},
	}

	for _, tt := range invalid {
		t.Run(tt.from+"→"+tt.to, func(t *testing.T) {
			db := testutil.NewTestDB(t)
			projectStore := repository.NewProjectStore(db)
			briefStore := repository.NewBriefStore(db)
			svc := trackr.NewService(projectStore, briefStore)

			p, _ := svc.CreateManualProject("Test", "small", "", "", "")

			// Get to starting state
			switch tt.from {
			case "in_progress":
				svc.TransitionStage(p.ID, "in_progress")
			case "parked":
				svc.TransitionStage(p.ID, "parked")
			case "published":
				svc.TransitionStage(p.ID, "in_progress")
				svc.TransitionStage(p.ID, "published")
			case "archived":
				svc.TransitionStage(p.ID, "archived")
			}

			_, err := svc.TransitionStage(p.ID, tt.to)
			if err != trackr.ErrInvalidTransition {
				t.Errorf("expected ErrInvalidTransition, got %v", err)
			}
		})
	}
}

func TestTransitionStage_SetsTimestamps(t *testing.T) {
	db := testutil.NewTestDB(t)
	svc := trackr.NewService(repository.NewProjectStore(db), repository.NewBriefStore(db))

	p, _ := svc.CreateManualProject("Test", "small", "", "", "")

	// candidate → in_progress sets date_started
	result, _ := svc.TransitionStage(p.ID, "in_progress")
	if result.DateStarted == nil {
		t.Error("date_started should be set")
	}

	// in_progress → published sets date_published
	result, _ = svc.TransitionStage(p.ID, "published")
	if result.DatePublished == nil {
		t.Error("date_published should be set")
	}
}

func TestTransitionStage_ParkedTimestamp(t *testing.T) {
	db := testutil.NewTestDB(t)
	svc := trackr.NewService(repository.NewProjectStore(db), repository.NewBriefStore(db))

	p, _ := svc.CreateManualProject("Test", "small", "", "", "")
	result, _ := svc.TransitionStage(p.ID, "parked")
	if result.DateParked == nil {
		t.Error("date_parked should be set")
	}
}

func TestUpdateMetadata(t *testing.T) {
	svc := setupService(t)
	p, _ := svc.CreateManualProject("Test", "small", "", "", "")

	err := svc.UpdateMetadata(p.ID, "https://gitea.local/x", "https://live.local/x", "notes", "New Title", "large")
	if err != nil {
		t.Fatal(err)
	}

	got, _ := svc.GetByID(p.ID)
	if got.GiteaURL != "https://gitea.local/x" {
		t.Errorf("gitea_url = %q", got.GiteaURL)
	}
	if got.Title != "New Title" {
		t.Errorf("title = %q", got.Title)
	}
}

func TestSaveDraft(t *testing.T) {
	svc := setupService(t)
	p, _ := svc.CreateManualProject("Test", "small", "", "", "")

	err := svc.SaveDraft(p.ID, "My LinkedIn draft")
	if err != nil {
		t.Fatal(err)
	}

	got, _ := svc.GetByID(p.ID)
	if got.LinkedInDraft != "My LinkedIn draft" {
		t.Errorf("draft = %q", got.LinkedInDraft)
	}
}

func TestEnsureProject_InheritsFromBrief(t *testing.T) {
	db := testutil.NewTestDB(t)
	briefStore := repository.NewBriefStore(db)
	projectStore := repository.NewProjectStore(db)
	svc := trackr.NewService(projectStore, briefStore)

	brief := &models.Brief{
		ClusterID:     1,
		Title:         "Brief Title",
		Complexity:    "large",
		DateGenerated: testutil.MustSeedBrief(t, db).DateGenerated,
	}
	// Use the seeded brief
	brief = testutil.MustSeedBrief(t, db)

	p, err := svc.EnsureProject(brief)
	if err != nil {
		t.Fatal(err)
	}
	if p.BriefID != brief.ID {
		t.Errorf("brief_id = %d, want %d", p.BriefID, brief.ID)
	}
	if p.Stage != "candidate" {
		t.Errorf("stage = %q, want candidate", p.Stage)
	}
}
