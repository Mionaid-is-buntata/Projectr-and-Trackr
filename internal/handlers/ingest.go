package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/yourname/projctr/internal/ingestion"
	"github.com/yourname/projctr/internal/pipeline"
	"github.com/yourname/projctr/internal/repository"
)

// LastIngestResult holds the most recent ingest run stats for dashboard display.
var LastIngestResult *ingestion.Result

// IngestHandler runs the ingestion pipeline then triggers the post-ingest pipeline (POST /api/ingest).
func IngestHandler(deps *Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		result, err := deps.Pipeline.Run(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		LastIngestResult = result

		// Trigger full pipeline asynchronously when new data was ingested.
		if result.Ingested > 0 && deps.PipelineSvc != nil {
			go deps.PipelineSvc.RunPostIngest(context.Background())
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ingested": result.Ingested,
			"skipped":  result.Skipped,
		})
	}
}

// IngestStatusHandler returns ingestion stats (GET /api/ingest/status).
func IngestStatusHandler(store *repository.DescriptionStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		count, err := store.Count()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"descriptions_count": count,
		})
	}
}

// DashboardPageHandler serves the dashboard HTML (GET /).
func DashboardPageHandler(deps *Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`<!DOCTYPE html>
<html>
<head><title>Projctr — Project Ideas</title>
<style>
body{font-family:system-ui,sans-serif;max-width:720px;margin:2rem auto;padding:0 1rem;line-height:1.5}
h1{font-size:1.5rem}
.stats{color:#666;font-size:0.9rem;margin-bottom:1rem}
.actions{margin-bottom:1rem;display:flex;gap:0.5rem;align-items:center;flex-wrap:wrap}
button{background:#0066cc;color:#fff;border:none;padding:0.4rem 1rem;border-radius:4px;cursor:pointer;font-size:0.9rem}
button:hover{background:#0052a3}
button.secondary{background:#f0f0f0;color:#333;border:1px solid #ccc}
button.secondary:hover{background:#e0e0e0}
.score-band{background:#f8f9fa;border:1px solid #ddd;border-radius:6px;padding:0.75rem 1rem;margin-bottom:1.5rem;font-size:0.9rem}
.score-band label{margin-right:0.4rem;color:#444}
.score-band input[type=number]{width:5rem;padding:0.25rem 0.4rem;border:1px solid #ccc;border-radius:4px;font-size:0.9rem}
.score-band .hint{color:#888;font-size:0.8rem;margin-top:0.4rem}
.score-band .feedback{color:#2a7a2a;font-size:0.8rem;margin-top:0.3rem;display:none}
.project-list{list-style:none;padding:0;margin:0}
.project-list li{margin:0.5rem 0;padding:0.75rem;background:#f8f9fa;border-radius:6px;border-left:3px solid #0066cc}
.project-list a{color:#0066cc;text-decoration:none;font-weight:500}
.project-list a:hover{text-decoration:underline}
.project-list .source{font-size:0.9rem;color:#444;margin-top:0.15rem}
.project-list .meta{font-size:0.85rem;color:#666;margin-top:0.15rem}
.empty{color:#666;font-style:italic}
nav{margin-bottom:1rem;font-size:0.85rem}
nav a.doc-btn{display:inline-block;background:#f0f0f0;color:#333;text-decoration:none;padding:0.35rem 0.85rem;border-radius:4px;margin-right:0.5rem;border:1px solid #ccc;font-size:0.85rem}
nav a.doc-btn:hover{background:#e0e0e0}
.ingest-status{padding:0.5rem 0.75rem;border-radius:4px;font-size:0.9rem;margin-bottom:1rem}
.ingest-status.running{background:#fff3cd;color:#856404;border:1px solid #ffc107}
.ingest-status.success{background:#d4edda;color:#155724;border:1px solid #28a745}
.ingest-status.error{background:#f8d7da;color:#721c24;border:1px solid #dc3545}
</style>
</head>
<body>
  <h1>Projctr</h1>
  <nav>
    <a class="doc-btn" href="/trackr/">Trackr</a>
    <a class="doc-btn" href="/docs/how-to" target="_blank" rel="noopener noreferrer">How-To Guide</a>
    <a class="doc-btn" href="/docs/api" target="_blank" rel="noopener noreferrer">API Reference</a>
  </nav>
  <div class="stats">
    Descriptions: <span id="count">—</span> ·
    Pain points: <span id="pain-points">—</span> ·
    Clusters: <span id="clusters">—</span> ·
    Briefs: <span id="briefs-count">—</span> ·
    Skipped (last run): <span id="skipped">—</span> ·
    Pipeline: <span id="phase">—</span>
  </div>

  <div class="score-band">
    <strong>Huntr score band</strong> — jobs ingested when score is between min (inclusive) and max (exclusive)<br>
    <div style="margin-top:0.5rem">
      <label for="score-min">Min:</label>
      <input type="number" id="score-min" min="0" step="10" value="0">
      &nbsp;
      <label for="score-max">Max:</label>
      <input type="number" id="score-max" min="1" step="10" value="300">
      &nbsp;
      <button class="secondary" onclick="saveScoreBand()">Save</button>
    </div>
    <div class="hint">Low min excludes very weak matches; low max catches near-match roles with small gaps. Persisted across restarts.</div>
    <div class="feedback" id="band-feedback">Saved.</div>
  </div>

  <div class="actions">
    <button onclick="runIngest()">Run Ingest + Pipeline</button>
  </div>
  <div class="ingest-status" id="ingest-status" style="display:none"></div>
  <h2>Project Ideas</h2>
  <ul id="projects" class="project-list"></ul>
  <script>
    function load() {
      fetch('/api/dashboard').then(r=>r.json()).then(d=>{
        document.getElementById('count').textContent        = d.descriptions_ingested ?? '—';
        document.getElementById('pain-points').textContent  = d.pain_points_extracted ?? '—';
        document.getElementById('clusters').textContent     = d.clusters_found ?? '—';
        document.getElementById('briefs-count').textContent = d.briefs_generated ?? '—';
        document.getElementById('skipped').textContent      = d.duplicates_skipped ?? '—';
        document.getElementById('phase').textContent        = d.pipeline_phase ?? '—';
      }).catch(()=>{});
      fetch('/api/settings').then(r=>r.json()).then(s=>{
        document.getElementById('score-min').value = s.score_min ?? 0;
        document.getElementById('score-max').value = s.score_max ?? 300;
      }).catch(()=>{});
      fetch('/api/briefs').then(r=>r.json()).then(briefs=>{
        const ul = document.getElementById('projects');
        if (!briefs || briefs.length === 0) {
          ul.innerHTML = '<li class="empty">No project briefs yet. Run ingest to generate briefs automatically.</li>';
          return;
        }
        ul.innerHTML = briefs.map(function(b){
          var sub = (b.source_role||b.source_company) ? '<div class="source">'+escapeHtml(b.source_role||'')+(b.source_role&&b.source_company?' — ':'')+escapeHtml(b.source_company||'')+'</div>' : '';
          return '<li><a href="/briefs/'+b.id+'">'+escapeHtml(b.title)+'</a>'+sub+'<div class="meta">'+(b.complexity||'')+' · '+(b.technology_stack||'')+'</div></li>';
        }).join('');
      }).catch(function(){ document.getElementById('projects').innerHTML = '<li class="empty">Could not load projects.</li>'; });
    }
    function escapeHtml(s){ return (s||'').replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;'); }
    function runIngest() {
      var btn = document.querySelector('.actions button');
      btn.disabled = true;
      btn.textContent = 'Ingesting…';
      btn.style.opacity = '0.7';
      var status = document.getElementById('ingest-status');
      status.style.display = 'block';
      status.textContent = 'Running ingest pipeline — this may take a minute…';
      status.className = 'ingest-status running';
      fetch('/api/ingest', {method:'POST'})
        .then(r=>{ if (!r.ok) throw new Error('Server error '+r.status); return r.json(); })
        .then(d=>{
          status.textContent = 'Done — ingested: '+d.ingested+', skipped: '+d.skipped+'. Pipeline running in background.';
          status.className = 'ingest-status success';
          load();
        })
        .catch(e=>{
          status.textContent = 'Ingest failed: '+e;
          status.className = 'ingest-status error';
        })
        .finally(()=>{
          btn.disabled = false;
          btn.textContent = 'Run Ingest + Pipeline';
          btn.style.opacity = '1';
        });
    }
    function saveScoreBand() {
      var min = parseFloat(document.getElementById('score-min').value);
      var max = parseFloat(document.getElementById('score-max').value);
      if (isNaN(min) || isNaN(max) || max <= min || min < 0) {
        alert('Invalid range: min must be >= 0 and max must be > min.');
        return;
      }
      fetch('/api/settings', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({score_min: min, score_max: max})
      }).then(r=>{
        if (!r.ok) return r.text().then(t=>{ throw new Error(t); });
        var fb = document.getElementById('band-feedback');
        fb.style.display = 'block';
        setTimeout(function(){ fb.style.display = 'none'; }, 2500);
      }).catch(e=>alert('Error saving: '+e));
    }
    load();
  </script>
</body>
</html>`))
	}
}

// DashboardHandler returns dashboard stats (GET /api/dashboard).
func DashboardHandler(deps *Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		count, err := deps.DescStore.Count()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		resp := map[string]interface{}{
			"descriptions_ingested": count,
			"duplicates_skipped":    nil,
			"pipeline_phase":        "idle",
		}

		if LastIngestResult != nil {
			resp["duplicates_skipped"] = LastIngestResult.Skipped
			resp["last_ingest_added"] = LastIngestResult.Ingested
		}

		if deps.PainPointStore != nil {
			if ppCount, err := deps.PainPointStore.Count(); err == nil {
				resp["pain_points_extracted"] = ppCount
			}
		}
		if deps.ClusterStore != nil {
			if cCount, err := deps.ClusterStore.Count(); err == nil {
				resp["clusters_found"] = cCount
			}
			if briefs, err := deps.BriefsDeps.BriefStore.List(); err == nil {
				resp["briefs_generated"] = len(briefs)
			}
		}

		resp["pipeline_phase"] = pipeline.Current.Phase

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}
