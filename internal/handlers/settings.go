package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/campbell/projctr/internal/huntr"
	"github.com/campbell/projctr/internal/repository"
)

// SettingsHandler returns the current score band (GET /api/settings).
func SettingsHandler(reader *huntr.JobReader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		min, max := reader.Range()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"score_min": min,
			"score_max": max,
		})
	}
}

// UpdateSettingsHandler updates the score band at runtime and persists it (POST /api/settings).
func UpdateSettingsHandler(reader *huntr.JobReader, store *repository.SettingsStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			ScoreMin float64 `json:"score_min"`
			ScoreMax float64 `json:"score_max"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		if body.ScoreMin < 0 || body.ScoreMax <= body.ScoreMin {
			http.Error(w, "score_min must be >= 0 and score_max must be > score_min", http.StatusBadRequest)
			return
		}
		reader.SetRange(body.ScoreMin, body.ScoreMax)
		if err := store.SetFloat("score_min", body.ScoreMin); err != nil {
			http.Error(w, "failed to persist score_min", http.StatusInternalServerError)
			return
		}
		if err := store.SetFloat("score_max", body.ScoreMax); err != nil {
			http.Error(w, "failed to persist score_max", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"score_min": body.ScoreMin,
			"score_max": body.ScoreMax,
		})
	}
}
