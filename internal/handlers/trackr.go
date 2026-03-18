package handlers

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/yourname/projctr/internal/linkedin"
	"github.com/yourname/projctr/internal/models"
	"github.com/yourname/projctr/internal/repository"
	"github.com/yourname/projctr/internal/trackr"
)

// TrackrDeps holds dependencies for Trackr handlers.
type TrackrDeps struct {
	Service      *trackr.Service
	BriefStore   *repository.BriefStore
	ProjectStore *repository.ProjectStore
	Generator    *linkedin.Generator // may be nil if LLM disabled
}

// RegisterTrackr adds Trackr routes to the router.
func RegisterTrackr(r chi.Router, deps *TrackrDeps) {
	if deps == nil {
		return
	}

	// HTML pages
	r.Get("/trackr/", trackrDashboardPage(deps))
	r.Get("/trackr/{id}", trackrDetailPage(deps))
	r.Get("/trackr/{id}/export", exportProjectHandler(deps))
	r.Get("/trackr/feed.xml", trackrFeedHandler(deps))

	// API routes
	r.Post("/api/trackr/projects", createProjectHandler(deps))
	r.Get("/api/trackr/projects", listProjectsHandler(deps))
	r.Get("/api/trackr/projects/{id}", getProjectHandler(deps))
	r.Post("/api/trackr/projects/{id}/transition", transitionHandler(deps))
	r.Put("/api/trackr/projects/{id}", updateProjectHandler(deps))
	r.Post("/api/trackr/projects/{id}/generate-post", generatePostHandler(deps))
	r.Put("/api/trackr/projects/{id}/post-draft", savePostDraftHandler(deps))
}

func listProjectsHandler(deps *TrackrDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projects, err := deps.Service.ListAll()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(projects)
	}
}

func getProjectHandler(deps *TrackrDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		p, err := deps.Service.GetByID(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if p == nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(p)
	}
}

type transitionRequest struct {
	To string `json:"to"`
}

func transitionHandler(deps *TrackrDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		var req transitionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		if req.To == "" {
			http.Error(w, `"to" field required`, http.StatusBadRequest)
			return
		}

		updated, err := deps.Service.TransitionStage(id, req.To)
		if err != nil {
			if errors.Is(err, trackr.ErrInvalidTransition) {
				http.Error(w, err.Error(), http.StatusUnprocessableEntity)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(updated)
	}
}

type updateRequest struct {
	GiteaURL   string `json:"gitea_url"`
	LiveURL    string `json:"live_url"`
	Notes      string `json:"notes"`
	Title      string `json:"title"`
	Complexity string `json:"complexity"`
}

func updateProjectHandler(deps *TrackrDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		var req updateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}

		if err := deps.Service.UpdateMetadata(id, req.GiteaURL, req.LiveURL, req.Notes, req.Title, req.Complexity); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		p, err := deps.Service.GetByID(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(p)
	}
}

func generatePostHandler(deps *TrackrDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Generator == nil {
			http.Error(w, "LLM unavailable — check LLM host is running", http.StatusServiceUnavailable)
			return
		}
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		p, err := deps.Service.GetByID(id)
		if err != nil || p == nil {
			http.Error(w, "project not found", http.StatusNotFound)
			return
		}
		var draft string
		if p.BriefID == 0 {
			draft, err = deps.Generator.GenerateFromProject(p.Title, p.Notes)
		} else {
			brief, berr := deps.BriefStore.GetByID(p.BriefID)
			if berr != nil || brief == nil {
				http.Error(w, "brief not found", http.StatusNotFound)
				return
			}
			draft, err = deps.Generator.Generate(brief)
		}
		if err != nil {
			if errors.Is(err, linkedin.ErrLLMUnavailable) {
				http.Error(w, "LLM unavailable — check LLM host is running", http.StatusServiceUnavailable)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := deps.Service.SaveDraft(id, draft); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"draft": draft})
	}
}

type saveDraftRequest struct {
	Draft string `json:"draft"`
}

func savePostDraftHandler(deps *TrackrDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		var req saveDraftRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		if err := deps.Service.SaveDraft(id, req.Draft); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	}
}

type createProjectRequest struct {
	Title      string `json:"title"`
	Complexity string `json:"complexity"`
	Notes      string `json:"notes"`
	GiteaURL   string `json:"gitea_url"`
	LiveURL    string `json:"live_url"`
}

func createProjectHandler(deps *TrackrDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req createProjectRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(req.Title) == "" {
			http.Error(w, "title is required", http.StatusBadRequest)
			return
		}
		if req.Complexity != "" && req.Complexity != "small" && req.Complexity != "medium" && req.Complexity != "large" {
			http.Error(w, "complexity must be small, medium, or large", http.StatusBadRequest)
			return
		}
		p, err := deps.Service.CreateManualProject(req.Title, req.Complexity, req.Notes, req.GiteaURL, req.LiveURL)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(p)
	}
}

// --- HTML page handlers ---

// seedProjects ensures every brief has a corresponding project row.
func seedProjects(deps *TrackrDeps) {
	if deps.BriefStore == nil {
		return
	}
	briefs, err := deps.BriefStore.List()
	if err != nil {
		log.Printf("trackr: seed briefs: %v", err)
		return
	}
	for _, b := range briefs {
		if _, err := deps.Service.EnsureProject(b); err != nil {
			log.Printf("trackr: ensure project for brief %d: %v", b.ID, err)
		}
	}
}

func selAttr(current, value string) string {
	if current == value {
		return ` selected`
	}
	return ""
}

var stageOrder = []string{"candidate", "in_progress", "parked", "published", "archived"}

var stageLabels = map[string]string{
	"candidate":   "Candidates",
	"in_progress": "In Progress",
	"parked":      "Parked",
	"published":   "Published",
	"archived":    "Archived",
}

var validTransitionsUI = map[string][]string{
	"candidate":   {"in_progress", "parked", "archived"},
	"in_progress": {"published", "parked", "candidate"},
	"parked":      {"candidate", "archived"},
	"published":   {"archived"},
	"archived":    {},
}

func trackrDashboardPage(deps *TrackrDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		seedProjects(deps)

		projects, err := deps.Service.ListAll()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Group by stage
		grouped := make(map[string][]*models.ProjectWithBrief)
		for _, p := range projects {
			grouped[p.Stage] = append(grouped[p.Stage], p)
		}

		var body strings.Builder
		for _, stage := range stageOrder {
			items := grouped[stage]
			if len(items) == 0 {
				continue
			}
			body.WriteString(`<h2>` + html.EscapeString(stageLabels[stage]) + ` <span class="count">(` + strconv.Itoa(len(items)) + `)</span></h2>`)
			body.WriteString(`<ul class="project-list">`)
			for _, p := range items {
				esc := html.EscapeString
				body.WriteString(`<li class="stage-` + esc(p.Stage) + `">`)
				body.WriteString(`<a href="/trackr/` + strconv.FormatInt(p.ID, 10) + `">` + esc(p.DisplayTitle()) + `</a>`)
				if dc := p.DisplayComplexity(); dc != "" {
					body.WriteString(` <span class="badge">` + esc(dc) + `</span>`)
				}
				body.WriteString(`<div class="actions">`)
				for _, next := range validTransitionsUI[p.Stage] {
					body.WriteString(fmt.Sprintf(
						`<button onclick="transition(%d,'%s')">%s</button>`,
						p.ID, next, stageLabels[next],
					))
				}
				body.WriteString(`</div></li>`)
			}
			body.WriteString(`</ul>`)
		}

		if len(projects) == 0 {
			body.WriteString(`<p class="empty">No projects yet. Generate briefs in Projctr first, or add one manually below.</p>`)
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head><title>Trackr — Portfolio Tracker</title>
<style>
body{font-family:system-ui,sans-serif;max-width:780px;margin:2rem auto;padding:0 1rem;line-height:1.5}
h1{font-size:1.5rem;margin-bottom:0.25rem}
h2{font-size:1.15rem;margin-top:1.5rem;color:#333}
.count{font-weight:normal;color:#888;font-size:0.9rem}
.back{display:inline-block;margin-bottom:1rem;color:#0066cc;text-decoration:none;font-size:0.9rem}
.back:hover{text-decoration:underline}
.project-list{list-style:none;padding:0;margin:0}
.project-list li{margin:0.5rem 0;padding:0.75rem;background:#f8f9fa;border-radius:6px;border-left:3px solid #0066cc;display:flex;align-items:center;justify-content:space-between;flex-wrap:wrap;gap:0.5rem}
.project-list li.stage-in_progress{border-left-color:#e6a817}
.project-list li.stage-parked{border-left-color:#888}
.project-list li.stage-published{border-left-color:#2a7a2a}
.project-list li.stage-archived{border-left-color:#ccc}
.project-list a{color:#0066cc;text-decoration:none;font-weight:500}
.project-list a:hover{text-decoration:underline}
.badge{display:inline-block;background:#e8e8e8;color:#555;padding:0.1rem 0.5rem;border-radius:3px;font-size:0.8rem;margin-left:0.4rem}
.actions{display:flex;gap:0.3rem}
.actions button{background:#f0f0f0;color:#333;border:1px solid #ccc;padding:0.25rem 0.6rem;border-radius:4px;cursor:pointer;font-size:0.8rem}
.actions button:hover{background:#e0e0e0}
.empty{color:#666;font-style:italic}
.top-actions{display:flex;align-items:center;justify-content:space-between;margin-bottom:0.5rem}
button.danger{background:#dc3545;color:#fff;border:none;padding:0.35rem 0.85rem;border-radius:4px;cursor:pointer;font-size:0.85rem}
button.danger:hover{background:#b02a37}
</style>
</head>
<body>
<a class="back" href="/">← Projctr Dashboard</a>
<div class="top-actions">
<h1 style="margin:0">Trackr</h1>
<button class="danger" onclick="clearTrackr()">Clear Trackr</button>
</div>
<div style="margin-bottom:1rem">
<button onclick="document.getElementById('add-form').style.display=document.getElementById('add-form').style.display==='none'?'block':'none'" style="background:#0066cc;color:#fff;border:none;padding:0.35rem 0.85rem;border-radius:4px;cursor:pointer;font-size:0.85rem">+ Add Project</button>
<div id="add-form" style="display:none;background:#f8f9fa;padding:1rem;border-radius:6px;margin-top:0.5rem">
<div style="margin:0.4rem 0"><label style="font-size:0.85rem;color:#555">Title *</label><input id="new-title" style="width:100%%;padding:0.4rem;border:1px solid #ccc;border-radius:4px;box-sizing:border-box"></div>
<div style="margin:0.4rem 0"><label style="font-size:0.85rem;color:#555">Complexity</label><select id="new-complexity" style="padding:0.4rem;border:1px solid #ccc;border-radius:4px"><option value="">—</option><option value="small">small</option><option value="medium">medium</option><option value="large">large</option></select></div>
<div style="margin:0.4rem 0"><label style="font-size:0.85rem;color:#555">Notes</label><textarea id="new-notes" rows="2" style="width:100%%;padding:0.4rem;border:1px solid #ccc;border-radius:4px;box-sizing:border-box"></textarea></div>
<button onclick="createProject()" style="margin-top:0.4rem">Create</button>
</div>
</div>
%s
<script>
function transition(id, to) {
  fetch('/api/trackr/projects/'+id+'/transition', {
    method:'POST',
    headers:{'Content-Type':'application/json'},
    body:JSON.stringify({to:to})
  }).then(r=>{
    if(!r.ok) return r.text().then(t=>{throw new Error(t)});
    location.reload();
  }).catch(e=>alert('Transition failed: '+e.message));
}
function createProject() {
  var title=document.getElementById('new-title').value.trim();
  if(!title){alert('Title is required');return;}
  fetch('/api/trackr/projects', {
    method:'POST',
    headers:{'Content-Type':'application/json'},
    body:JSON.stringify({
      title:title,
      complexity:document.getElementById('new-complexity').value,
      notes:document.getElementById('new-notes').value
    })
  }).then(r=>{
    if(!r.ok) return r.text().then(t=>{throw new Error(t)});
    location.reload();
  }).catch(e=>alert('Create failed: '+e.message));
}
function clearTrackr() {
  if (!confirm('This will delete all Trackr project records (stage tracking, notes, URLs). Briefs and pipeline data will be kept. Are you sure?')) return;
  fetch('/api/admin/clear-trackr', {method:'POST'})
    .then(r=>{ if (!r.ok) return r.text().then(t=>{ throw new Error(t); }); })
    .then(()=>location.reload())
    .catch(e=>alert('Clear failed: '+e));
}
</script>
</body>
</html>`, body.String())
	}
}

func trackrDetailPage(deps *TrackrDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		p, err := deps.Service.GetByID(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if p == nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		esc := html.EscapeString
		// Determine display title: brief title for brief-sourced, project title for manual
		var title string
		var briefTitle string
		if p.BriefID != 0 {
			if brief, err := deps.BriefStore.GetByID(p.BriefID); err == nil && brief != nil {
				briefTitle = brief.Title
			}
		}
		if briefTitle != "" {
			title = briefTitle
		} else if p.Title != "" {
			title = p.Title
		} else {
			title = fmt.Sprintf("Project #%d", p.ID)
		}
		isManual := p.BriefID == 0

		// Build transition buttons
		var buttons strings.Builder
		for _, next := range validTransitionsUI[p.Stage] {
			buttons.WriteString(fmt.Sprintf(
				`<button onclick="transition(%d,'%s')">Move to %s</button> `,
				p.ID, next, stageLabels[next],
			))
		}

		// Manual project fields (title + complexity editable only for manual projects)
		var manualFields string
		if isManual {
			manualFields = fmt.Sprintf(
				`<div class="form-row"><label for="meta_title">Title</label><input id="meta_title" value="%s"></div>`+
					`<div class="form-row"><label for="meta_complexity">Complexity</label><select id="meta_complexity"><option value="">—</option><option value="small"%s>small</option><option value="medium"%s>medium</option><option value="large"%s>large</option></select></div>`,
				esc(p.Title),
				selAttr(p.Complexity, "small"), selAttr(p.Complexity, "medium"), selAttr(p.Complexity, "large"),
			)
		}

		// LinkedIn draft section
		var draftSection string
		if p.LinkedInDraft != "" {
			draftSection = fmt.Sprintf(`<div class="section">
<h2>LinkedIn Draft</h2>
<textarea id="draft" rows="8" style="width:100%%">%s</textarea>
<div class="draft-actions">
<button onclick="generatePost()">Regenerate</button>
<button onclick="saveDraft()">Save Draft</button>
<button onclick="copyDraft()">Copy to Clipboard</button>
</div>
<span class="feedback" id="draft-feedback"></span>
</div>`, esc(p.LinkedInDraft))
		} else {
			draftSection = `<div class="section">
<h2>LinkedIn Draft</h2>
<p class="empty" id="no-draft">No draft yet.</p>
<textarea id="draft" rows="8" style="width:100%;display:none"></textarea>
<div class="draft-actions">
<button onclick="generatePost()">Generate LinkedIn Post</button>
<button onclick="saveDraft()" style="display:none" id="btn-save-draft">Save Draft</button>
<button onclick="copyDraft()" style="display:none" id="btn-copy-draft">Copy to Clipboard</button>
</div>
<span class="feedback" id="draft-feedback"></span>
</div>`
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head><title>%s — Trackr</title>
<style>
body{font-family:system-ui,sans-serif;max-width:720px;margin:2rem auto;padding:0 1rem;line-height:1.5}
h1{font-size:1.5rem;margin-bottom:0.25rem}
h2{font-size:1.1rem;margin-top:1.5rem;color:#333}
.back{display:inline-block;margin-bottom:1rem;color:#0066cc;text-decoration:none;font-size:0.9rem}
.back:hover{text-decoration:underline}
.stage-label{display:inline-block;background:#e8e8e8;color:#555;padding:0.15rem 0.6rem;border-radius:3px;font-size:0.9rem}
.section{background:#f8f9fa;padding:1rem;border-radius:6px;margin:0.75rem 0}
.form-row{margin:0.5rem 0}
.form-row label{display:block;font-size:0.85rem;color:#555;margin-bottom:0.2rem}
.form-row input,.form-row textarea{width:100%%;padding:0.4rem;border:1px solid #ccc;border-radius:4px;font-size:0.9rem;box-sizing:border-box}
.form-row textarea{resize:vertical}
button{background:#0066cc;color:#fff;border:none;padding:0.4rem 1rem;border-radius:4px;cursor:pointer;font-size:0.9rem;margin-right:0.3rem}
button:hover{background:#0052a3}
button.secondary{background:#f0f0f0;color:#333;border:1px solid #ccc}
button.secondary:hover{background:#e0e0e0}
.transitions{margin:1rem 0}
.transitions button{background:#f0f0f0;color:#333;border:1px solid #ccc}
.transitions button:hover{background:#e0e0e0}
.draft-actions{margin-top:0.5rem;display:flex;gap:0.3rem}
.empty{color:#666;font-style:italic}
.feedback{color:#2a7a2a;font-size:0.85rem;display:none;margin-top:0.3rem}
#draft{font-family:system-ui,sans-serif}
</style>
</head>
<body>
<a class="back" href="/trackr/">← Back to Trackr</a>
<h1>%s</h1>
<p>Stage: <span class="stage-label">%s</span></p>

<div class="transitions">%s</div>

<div class="section">
<h2>Metadata</h2>
%s
<div class="form-row"><label for="gitea_url">Gitea URL</label><input id="gitea_url" value="%s"></div>
<div class="form-row"><label for="live_url">Live URL</label><input id="live_url" value="%s"></div>
<div class="form-row"><label for="notes">Notes</label><textarea id="notes" rows="3">%s</textarea></div>
<button onclick="saveMeta()">Save</button>
<span class="feedback" id="meta-feedback">Saved.</span>
</div>

%s

<script>
function transition(id, to) {
  fetch('/api/trackr/projects/'+id+'/transition', {
    method:'POST',
    headers:{'Content-Type':'application/json'},
    body:JSON.stringify({to:to})
  }).then(r=>{
    if(!r.ok) return r.text().then(t=>{throw new Error(t)});
    location.reload();
  }).catch(e=>alert('Transition failed: '+e.message));
}
function saveMeta() {
  var payload={
    gitea_url: document.getElementById('gitea_url').value,
    live_url: document.getElementById('live_url').value,
    notes: document.getElementById('notes').value,
    title: (document.getElementById('meta_title')||{}).value||'',
    complexity: (document.getElementById('meta_complexity')||{}).value||''
  };
  fetch('/api/trackr/projects/%d', {
    method:'PUT',
    headers:{'Content-Type':'application/json'},
    body:JSON.stringify(payload)
  }).then(r=>{
    if(!r.ok) return r.text().then(t=>{throw new Error(t)});
    var fb=document.getElementById('meta-feedback');fb.style.display='inline';setTimeout(function(){fb.style.display='none'},2500);
  }).catch(e=>alert('Save failed: '+e.message));
}
function saveDraft() {
  var el=document.getElementById('draft');if(!el)return;
  fetch('/api/trackr/projects/%d/post-draft', {
    method:'PUT',
    headers:{'Content-Type':'application/json'},
    body:JSON.stringify({draft:el.value})
  }).then(r=>{if(!r.ok) throw new Error('save failed')})
  .catch(e=>alert(e.message));
}
function copyDraft() {
  var el=document.getElementById('draft');if(!el)return;
  navigator.clipboard.writeText(el.value).then(()=>alert('Copied!')).catch(()=>alert('Copy failed'));
}
function generatePost() {
  var fb=document.getElementById('draft-feedback');
  fb.textContent='Generating...';fb.style.display='inline';fb.style.color='#666';
  fetch('/api/trackr/projects/%d/generate-post', {
    method:'POST',
    headers:{'Content-Type':'application/json'}
  }).then(r=>{
    if(!r.ok) return r.text().then(t=>{throw new Error(t)});
    return r.json();
  }).then(d=>{
    var el=document.getElementById('draft');
    el.value=d.draft;el.style.display='block';
    var nd=document.getElementById('no-draft');if(nd)nd.style.display='none';
    var bs=document.getElementById('btn-save-draft');if(bs)bs.style.display='';
    var bc=document.getElementById('btn-copy-draft');if(bc)bc.style.display='';
    fb.textContent='Generated!';fb.style.color='#2a7a2a';
    setTimeout(function(){fb.style.display='none'},2500);
  }).catch(e=>{
    fb.textContent='Error: '+e.message;fb.style.color='#c00';
  });
}
</script>
</body>
</html>`,
			esc(title), esc(title), esc(stageLabels[p.Stage]),
			buttons.String(),
			manualFields,
			esc(p.GiteaURL), esc(p.LiveURL), esc(p.Notes),
			draftSection,
			p.ID, p.ID, p.ID,
		)
	}
}

// --- Task 3.1: Markdown export ---

func exportProjectHandler(deps *TrackrDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		p, err := deps.Service.GetByID(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if p == nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/markdown")
		w.Header().Set("Content-Disposition", "attachment; filename=project-"+idStr+".md")

		if p.BriefID == 0 {
			// Manual project — no brief
			w.Write([]byte("# " + p.Title + "\n\n"))
			w.Write([]byte("**Stage:** " + stageLabels[p.Stage] + "\n\n"))
			if p.Complexity != "" {
				w.Write([]byte("**Complexity:** " + p.Complexity + "\n\n"))
			}
			if p.GiteaURL != "" {
				w.Write([]byte("**Gitea:** " + p.GiteaURL + "\n\n"))
			}
			if p.LiveURL != "" {
				w.Write([]byte("**Live:** " + p.LiveURL + "\n\n"))
			}
			if p.Notes != "" {
				w.Write([]byte("## Notes\n\n" + p.Notes + "\n\n"))
			}
			if p.LinkedInDraft != "" {
				w.Write([]byte("## LinkedIn Draft\n\n" + p.LinkedInDraft + "\n"))
			}
		} else {
			brief, err := deps.BriefStore.GetByID(p.BriefID)
			if err != nil || brief == nil {
				http.Error(w, "brief not found", http.StatusNotFound)
				return
			}

			w.Write([]byte("# " + brief.Title + "\n\n"))
			w.Write([]byte("**Stage:** " + stageLabels[p.Stage] + "\n\n"))
			if p.GiteaURL != "" {
				w.Write([]byte("**Gitea:** " + p.GiteaURL + "\n\n"))
			}
			if p.LiveURL != "" {
				w.Write([]byte("**Live:** " + p.LiveURL + "\n\n"))
			}
			w.Write([]byte("## Problem Statement\n\n" + brief.ProblemStatement + "\n\n"))
			w.Write([]byte("## Suggested Approach\n\n" + brief.SuggestedApproach + "\n\n"))
			w.Write([]byte("## Technology Stack\n\n" + brief.TechnologyStack + "\n\n"))
			w.Write([]byte("## Complexity\n\n" + brief.Complexity + "\n\n"))
			if p.Notes != "" {
				w.Write([]byte("## Notes\n\n" + p.Notes + "\n\n"))
			}
			if p.LinkedInDraft != "" {
				w.Write([]byte("## LinkedIn Draft\n\n" + p.LinkedInDraft + "\n"))
			}
		}
	}
}

// --- Task 3.4: RSS/Atom feed ---

type atomFeed struct {
	XMLName xml.Name    `xml:"feed"`
	XMLNS   string      `xml:"xmlns,attr"`
	Title   string      `xml:"title"`
	Link    atomLink    `xml:"link"`
	Updated string      `xml:"updated"`
	Entries []atomEntry `xml:"entry"`
}

type atomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr,omitempty"`
}

type atomEntry struct {
	Title   string   `xml:"title"`
	Link    atomLink `xml:"link"`
	ID      string   `xml:"id"`
	Updated string   `xml:"updated"`
	Summary string   `xml:"summary"`
}

func trackrFeedHandler(deps *TrackrDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projects, err := deps.Service.ListAll()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Filter to published projects only
		var published []*models.ProjectWithBrief
		for _, p := range projects {
			if p.Stage == "published" {
				published = append(published, p)
			}
		}

		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		baseURL := scheme + "://" + r.Host

		feedUpdated := time.Now().UTC().Format(time.RFC3339)
		if len(published) > 0 && published[0].DatePublished != nil {
			feedUpdated = published[0].DatePublished.UTC().Format(time.RFC3339)
		}

		entries := make([]atomEntry, 0, len(published))
		for _, p := range published {
			updated := p.DateCreated.UTC().Format(time.RFC3339)
			if p.DatePublished != nil {
				updated = p.DatePublished.UTC().Format(time.RFC3339)
			}

			var summary string
			if p.GiteaURL != "" {
				summary += "Gitea: " + p.GiteaURL + "\n"
			}
			if p.LiveURL != "" {
				summary += "Live: " + p.LiveURL + "\n"
			}
			if p.LinkedInDraft != "" {
				summary += "\n" + p.LinkedInDraft
			}

			entries = append(entries, atomEntry{
				Title:   p.DisplayTitle(),
				Link:    atomLink{Href: baseURL + "/trackr/" + strconv.FormatInt(p.ID, 10)},
				ID:      baseURL + "/trackr/" + strconv.FormatInt(p.ID, 10),
				Updated: updated,
				Summary: strings.TrimSpace(summary),
			})
		}

		feed := atomFeed{
			XMLNS:   "http://www.w3.org/2005/Atom",
			Title:   "Trackr — Published Projects",
			Link:    atomLink{Href: baseURL + "/trackr/feed.xml", Rel: "self"},
			Updated: feedUpdated,
			Entries: entries,
		}

		w.Header().Set("Content-Type", "application/atom+xml; charset=utf-8")
		w.Write([]byte(xml.Header))
		xml.NewEncoder(w).Encode(feed)
	}
}
