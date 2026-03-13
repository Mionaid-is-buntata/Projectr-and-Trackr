# Trackr — Tech Document

## Stack

All inherited from Projctr. Nothing new is introduced.

| Concern | Technology |
|---|---|
| Language | Go 1.24 |
| HTTP router | `github.com/go-chi/chi/v5` |
| Database | SQLite via `modernc.org/sqlite` (WAL mode) |
| LLM | Ollama on LLM host — same endpoint as extraction |
| Config | TOML via `github.com/BurntSushi/toml` |
| Templates | Go `html/template` (server-rendered) |
| Deployment | ARM64 cross-compile → rsync → systemd on Raspberry Pi |

## File Locations

### New files

```
internal/trackr/service.go
internal/linkedin/generator.go
internal/repository/projects.go
internal/handlers/trackr.go
templates/trackr/dashboard.html
templates/trackr/detail.html
```

### Modified files

```
internal/models/models.go          — extend Project struct; add ProjectWithBrief
internal/database/migrations.go    — add ALTER TABLE columns; migrate old stage values
internal/config/config.go          — add TrackrConfig + TrackrLLMConfig structs
internal/handlers/routes.go        — add TrackrDeps field; call RegisterTrackr if non-nil
cmd/server/main.go                 — wire ProjectStore, trackr.Service, linkedin.Generator
config.toml                        — add [trackr] and [trackr.llm] sections
```

## Build Commands

```bash
# Standard local build (unchanged)
make build

# Cross-compile for ARM64 Raspberry Pi
make arm

# Live reload for development
make dev

# Deploy to Raspberry Pi
make deploy
```

No new Makefile targets are required.

## Config Additions

Add to `config.toml`:

```toml
[trackr]
enabled = true

[trackr.llm]
# Leave blank to inherit from [extraction.llm]
endpoint = ""
model    = ""
```

Add corresponding structs in `internal/config/config.go`:

```go
type TrackrConfig struct {
    Enabled bool            `toml:"enabled"`
    LLM     TrackrLLMConfig `toml:"llm"`
}

type TrackrLLMConfig struct {
    Endpoint string `toml:"endpoint"`
    Model    string `toml:"model"`
}
```

LLM fallback logic in `main.go`:

```go
llmEndpoint := cfg.Trackr.LLM.Endpoint
if llmEndpoint == "" {
    llmEndpoint = cfg.Extraction.LLM.Endpoint
}
llmModel := cfg.Trackr.LLM.Model
if llmModel == "" {
    llmModel = cfg.Extraction.LLM.Model
}
```

## Database Migration Conventions

All migrations in `internal/database/migrations.go` use `IF NOT EXISTS` or equivalent to be idempotent. For columns, use SQLite's `ALTER TABLE ... ADD COLUMN` which is a no-op if the column already exists in recent SQLite versions — but wrap in a helper that checks `PRAGMA table_info` first since `modernc.org/sqlite` may vary.

Pattern to follow (already used in codebase):

```go
func addColumnIfNotExists(db *sql.DB, table, column, definition string) error {
    // query PRAGMA table_info(table), check column names, ALTER TABLE if absent
}
```

Data migration for old stage values (run once, idempotent via UPDATE WHERE):

```sql
UPDATE projects SET stage = 'candidate'  WHERE stage = 'selected';
UPDATE projects SET stage = 'published'  WHERE stage = 'complete';
```

## Patterns to Follow

| New component | Copy pattern from |
|---|---|
| `internal/repository/projects.go` | `internal/repository/briefs.go` |
| `internal/linkedin/generator.go` | `internal/extraction/llm.go` |
| `internal/handlers/trackr.go` | `internal/handlers/briefs.go` |
| `templates/trackr/dashboard.html` | `templates/index.html` |
| `templates/trackr/detail.html` | `templates/brief_detail.html` |

## Ollama Call Shape

The Ollama endpoint and request/response structs are already defined in `internal/extraction/llm.go`. When implementing `internal/linkedin/generator.go`, extract the shared HTTP call logic if duplication becomes significant — otherwise copy the pattern directly (two files is not a violation worth abstracting in v1).

Endpoint: `POST http://<host>/api/generate`

```json
{
  "model": "llama3.2",
  "prompt": "...",
  "stream": false
}
```

Response field: `response` (string).

## Error Types

Define in each package (no shared errors package needed in v1):

```go
// internal/trackr/service.go
var ErrInvalidTransition = errors.New("invalid stage transition")

// internal/linkedin/generator.go
var ErrLLMUnavailable = errors.New("LLM unavailable")
```

Handler mapping:

```go
switch {
case errors.Is(err, trackr.ErrInvalidTransition):
    http.Error(w, err.Error(), http.StatusUnprocessableEntity)  // 422
case errors.Is(err, linkedin.ErrLLMUnavailable):
    http.Error(w, "LLM unavailable — check LLM host is running", http.StatusServiceUnavailable)  // 503
}
```

## Testing Notes

No tests exist in Projctr yet. Trackr does not need to introduce tests — but if any are added, the state machine transition logic in `service.go` is the highest-value unit to test (pure function, no DB dependency).

## Deployment Notes

Trackr ships inside the same `projctr` binary. No changes to `projctr.service`, the rsync target, or the systemd configuration are required.

The `projects` table already exists on the deploy target's SQLite DB (it was created by Projctr's existing migrations). The new `ALTER TABLE` statements in the updated migration will run on first startup after deploy and add the three new columns.

Deploy sequence (unchanged):

```bash
make arm      # cross-compile to ./bin/projctr-arm64
make deploy   # rsync binary + config, SSH restart systemd
```
