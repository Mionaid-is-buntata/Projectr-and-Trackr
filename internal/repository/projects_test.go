package repository_test

import (
	"testing"
	"time"

	"github.com/yourname/projctr/internal/models"
	"github.com/yourname/projctr/internal/repository"
	"github.com/yourname/projctr/internal/testutil"
)

func TestProjectStore_InsertAndGetByID(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := repository.NewProjectStore(db)

	p := &models.Project{
		BriefID: 0, Stage: "candidate", Title: "My Project",
		Complexity: "medium", DateCreated: time.Now(),
	}
	id, err := store.Insert(p)
	if err != nil {
		t.Fatal(err)
	}

	got, err := store.GetByID(id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "My Project" {
		t.Errorf("title = %q", got.Title)
	}
	if got.Stage != "candidate" {
		t.Errorf("stage = %q", got.Stage)
	}
}

func TestProjectStore_GetByBriefID(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := repository.NewProjectStore(db)

	// Not found
	got, err := store.GetByBriefID(999)
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Error("expected nil for missing brief")
	}

	// Found
	briefID := testutil.SeedBrief(t, db, 0) // cluster_id doesn't matter for this test
	id, _ := store.Insert(&models.Project{BriefID: briefID, Stage: "candidate", DateCreated: time.Now()})

	got, err = store.GetByBriefID(briefID)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.ID != id {
		t.Errorf("expected project %d, got %v", id, got)
	}
}

func TestProjectStore_List(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := repository.NewProjectStore(db)

	store.Insert(&models.Project{Stage: "candidate", Title: "A", DateCreated: time.Now().Add(-time.Hour)})
	store.Insert(&models.Project{Stage: "candidate", Title: "B", DateCreated: time.Now()})

	list, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2, got %d", len(list))
	}
	// Most recent first
	if list[0].Title != "B" {
		t.Errorf("first = %q, want B", list[0].Title)
	}
}

func TestProjectStore_UpdateStage(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := repository.NewProjectStore(db)

	id, _ := store.Insert(&models.Project{Stage: "candidate", DateCreated: time.Now()})
	now := time.Now()
	err := store.UpdateStage(id, "in_progress", map[string]*time.Time{"date_started": &now})
	if err != nil {
		t.Fatal(err)
	}

	got, _ := store.GetByID(id)
	if got.Stage != "in_progress" {
		t.Errorf("stage = %q", got.Stage)
	}
	if got.DateStarted == nil {
		t.Error("date_started should be set")
	}
}

func TestProjectStore_Update(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := repository.NewProjectStore(db)

	id, _ := store.Insert(&models.Project{Stage: "candidate", DateCreated: time.Now()})

	p, _ := store.GetByID(id)
	p.Title = "Updated"
	p.GiteaURL = "https://gitea.local/repo"
	p.LiveURL = "https://live.local"
	p.Notes = "Some notes"
	p.LinkedInDraft = "A draft"
	p.Complexity = "large"
	if err := store.Update(p); err != nil {
		t.Fatal(err)
	}

	got, _ := store.GetByID(id)
	if got.Title != "Updated" {
		t.Errorf("title = %q", got.Title)
	}
	if got.GiteaURL != "https://gitea.local/repo" {
		t.Errorf("gitea_url = %q", got.GiteaURL)
	}
	if got.LinkedInDraft != "A draft" {
		t.Errorf("draft = %q", got.LinkedInDraft)
	}
}

func TestProjectStore_Clear(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := repository.NewProjectStore(db)

	store.Insert(&models.Project{Stage: "candidate", DateCreated: time.Now()})
	store.Clear()

	list, _ := store.List()
	if len(list) != 0 {
		t.Errorf("expected 0 after clear, got %d", len(list))
	}
}
