package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/yourname/projctr/internal/huntr"
	"github.com/yourname/projctr/internal/ingestion"
	"github.com/yourname/projctr/internal/pipeline"
	"github.com/yourname/projctr/internal/repository"
)

// Dependencies holds services required by handlers.
type Dependencies struct {
	Pipeline       *ingestion.Pipeline
	DescStore      *repository.DescriptionStore
	PainPointStore *repository.PainPointStore
	ClusterStore   *repository.ClusterStore
	PipelineSvc    *pipeline.Service
	BriefsDeps     *BriefsDeps
	TrackrDeps     *TrackrDeps
	JobReader      *huntr.JobReader
	SettingsStore  *repository.SettingsStore
}

// docHandler serves a markdown file from ./docs/ as preformatted HTML.
func docHandler(filename, title string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := os.ReadFile("docs/" + filename)
		if err != nil {
			http.Error(w, "document not found", http.StatusNotFound)
			return
		}
		escaped := strings.ReplaceAll(string(data), "&", "&amp;")
		escaped = strings.ReplaceAll(escaped, "<", "&lt;")
		escaped = strings.ReplaceAll(escaped, ">", "&gt;")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!DOCTYPE html><html><head><title>%s — Projctr</title>`+
			`<style>body{font-family:system-ui,sans-serif;max-width:860px;margin:2rem auto;padding:0 1rem;line-height:1.6}`+
			`pre{background:#f4f4f4;padding:1rem;border-radius:6px;overflow-x:auto;white-space:pre-wrap}`+
			`a{color:#0066cc} nav{margin-bottom:1.5rem;font-size:0.9rem}</style></head>`+
			`<body><nav><a href="/">← Dashboard</a></nav><pre>%s</pre></body></html>`,
			title, escaped)
	}
}

// Register attaches all Projctr routes to the given chi Router.
func Register(r chi.Router, deps *Dependencies) {
	r.Get("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})
	r.Get("/", DashboardPageHandler(deps))
	r.Get("/docs/how-to", docHandler("how-to.md", "How-To Guide"))
	r.Get("/docs/api", docHandler("api-reference.md", "API Reference"))
	r.Get("/docs/technical", docHandler("technical-deep-dive.md", "Technical Deep-Dive"))

	if deps != nil {
		if deps.Pipeline != nil {
			r.Post("/api/ingest", IngestHandler(deps))
		}
		if deps.DescStore != nil {
			r.Get("/api/ingest/status", IngestStatusHandler(deps.DescStore))
			r.Get("/api/dashboard", DashboardHandler(deps))
		}
		r.Get("/api/pipeline/status", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(pipeline.Current)
		})
	}
	if deps != nil && deps.BriefsDeps != nil {
		RegisterBriefs(r, deps.BriefsDeps)
	}
	if deps != nil && deps.TrackrDeps != nil {
		RegisterTrackr(r, deps.TrackrDeps)
	}
	if deps != nil && deps.JobReader != nil && deps.SettingsStore != nil {
		r.Get("/api/settings", SettingsHandler(deps.JobReader))
		r.Post("/api/settings", UpdateSettingsHandler(deps.JobReader, deps.SettingsStore))
	}
	if deps != nil && deps.BriefsDeps != nil && deps.TrackrDeps != nil &&
		deps.TrackrDeps.ProjectStore != nil {
		r.Post("/api/admin/clear-ideas", ClearIdeasHandler(deps))
		r.Post("/api/admin/clear-trackr", ClearTrackrHandler(deps))
	}
}
