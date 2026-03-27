package repository_test

import (
	"testing"

	"github.com/yourname/projctr/internal/repository"
	"github.com/yourname/projctr/internal/testutil"
)

func TestSettingsStore_GetFloat_Default(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := repository.NewSettingsStore(db)

	got := store.GetFloat("missing_key", 42.5)
	if got != 42.5 {
		t.Errorf("got %f, want 42.5", got)
	}
}

func TestSettingsStore_SetAndGet(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := repository.NewSettingsStore(db)

	if err := store.SetFloat("score_min", 100.0); err != nil {
		t.Fatal(err)
	}
	got := store.GetFloat("score_min", 0)
	if got != 100.0 {
		t.Errorf("got %f, want 100.0", got)
	}
}

func TestSettingsStore_Upsert(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := repository.NewSettingsStore(db)

	store.SetFloat("key", 1.0)
	store.SetFloat("key", 2.0)

	got := store.GetFloat("key", 0)
	if got != 2.0 {
		t.Errorf("got %f, want 2.0 after upsert", got)
	}
}
