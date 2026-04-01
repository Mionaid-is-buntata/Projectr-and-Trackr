package repository_test

import (
	"testing"
	"time"

	"github.com/campbell/projctr/internal/models"
	"github.com/campbell/projctr/internal/repository"
	"github.com/campbell/projctr/internal/testutil"
)

func TestDescriptionStore_InsertAndGet(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := repository.NewDescriptionStore(db)

	d := &models.Description{
		HuntrID: "test-001", RoleTitle: "Go Engineer", Sector: "FinTech",
		Location: "London", SourceBoard: "LinkedIn", HuntrScore: 150,
		RawText: "Job description text", DateIngested: time.Now(), ContentHash: "abc123",
	}
	id, err := store.Insert(d)
	if err != nil {
		t.Fatal(err)
	}

	got, err := store.GetByID(id)
	if err != nil {
		t.Fatal(err)
	}
	if got.RoleTitle != "Go Engineer" {
		t.Errorf("RoleTitle = %q", got.RoleTitle)
	}
	if got.HuntrID != "test-001" {
		t.Errorf("HuntrID = %q", got.HuntrID)
	}
}

func TestDescriptionStore_HasHuntrID(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := repository.NewDescriptionStore(db)

	has, _ := store.HasHuntrID("missing")
	if has {
		t.Error("should not find missing ID")
	}

	testutil.SeedDescriptionWith(t, db, "exists", "Title", "Sector", "Text")

	has, _ = store.HasHuntrID("exists")
	if !has {
		t.Error("should find existing ID")
	}
}

func TestDescriptionStore_HasContentHash(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := repository.NewDescriptionStore(db)

	has, _ := store.HasContentHash("missing-hash")
	if has {
		t.Error("should not find missing hash")
	}

	testutil.SeedDescriptionWith(t, db, "h1", "Title", "Sector", "Text")

	has, _ = store.HasContentHash("hash-h1")
	if !has {
		t.Error("should find existing hash")
	}
}

func TestDescriptionStore_Count(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := repository.NewDescriptionStore(db)

	n, _ := store.Count()
	if n != 0 {
		t.Errorf("initial count = %d", n)
	}

	testutil.SeedDescription(t, db)
	n, _ = store.Count()
	if n != 1 {
		t.Errorf("count = %d, want 1", n)
	}
}

func TestDescriptionStore_ListUnextracted(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := repository.NewDescriptionStore(db)

	descID := testutil.SeedDescription(t, db)
	descs, err := store.ListUnextracted()
	if err != nil {
		t.Fatal("ListUnextracted error:", err)
	}
	if len(descs) != 1 {
		t.Fatalf("expected 1 unextracted, got %d", len(descs))
	}

	// Add a pain point — description should no longer be unextracted
	testutil.SeedPainPoint(t, db, descID, "challenge", "platform", 0.9)
	descs, _ = store.ListUnextracted()
	if len(descs) != 0 {
		t.Errorf("expected 0 unextracted after adding pain point, got %d", len(descs))
	}
}

func TestDescriptionStore_Clear(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := repository.NewDescriptionStore(db)
	testutil.SeedDescription(t, db)

	store.Clear()
	n, _ := store.Count()
	if n != 0 {
		t.Errorf("count after clear = %d", n)
	}
}
