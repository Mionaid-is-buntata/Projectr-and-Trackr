# Trackr — Structure Document

## Architectural Position

Trackr is **not a separate service**. It extends Projctr as new packages and routes within the same binary. It shares the existing SQLite database, the existing chi router, the existing Ollama LLM endpoint, and the existing `make deploy` pipeline.

```
cmd/server/main.go
    │
    ├── internal/handlers/trackr.go      (NEW — routes under /trackr/ and /api/trackr/)
    ├── internal/trackr/service.go       (NEW — state machine, EnsureProject)
    ├── internal/repository/projects.go  (NEW — ProjectStore CRUD)
    └── internal/linkedin/generator.go   (NEW — Ollama post generation)
```

All new packages follow the established patterns in Projctr (briefs, extraction). Nothing is invented from scratch.

## Data Flow

```
briefs table (Projctr)
    │  READ ONLY
    │  JOIN on brief_id
    ▼
projects table (Trackr owns)
    ├── stage transitions (service.go)
    ├── metadata updates (gitea_url, live_url, notes)
    └── linkedin_draft (generator.go writes here)
```

Trackr **never writes to `briefs`**. The `briefs` table is read-only from Trackr's perspective.

## Database Schema Changes

Three columns are added to the existing `projects` table via idempotent `ALTER TABLE` statements in `Migrate()`:

```sql
-- Already exists (from current Projctr migrations):
-- id, brief_id, stage, repository_url, linkedin_url, notes,
-- date_created, date_started, date_published

-- Added by Trackr migrations (idempotent ALTER TABLE IF NOT EXISTS):
ALTER TABLE projects ADD COLUMN gitea_url TEXT;        -- alias for repository_url, use this going forward
ALTER TABLE projects ADD COLUMN live_url TEXT;          -- deployed/demo URL
ALTER TABLE projects ADD COLUMN linkedin_draft TEXT;    -- generated post draft
ALTER TABLE projects ADD COLUMN date_parked DATETIME;   -- when project entered parked stage
```

Stage values are constrained in the Go service layer (not DB). Any existing rows with old stage values (`selected`, `complete`) are migrated to `candidate` and `published` respectively in the same migration function.

## State Machine

Defined in `internal/trackr/service.go` as a map of valid transitions:

```go
var validTransitions = map[string][]string{
    "candidate":   {"in_progress", "parked", "archived"},
    "in_progress": {"published", "parked", "candidate"},
    "parked":      {"candidate", "archived"},
    "published":   {"archived"},
    "archived":    {},
}
```

`TransitionStage(projectID int64, toStage string) error` validates against this map before writing. Returns a typed `ErrInvalidTransition` on failure — the handler maps this to HTTP 422.

## Package Responsibilities

### `internal/repository/projects.go`

`ProjectStore` — thin CRUD layer over SQLite. Follow `internal/repository/briefs.go` exactly.

Methods:
- `Insert(ctx, *models.Project) (int64, error)`
- `GetByID(ctx, id int64) (*models.Project, error)`
- `GetByBriefID(ctx, briefID int64) (*models.Project, error)`
- `List(ctx) ([]*models.ProjectWithBrief, error)` — JOIN with `briefs` for title/complexity
- `UpdateStage(ctx, id int64, stage string, timestamps map[string]*time.Time) error`
- `Update(ctx, *models.Project) error` — full mutable field update

### `internal/trackr/service.go`

`Service` — business logic layer wrapping `ProjectStore`.

Methods:
- `EnsureProject(ctx, brief *models.Brief) (*models.Project, error)` — idempotent: returns existing or creates new `candidate` row
- `TransitionStage(ctx, projectID int64, toStage string) error` — validates state machine, sets date fields
- `GetForBrief(ctx, briefID int64) (*models.Project, error)`
- `ListAll(ctx) ([]*models.ProjectWithBrief, error)`
- `UpdateMetadata(ctx, projectID int64, giteaURL, liveURL, notes string) error`
- `SaveDraft(ctx, projectID int64, draft string) error`

### `internal/linkedin/generator.go`

`Generator` — single-shot Ollama call. Pattern mirrors `internal/extraction/llm.go`.

```go
type Generator struct {
    endpoint string
    model    string
    client   *http.Client  // 30s timeout
}

func (g *Generator) Generate(b *models.Brief) (string, error)
```

Prompt template:
```
Write a professional LinkedIn post (150-250 words) announcing that I built [title].

Problem it solves: [problem_statement]
My approach: [suggested_approach]
Technologies used: [technology_stack]
Angle: [linkedin_angle]

Requirements:
- Professional but conversational tone
- First person
- 3-5 relevant hashtags at the end
- No em-dashes
```

Returns `("", ErrLLMUnavailable)` if Ollama is unreachable. Never panics or blocks indefinitely.

### `internal/handlers/trackr.go`

Registers all Trackr routes. Wired in via `RegisterTrackr(r chi.Router, deps TrackrDeps)`.

```go
type TrackrDeps struct {
    Service   *trackr.Service
    Generator *linkedin.Generator  // may be nil if LLM disabled
}
```

If `TrackrDeps` is nil when `Register` is called, the `/trackr/` routes are simply not registered. Projctr starts normally without Trackr.

## HTTP Routes

### HTML Pages

| Method | Path | Description |
|---|---|---|
| GET | `/trackr/` | Project list grouped by stage |
| GET | `/trackr/{id}` | Project detail with LinkedIn draft editor |

### API

| Method | Path | Description |
|---|---|---|
| GET | `/api/trackr/projects` | List all projects with brief title joined |
| GET | `/api/trackr/projects/{id}` | Single project detail |
| POST | `/api/trackr/projects/{id}/transition` | `{"to":"in_progress"}` — validate + apply stage transition |
| PUT | `/api/trackr/projects/{id}` | Update `gitea_url`, `live_url`, `notes` |
| POST | `/api/trackr/projects/{id}/generate-post` | Generate LinkedIn draft via Ollama; returns `{"draft":"..."}` |
| PUT | `/api/trackr/projects/{id}/post-draft` | Save edited draft text |

## Models

Add to `internal/models/models.go`:

```go
// Additions to existing Project struct:
LiveURL       string     `json:"live_url"`
LinkedInDraft string     `json:"linkedin_draft"`
DateParked    *time.Time `json:"date_parked,omitempty"`

// New type for list views (JOIN result):
type ProjectWithBrief struct {
    Project
    BriefTitle      string `json:"brief_title"`
    BriefComplexity string `json:"brief_complexity"`
}
```

## Config

Add `[trackr]` section to `Config`:

```toml
[trackr]
enabled = true

[trackr.llm]
endpoint = ""    # falls back to [extraction.llm].endpoint if blank
model    = ""    # falls back to [extraction.llm].model if blank
```

If `[trackr.llm]` is absent or empty, the generator inherits from `[extraction.llm]`. If extraction LLM is also disabled, `Generator` is nil and the generate-post route returns HTTP 503.

## Templates

Follow the existing `templates/` convention (Go `html/template`, server-rendered, no JS framework).

`templates/trackr/dashboard.html`:
- Stage-grouped list (five sections: candidate, in_progress, parked, published, archived)
- Each card: brief title, complexity badge, stage transition buttons, link to detail

`templates/trackr/detail.html`:
- Project metadata form (gitea_url, live_url, notes) with save button
- Stage transition buttons (only valid next states shown)
- LinkedIn draft section: "Generate" button → shows draft in textarea → copy-to-clipboard button

## Failure Modes

| Failure | Behaviour |
|---|---|
| Ollama unreachable | `Generate()` returns `ErrLLMUnavailable`; handler returns 503 with message "LLM unavailable — check LLM host is running" |
| Invalid stage transition | `TransitionStage` returns `ErrInvalidTransition`; handler returns 422 |
| Brief has no project row | `EnsureProject` creates one; never returns 404 for a valid brief |
| SQLite write conflict | WAL mode serialises writers; no special handling needed at single-user scale |
