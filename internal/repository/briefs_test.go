package repository_test

import (
	"testing"
	"time"

	"github.com/campbell/projctr/internal/models"
	"github.com/campbell/projctr/internal/repository"
	"github.com/campbell/projctr/internal/testutil"
)

func TestBriefStore_InsertAndGetByID(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := repository.NewBriefStore(db)

	now := time.Now()
	b := &models.Brief{
		ClusterID:         1,
		Title:             "Test Brief",
		ProblemStatement:  "Problem",
		SuggestedApproach: "Approach",
		TechnologyStack:   `["Go","Docker"]`,
		Complexity:        "medium",
		LinkedInAngle:     "Angle",
		IsEdited:          false,
		DateGenerated:     now,
	}
	id, err := store.Insert(b)
	if err != nil {
		t.Fatal(err)
	}

	got, err := store.GetByID(id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Test Brief" {
		t.Errorf("title = %q", got.Title)
	}
	if got.TechnologyStack != `["Go","Docker"]` {
		t.Errorf("tech stack = %q", got.TechnologyStack)
	}
	if got.IsEdited {
		t.Error("should not be edited")
	}
}

func TestBriefStore_List(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := repository.NewBriefStore(db)

	now := time.Now()
	store.Insert(&models.Brief{ClusterID: 1, Title: "A", DateGenerated: now.Add(-time.Hour)})
	store.Insert(&models.Brief{ClusterID: 2, Title: "B", DateGenerated: now})

	briefs, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(briefs) != 2 {
		t.Fatalf("expected 2, got %d", len(briefs))
	}
	// Most recent first
	if briefs[0].Title != "B" {
		t.Errorf("first = %q, want B (newest)", briefs[0].Title)
	}
}

func TestBriefStore_UpdateTitle(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := repository.NewBriefStore(db)

	id, _ := store.Insert(&models.Brief{ClusterID: 1, Title: "Old", DateGenerated: time.Now()})
	if err := store.UpdateTitle(id, "New Title"); err != nil {
		t.Fatal(err)
	}

	got, _ := store.GetByID(id)
	if got.Title != "New Title" {
		t.Errorf("title = %q", got.Title)
	}
	if !got.IsEdited {
		t.Error("should be marked as edited")
	}
	if got.DateModified == nil {
		t.Error("date_modified should be set")
	}
}

func TestBriefStore_SetGeneratedTitle(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := repository.NewBriefStore(db)

	id, _ := store.Insert(&models.Brief{ClusterID: 1, Title: "Old", DateGenerated: time.Now()})
	if err := store.SetGeneratedTitle(id, "Brief 1: Generated"); err != nil {
		t.Fatal(err)
	}

	got, _ := store.GetByID(id)
	if got.Title != "Brief 1: Generated" {
		t.Errorf("title = %q", got.Title)
	}
	if got.IsEdited {
		t.Error("should NOT be marked as edited")
	}
}

func TestBriefStore_Clear(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := repository.NewBriefStore(db)

	store.Insert(&models.Brief{ClusterID: 1, Title: "A", DateGenerated: time.Now()})
	store.Clear()

	briefs, _ := store.List()
	if len(briefs) != 0 {
		t.Errorf("expected 0 after clear, got %d", len(briefs))
	}
}
