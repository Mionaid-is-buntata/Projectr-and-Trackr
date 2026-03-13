# Trackr — Task List

> Before starting any task, read `docs/trackr/product.md`, `docs/trackr/structure.md`, and `docs/trackr/tech.md`.
> Switch to a feature branch before touching code: `git checkout -b feature/trackr-<slug>`

---

## Phase 1 — Stage Tracker

No LLM. No config changes. Pure CRUD + state machine + HTML dashboard.

### Task 1.1 — Extend data model and run migrations

**Branch:** `feature/trackr-model`

Steps:
1. In `internal/models/models.go` — add `LiveURL string`, `LinkedInDraft string`, `DateParked *time.Time` to the `Project` struct. Add new `ProjectWithBrief` type (embeds `Project`, adds `BriefTitle string`, `BriefComplexity string`). Tighten the stage comment to the canonical set: `candidate | in_progress | parked | published | archived`.
2. In `internal/database/migrations.go` — add idempotent `ALTER TABLE projects ADD COLUMN` for `gitea_url TEXT`, `live_url TEXT`, `linkedin_draft TEXT`, `date_parked DATETIME`. Add data migration: `UPDATE projects SET stage = 'candidate' WHERE stage = 'selected'` and `UPDATE projects SET stage = 'published' WHERE stage = 'complete'`.
3. Build and run locally. Verify migrations apply cleanly against an existing DB with `make dev`.

**Done when:** `make build` succeeds; DB schema has the four new columns; no existing data is corrupted.

---

### Task 1.2 — ProjectStore (repository layer)

**Branch:** `feature/trackr-repository`

Steps:
1. Create `internal/repository/projects.go`. Copy structure from `internal/repository/briefs.go`.
2. Implement: `Insert`, `GetByID`, `GetByBriefID`, `List` (JOIN with `briefs` for `ProjectWithBrief`), `UpdateStage`, `Update`.
3. `List` query: `SELECT p.*, b.title AS brief_title, b.complexity AS brief_complexity FROM projects p JOIN briefs b ON p.brief_id = b.id ORDER BY p.date_created DESC`.
4. `UpdateStage` takes `id int64`, `stage string`, and a `timestamps` map to set whichever date field corresponds to the new stage.

**Done when:** Store compiles; methods return correct types with null fields handled.

---

### Task 1.3 — State machine service

**Branch:** `feature/trackr-service`

Steps:
1. Create `internal/trackr/service.go`.
2. Define `validTransitions` map (see `docs/trackr/structure.md`).
3. Implement `TransitionStage` — validate transition, compute which date field to set (`date_started` for `in_progress`, `date_published` for `published`, `date_parked` for `parked`), call `store.UpdateStage`.
4. Implement `EnsureProject(ctx, brief *models.Brief) (*models.Project, error)` — call `GetByBriefID`; if not found, call `Insert` with `stage: "candidate"`, `date_created: time.Now()`.
5. Implement remaining methods: `ListAll`, `GetForBrief`, `UpdateMetadata`, `SaveDraft`.

**Done when:** State machine rejects invalid transitions with `ErrInvalidTransition`; `EnsureProject` is idempotent.

---

### Task 1.4 — HTTP handlers and route registration

**Branch:** `feature/trackr-handlers`

Steps:
1. Create `internal/handlers/trackr.go`.
2. Define `TrackrDeps` struct with `Service *trackr.Service` and `Generator *linkedin.Generator` (may be nil).
3. Implement all API routes from `docs/trackr/structure.md` (skip `generate-post` and `post-draft` — those are Phase 2).
4. `POST /api/trackr/projects/{id}/transition` — decode `{"to":"<stage>"}`, call `service.TransitionStage`, return 200 with updated project JSON or 422 on `ErrInvalidTransition`.
5. Add `RegisterTrackr(r chi.Router, deps TrackrDeps)` function.
6. In `internal/handlers/routes.go` — add `TrackrDeps *TrackrDeps` field to `Dependencies`; add `if deps.TrackrDeps != nil { RegisterTrackr(r, *deps.TrackrDeps) }` in `Register`.
7. In `cmd/server/main.go` — instantiate `ProjectStore`, `trackr.Service`, wire into `Dependencies.TrackrDeps`.

**Done when:** `GET /api/trackr/projects` returns JSON list; stage transition endpoint validates correctly.

---

### Task 1.5 — HTML dashboard and detail templates

**Branch:** `feature/trackr-templates`

Steps:
1. Create `templates/trackr/dashboard.html` — server-rendered list grouped by stage. Five stage sections (skip empty ones). Each card: brief title, complexity badge, current stage, buttons for valid next transitions only, link to detail page.
2. Create `templates/trackr/detail.html` — project metadata form (gitea_url, live_url, notes) with save button via `PUT /api/trackr/projects/{id}`. Stage transition buttons. Placeholder for LinkedIn draft section (Phase 2 will populate this).
3. Add handlers `GET /trackr/` and `GET /trackr/{id}` in `trackr.go`. On `GET /trackr/`, call `service.ListAll` after seeding any unseen briefs via a batch `EnsureProject` loop.
4. Add `/trackr/` link to the main dashboard template (`templates/index.html`).

**Done when:** Dashboard loads in browser; all briefs appear as candidates; stage buttons work; detail page saves metadata.

---

## Phase 2 — LinkedIn Post Generator

Requires Phase 1 complete. Requires Ollama accessible on the LLM host.

### Task 2.1 — LinkedIn generator package

**Branch:** `feature/trackr-linkedin-generator`

Steps:
1. Create `internal/linkedin/generator.go`. Copy HTTP call pattern from `internal/extraction/llm.go`.
2. Implement `Generator` struct (endpoint, model, 30s `http.Client`).
3. Implement `Generate(b *models.Brief) (string, error)` — construct prompt from `product.md` spec, call `/api/generate`, return response string or `ErrLLMUnavailable`.
4. In `internal/config/config.go` — add `TrackrConfig` and `TrackrLLMConfig` structs; add `Trackr TrackrConfig` field to main `Config`.
5. In `cmd/server/main.go` — resolve LLM endpoint/model with fallback to `[extraction.llm]`; instantiate `linkedin.Generator` only if LLM is enabled; set `TrackrDeps.Generator`.
6. Add `[trackr]` section to `config.toml`.

**Done when:** `Generator.Generate()` returns a non-empty draft when Ollama is running; returns `ErrLLMUnavailable` when Ollama is not reachable (test by pointing at a bad endpoint).

---

### Task 2.2 — Generate and save draft routes

**Branch:** `feature/trackr-linkedin-routes`

Steps:
1. In `internal/handlers/trackr.go` — implement `POST /api/trackr/projects/{id}/generate-post`. Fetch project and its brief, call `deps.Generator.Generate(brief)`, store result via `service.SaveDraft`, return `{"draft":"..."}`. If `deps.Generator` is nil, return 503 immediately.
2. Implement `PUT /api/trackr/projects/{id}/post-draft` — decode `{"draft":"..."}`, call `service.SaveDraft`.
3. Update `templates/trackr/detail.html` — add LinkedIn draft section: "Generate post" button (calls generate-post API), textarea showing existing or newly generated draft, "Copy to clipboard" button (JS `navigator.clipboard.writeText`), save button (calls post-draft API).

**Done when:** Generate button produces a draft; draft persists across page reloads; copy button works.

---

## Phase 3 — Polish (deferred, no timeline)

These tasks are defined for future reference. Do not start before Phase 2 is complete.

| Task | Description |
|---|---|
| 3.1 Markdown export | `GET /trackr/{id}/export` — download project + LinkedIn draft as `.md`. Mirror `exportBriefHandler`. |
| 3.2 Published index | Read-only public page listing `published` projects with Gitea and live URLs. |
| 3.3 Auto-seed on ingest | After ingestion pipeline completes, call `EnsureProject` for all new briefs (hook into `ingest.Run`). |
| 3.4 RSS feed | `GET /trackr/feed.xml` — Atom feed of published projects for external readers. |

---

## Status Tracking

Update this table after each task completes.

| Task | Status | Branch | Notes |
|---|---|---|---|
| 1.1 Model + migrations | `done` | `feature/trackr-model` | Project struct extended; 4 ALTER TABLE columns added; stage values migrated |
| 1.2 ProjectStore | `done` | `feature/trackr-model` | Insert, GetByID, GetByBriefID, List (JOIN), UpdateStage, Update |
| 1.3 Service + state machine | `done` | `feature/trackr-model` | State machine validates transitions; EnsureProject idempotent; all methods implemented |
| 1.4 HTTP handlers | `done` | `feature/trackr-model` | API routes registered; wired into routes.go and main.go |
| 1.5 HTML templates | `done` | `feature/trackr-model` | Dashboard with stage-grouped list; detail page with metadata form + transitions; Trackr link on main dashboard |
| 2.1 LinkedIn generator | `done` | `feature/trackr-model` | Generator + config structs + LLM fallback logic + wired in main.go |
| 2.2 Generate/save routes | `done` | `feature/trackr-model` | generate-post + post-draft routes; detail page UI with generate/save/copy buttons |
| 3.1 Markdown export | `done` | `feature/trackr-phase3` | GET /trackr/{id}/export — downloads project + brief + LinkedIn draft as .md |
| 3.2 Published index | `skipped` | — | Not required per user |
| 3.3 Auto-seed on ingest | `done` | `feature/trackr-phase3` | Pipeline OnComplete callback seeds projects for all briefs |
| 3.4 RSS feed | `done` | `feature/trackr-phase3` | GET /trackr/feed.xml — Atom feed of published projects |
