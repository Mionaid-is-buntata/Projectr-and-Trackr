package handlers

import (
	"encoding/json"
	"errors"
	"html"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/campbell/projctr/internal/briefs"
	"github.com/campbell/projctr/internal/models"
	"github.com/campbell/projctr/internal/repository"
	"github.com/campbell/projctr/internal/trackr"
)

// BriefsDeps holds dependencies for brief handlers.
type BriefsDeps struct {
	Generator    *briefs.Generator
	BriefStore   *repository.BriefStore
	ClusterStore *repository.ClusterStore
	DescStore    *repository.DescriptionStore
	ProjectStore *repository.ProjectStore // optional: resolve Trackr project id for brief page
	Trackr       *trackr.Service          // optional: ensure project row exists when viewing a brief
}

// RegisterBriefs adds brief-related routes.
func RegisterBriefs(r chi.Router, deps *BriefsDeps) {
	if deps == nil {
		return
	}
	r.Get("/api/briefs", listBriefsHandler(deps))
	r.Post("/api/briefs/generate", generateBriefHandler(deps))
	r.Get("/api/briefs/{id}", getBriefHandler(deps))
	r.Put("/api/briefs/{id}", updateBriefHandler(deps))
	r.Get("/api/briefs/{id}/export", exportBriefHandler(deps))
	r.Post("/api/briefs/{id}/refine", refineBriefHandler(deps))
	r.Get("/briefs/{id}", briefDetailPageHandler(deps))
}

func listBriefsHandler(deps *BriefsDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		briefs, err := deps.BriefStore.List()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		out := make([]models.Brief, len(briefs))
		for i, b := range briefs {
			out[i] = *b
			out[i].Title = b.DisplayTitle()
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(out)
	}
}

type generateRequest struct {
	ClusterID   *int64 `json:"cluster_id"`
	DescriptionID *int64 `json:"description_id"` // optional: create synthetic cluster from description
}

func generateBriefHandler(deps *BriefsDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req generateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}

		var cluster *models.Cluster
		if req.ClusterID != nil {
			var err error
			cluster, err = deps.ClusterStore.GetByID(*req.ClusterID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if cluster == nil {
				http.Error(w, "cluster not found", http.StatusNotFound)
				return
			}
		} else if req.DescriptionID != nil && deps.DescStore != nil {
			// Create synthetic cluster from description
			desc, err := deps.DescStore.GetByID(*req.DescriptionID)
			if err != nil || desc == nil {
				http.Error(w, "description not found", http.StatusNotFound)
				return
			}
			cluster = &models.Cluster{
				Summary:       desc.RoleTitle + " at " + desc.Sector,
				Frequency:     1,
				GapType:       "skill_acquisition",
				DateClustered: time.Now(),
			}
			id, err := deps.ClusterStore.Insert(cluster)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			cluster.ID = id
			// Attach source company/role from the Huntr description.
			// Note: Sector holds the company name (Huntr's mapping).
			b := deps.Generator.GenerateFromCluster(cluster)
			b.SourceCompany = desc.Sector
			b.SourceRole = desc.RoleTitle
			briefID, err := deps.BriefStore.Insert(b)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			b.ID = briefID
			if err := applyBriefTitleAfterInsert(deps, b); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(b)
			return
		} else {
			// Try first available cluster
			clusters, err := deps.ClusterStore.List()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if len(clusters) == 0 {
				http.Error(w, "no clusters available; provide cluster_id or description_id", http.StatusBadRequest)
				return
			}
			cluster = clusters[0]
		}

		b := deps.Generator.GenerateFromCluster(cluster)
		id, err := deps.BriefStore.Insert(b)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		b.ID = id
		if err := applyBriefTitleAfterInsert(deps, b); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(b)
	}
}

func applyBriefTitleAfterInsert(deps *BriefsDeps, b *models.Brief) error {
	if deps == nil || deps.Generator == nil || deps.BriefStore == nil {
		return nil
	}
	t := deps.Generator.FinalizeBriefTitle(b.ID, b.ProblemStatement)
	if err := deps.BriefStore.SetGeneratedTitle(b.ID, t); err != nil {
		return err
	}
	b.Title = t
	return nil
}

func getBriefHandler(deps *BriefsDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		b, err := deps.BriefStore.GetByID(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if b == nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		out := *b
		out.Title = b.DisplayTitle()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(&out)
	}
}

type updateBriefRequest struct {
	Title string `json:"title"`
}

func updateBriefHandler(deps *BriefsDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		b, err := deps.BriefStore.GetByID(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if b == nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		var req updateBriefRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		title := strings.TrimSpace(req.Title)
		if title == "" {
			http.Error(w, "title is required and cannot be empty", http.StatusBadRequest)
			return
		}
		if err := deps.BriefStore.UpdateTitle(id, title); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		b, _ = deps.BriefStore.GetByID(id)
		if b != nil {
			out := *b
			out.Title = b.DisplayTitle()
			b = &out
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(b)
	}
}

func refineBriefHandler(deps *BriefsDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		b, err := deps.BriefStore.GetByID(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if b == nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		refined, err := deps.Generator.Refine(b.ClusterID)
		if err != nil {
			if errors.Is(err, briefs.ErrFrancisUnavailable) {
				http.Error(w, "Francis is offline — try again when it's back", http.StatusServiceUnavailable)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := deps.BriefStore.UpdateFromFrancis(id, refined.Title, refined.ProblemStatement, refined.SuggestedApproach, refined.LinkedInAngle, refined.Complexity, refined.Source, refined.ImpactScore); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		b, _ = deps.BriefStore.GetByID(id)
		if b != nil {
			out := *b
			out.Title = b.DisplayTitle()
			b = &out
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(b)
	}
}

func exportBriefHandler(deps *BriefsDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		b, err := deps.BriefStore.GetByID(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if b == nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/markdown")
		w.Header().Set("Content-Disposition", "attachment; filename=brief-"+idStr+".md")
		exportMarkdown(w, b)
	}
}

func exportMarkdown(w http.ResponseWriter, b *models.Brief) {
	w.Write([]byte("# " + b.DisplayTitle() + "\n\n"))
	if b.SourceRole != "" || b.SourceCompany != "" {
		w.Write([]byte("**" + b.SourceRole + " — " + b.SourceCompany + "**\n\n"))
	}
	w.Write([]byte("## Problem Statement\n\n" + b.ProblemStatement + "\n\n"))
	w.Write([]byte("## Suggested Approach\n\n" + b.SuggestedApproach + "\n\n"))
	w.Write([]byte("## Technology Stack\n\n" + b.TechnologyStack + "\n\n"))
	w.Write([]byte("## Project Layout\n\n" + b.ProjectLayout + "\n\n"))
	w.Write([]byte("## Complexity\n\n" + b.Complexity + "\n\n"))
}

func briefDetailPageHandler(deps *BriefsDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		b, err := deps.BriefStore.GetByID(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if b == nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		var trackrID int64
		if deps.Trackr != nil {
			if _, err := deps.Trackr.EnsureProject(b); err != nil {
				// non-fatal: page still useful without Trackr link
			}
		}
		if deps.ProjectStore != nil {
			if p, err := deps.ProjectStore.GetByBriefID(id); err == nil && p != nil {
				trackrID = p.ID
			}
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		renderBriefHTML(w, b, trackrID, deps.Generator != nil && deps.Generator.CanRefine())
	}
}

func renderBriefHTML(w http.ResponseWriter, b *models.Brief, trackrProjectID int64, canRefine bool) {
	escape := html.EscapeString
	title := escape(b.DisplayTitle())
	problem := escape(b.ProblemStatement)
	approach := escape(b.SuggestedApproach)
	techStack := escape(b.TechnologyStack)
	layout := escape(b.ProjectLayout)
	complexity := escape(b.Complexity)
	linkedIn := escape(b.LinkedInAngle)
	sourceRole := escape(b.SourceRole)
	sourceCompany := escape(b.SourceCompany)

	page := `<!DOCTYPE html>
<html>
<head><title>` + title + `</title>
<style>
body{font-family:system-ui,sans-serif;max-width:720px;margin:2rem auto;padding:0 1rem;line-height:1.5}
h1{font-size:1.5rem;margin-bottom:0.25rem}
h2{font-size:1.1rem;margin-top:1.5rem;color:#333}
.subheader{font-size:0.95rem;color:#555;margin-bottom:0.25rem}
.subheader strong{color:#333}
.back{display:inline-block;margin-bottom:1rem;color:#0066cc;text-decoration:none}
.back:hover{text-decoration:underline}
.trackr-link{margin:0.5rem 0}
.trackr-link a{color:#0066cc;text-decoration:none;font-weight:500}
.trackr-link a:hover{text-decoration:underline}
.section{background:#f8f9fa;padding:1rem;border-radius:6px;margin:0.5rem 0}
pre{white-space:pre-wrap;font-size:0.9rem;overflow-x:auto}
.export{display:inline-block;margin-top:1rem;padding:0.4rem 0.8rem;background:#0066cc;color:white;text-decoration:none;border-radius:4px;font-size:0.9rem}
.export:hover{background:#0052a3}
.refine-btn{margin-top:1rem;margin-left:0.5rem;padding:0.4rem 0.8rem;background:#6f42c1;color:white;border:none;border-radius:4px;font-size:0.9rem;cursor:pointer}
.refine-btn:hover{background:#5a32a3}
.refine-feedback{font-size:0.85rem;margin-left:0.5rem}
</style>
</head>
<body>
<a class="back" href="/">← Back to projects</a>
<h1>` + title + `</h1>`

	if trackrProjectID != 0 {
		page += `<p class="trackr-link"><a href="/trackr/` + strconv.FormatInt(trackrProjectID, 10) + `">Open in Trackr</a> <span style="color:#666;font-size:0.9rem">(same brief — stage &amp; links)</span></p>`
	}

	if sourceRole != "" || sourceCompany != "" {
		page += `<p class="subheader"><strong>` + sourceRole + `</strong> — ` + sourceCompany + `</p>`
	}

	page += `<p><em>Complexity: ` + complexity + `</em></p>

<div class="section"><h2>Problem Statement</h2><p>` + problem + `</p></div>
<div class="section"><h2>Suggested Approach</h2><pre>` + approach + `</pre></div>
<div class="section"><h2>Technology Stack</h2><pre>` + techStack + `</pre></div>
<div class="section"><h2>Project Layout</h2><pre>` + layout + `</pre></div>`

	if linkedIn != "" {
		page += `<div class="section"><h2>LinkedIn Angle</h2><p>` + linkedIn + `</p></div>`
	}
	briefIDStr := strconv.FormatInt(b.ID, 10)
	page += `<div style="margin-top:1rem">` +
		`<a class="export" href="/api/briefs/` + briefIDStr + `/export">Download as Markdown</a>`

	if canRefine && b.GenerationSource != "francis" {
		page += `<button class="refine-btn" onclick="refineWithFrancis()">Refine with Francis (model)</button>` +
			`<span class="refine-feedback" id="refine-fb"></span>`
	}
	page += `</div>
<script>
function refineWithFrancis() {
  var fb = document.getElementById('refine-fb');
  fb.textContent = 'Sending to Francis…'; fb.style.color = '#6f42c1';
  fetch('/api/briefs/` + briefIDStr + `/refine', {method:'POST'})
    .then(function(r) {
      if (!r.ok) return r.text().then(function(t) { throw new Error(t); });
      fb.textContent = 'Done! Reloading…'; fb.style.color = '#2a7a2a';
      setTimeout(function() { location.reload(); }, 800);
    })
    .catch(function(e) {
      fb.textContent = e.message; fb.style.color = '#c00';
    });
}
</script>
</body>
</html>`
	w.Write([]byte(page))
}
