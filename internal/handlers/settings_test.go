package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/yourname/projctr/internal/handlers"
	"github.com/yourname/projctr/internal/huntr"
	"github.com/yourname/projctr/internal/repository"
	"github.com/yourname/projctr/internal/testutil"
)

func setupSettingsRouter(t *testing.T) (chi.Router, *huntr.JobReader) {
	t.Helper()
	db := testutil.NewTestDB(t)
	reader := huntr.NewJobReader(t.TempDir(), 0, 300)
	store := repository.NewSettingsStore(db)
	r := chi.NewRouter()
	handlers.Register(r, &handlers.Dependencies{
		JobReader:     reader,
		SettingsStore: store,
	})
	return r, reader
}

func TestSettingsHandler_Get(t *testing.T) {
	r, _ := setupSettingsRouter(t)

	req := httptest.NewRequest("GET", "/api/settings", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var body map[string]float64
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["score_min"] != 0 || body["score_max"] != 300 {
		t.Errorf("got min=%f max=%f, want 0/300", body["score_min"], body["score_max"])
	}
}

func TestSettingsHandler_Update(t *testing.T) {
	r, reader := setupSettingsRouter(t)

	payload := `{"score_min": 50, "score_max": 250}`
	req := httptest.NewRequest("POST", "/api/settings", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body: %s", w.Code, w.Body.String())
	}

	min, max := reader.Range()
	if min != 50 || max != 250 {
		t.Errorf("reader range = (%f, %f), want (50, 250)", min, max)
	}
}

func TestSettingsHandler_Update_InvalidRange(t *testing.T) {
	r, _ := setupSettingsRouter(t)

	tests := []struct {
		name    string
		payload string
	}{
		{"min >= max", `{"score_min": 300, "score_max": 100}`},
		{"negative min", `{"score_min": -1, "score_max": 100}`},
		{"equal", `{"score_min": 100, "score_max": 100}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/settings", bytes.NewBufferString(tt.payload))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", w.Code)
			}
		})
	}
}

func TestSettingsHandler_Update_InvalidJSON(t *testing.T) {
	r, _ := setupSettingsRouter(t)

	req := httptest.NewRequest("POST", "/api/settings", bytes.NewBufferString("not json"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}
