package handlers

import (
	"encoding/json"
	"net/http"
)

// ClearIdeasHandler deletes all pipeline data (projects, briefs, clusters, pain points,
// descriptions) so a fresh Huntr ingest can repopulate everything.
// POST /api/admin/clear-ideas
func ClearIdeasHandler(deps *Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Delete in reverse-dependency order.
		if err := deps.TrackrDeps.ProjectStore.Clear(); err != nil {
			http.Error(w, "clear projects: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if err := deps.BriefsDeps.BriefStore.Clear(); err != nil {
			http.Error(w, "clear briefs: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if err := deps.ClusterStore.Clear(); err != nil {
			http.Error(w, "clear clusters: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if err := deps.PainPointStore.Clear(); err != nil {
			http.Error(w, "clear pain points: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if err := deps.DescStore.Clear(); err != nil {
			http.Error(w, "clear descriptions: "+err.Error(), http.StatusInternalServerError)
			return
		}
		// Reset last ingest result so dashboard stats read zero.
		LastIngestResult = nil

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

// ClearTrackrHandler deletes all Trackr project records, leaving briefs and pipeline
// data intact. Useful for resetting stage-tracking without re-ingesting from Huntr.
// POST /api/admin/clear-trackr
func ClearTrackrHandler(deps *Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := deps.TrackrDeps.ProjectStore.Clear(); err != nil {
			http.Error(w, "clear projects: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}
