package handlers_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/yourname/projctr/internal/handlers"
	"github.com/yourname/projctr/internal/linkedin"
	"github.com/yourname/projctr/internal/repository"
	"github.com/yourname/projctr/internal/testutil"
	"github.com/yourname/projctr/internal/trackr"
)

func setupTrackrRouter(t *testing.T, gen *linkedin.Generator) (chi.Router, *trackr.Service, *repository.BriefStore, *repository.ProjectStore) {
	t.Helper()
	db := testutil.NewTestDB(t)
	briefStore := repository.NewBriefStore(db)
	projectStore := repository.NewProjectStore(db)
	svc := trackr.NewService(projectStore, briefStore)

	deps := &handlers.TrackrDeps{
		Service:      svc,
		BriefStore:   briefStore,
		ProjectStore: projectStore,
		Generator:    gen,
	}
	r := chi.NewRouter()
	handlers.RegisterTrackr(r, deps)
	return r, svc, briefStore, projectStore
}

func TestListProjects(t *testing.T) {
	r, svc, _, _ := setupTrackrRouter(t, nil)
	svc.CreateManualProject("Test Project", "small", "", "", "")

	req := httptest.NewRequest("GET", "/api/trackr/projects", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var projects []map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &projects)
	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projects))
	}
}

func TestTransitionHandler_Valid(t *testing.T) {
	r, svc, _, _ := setupTrackrRouter(t, nil)
	p, _ := svc.CreateManualProject("Test", "small", "", "", "")

	payload := `{"to":"in_progress"}`
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/trackr/projects/%d/transition", p.ID), bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body: %s", w.Code, w.Body.String())
	}
}

func TestTransitionHandler_Invalid(t *testing.T) {
	r, svc, _, _ := setupTrackrRouter(t, nil)
	p, _ := svc.CreateManualProject("Test", "small", "", "", "")

	payload := `{"to":"published"}`
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/trackr/projects/%d/transition", p.ID), bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", w.Code)
	}
}

func TestTransitionHandler_NotFound(t *testing.T) {
	r, _, _, _ := setupTrackrRouter(t, nil)

	payload := `{"to":"in_progress"}`
	req := httptest.NewRequest("POST", "/api/trackr/projects/9999/transition", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code == http.StatusOK {
		t.Fatal("expected error for non-existent project")
	}
}

func TestGeneratePostHandler_NoLLM(t *testing.T) {
	r, svc, _, _ := setupTrackrRouter(t, nil)
	p, _ := svc.CreateManualProject("Test", "small", "", "", "")

	req := httptest.NewRequest("POST", fmt.Sprintf("/api/trackr/projects/%d/generate-post", p.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 (no LLM)", w.Code)
	}
}

func TestGeneratePostHandler_WithLLM(t *testing.T) {
	srv := testutil.MockOllamaServer(t, "Generated LinkedIn post content")
	gen := linkedin.NewGenerator(srv.URL, "test-model")
	r, svc, _, _ := setupTrackrRouter(t, gen)
	p, _ := svc.CreateManualProject("Test", "small", "Some notes", "", "")

	req := httptest.NewRequest("POST", fmt.Sprintf("/api/trackr/projects/%d/generate-post", p.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body: %s", w.Code, w.Body.String())
	}
	var body map[string]string
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["draft"] != "Generated LinkedIn post content" {
		t.Errorf("draft = %q", body["draft"])
	}
}

func TestUpdateProjectHandler(t *testing.T) {
	r, svc, _, _ := setupTrackrRouter(t, nil)
	p, _ := svc.CreateManualProject("Test", "small", "", "", "")

	payload := `{"gitea_url":"https://gitea.local/test","live_url":"https://live.local","notes":"updated notes","title":"New Title","complexity":"large"}`
	req := httptest.NewRequest("PUT", fmt.Sprintf("/api/trackr/projects/%d", p.ID), bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body: %s", w.Code, w.Body.String())
	}

	// Verify the update persisted
	updated, _ := svc.GetByID(p.ID)
	if updated.GiteaURL != "https://gitea.local/test" {
		t.Errorf("gitea_url = %q", updated.GiteaURL)
	}
	if updated.Title != "New Title" {
		t.Errorf("title = %q", updated.Title)
	}
}

func TestGetProjectHandler(t *testing.T) {
	r, svc, _, _ := setupTrackrRouter(t, nil)
	p, _ := svc.CreateManualProject("Test", "small", "", "", "")

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/trackr/projects/%d", p.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestGetProjectHandler_NotFound(t *testing.T) {
	r, _, _, _ := setupTrackrRouter(t, nil)

	req := httptest.NewRequest("GET", "/api/trackr/projects/9999", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}
