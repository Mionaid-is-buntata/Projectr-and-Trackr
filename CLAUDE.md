# CLAUDE — Projctr + Trackr

## Project Overview

Projctr analyses sub-threshold job descriptions from Huntr to extract, cluster, and prioritise portfolio project ideas. **Trackr** extends Projctr with a portfolio stage tracker and LinkedIn post generator.

**Go module:** `github.com/yourname/projctr` | **Go version:** 1.24.0

---

## Architecture

### Data Flow
```
Huntr JSON → Ingestion → SQLite → Extraction → Clustering → Brief Generation → Trackr (stage tracking + LinkedIn drafts)
```

### Directory Structure
```
cmd/server/main.go              # Entry point & bootstrap
config/
  tech-dict.toml                # Technology keyword dictionary
internal/
  config/                       # TOML config loading + env overrides
  database/                     # SQLite (WAL mode) + idempotent migrations
  models/                       # Domain entities (Description, PainPoint, Cluster, Brief, Project, ProjectWithBrief)
  repository/                   # CRUD stores (descriptions, briefs, clusters, projects, settings)
  huntr/                        # Huntr JSON job file reader
  ingestion/                    # Dedup pipeline (SHA-256 + optional fuzzy via Qdrant)
  extraction/                   # Pain point extraction (rules + optional LLM via Ollama)
  clustering/                   # Embedding (all-MiniLM-L6-v2, 384d) + DBSCAN
  briefs/                       # Brief generation with complexity-based layouts
  pipeline/                     # Post-ingest orchestration (extract -> cluster -> generate -> auto-seed)
  trackr/                       # Portfolio stage machine + business logic
  linkedin/                     # LinkedIn post generation via Ollama
  vectordb/                     # Qdrant client (optional, graceful degradation)
  handlers/                     # chi HTTP handlers & routing (inline HTML, no templates)
infrastructure/
  projctr.service               # systemd unit for Raspberry Pi deploy target
docs/
  reference/                    # Trackr design docs (product, structure, tech, tasks)
```

### Key Dependencies
- `github.com/go-chi/chi/v5` — HTTP router
- `modernc.org/sqlite` — Pure Go SQLite driver
- `github.com/qdrant/go-client` — Vector DB (optional)
- `github.com/BurntSushi/toml` — Config parsing

---

## Configuration

**Files:** `config.toml` (gitignored) | `config.example.toml` (committed)

Key sections: `[server]`, `[database]`, `[huntr]`, `[qdrant]`, `[embedding]`, `[extraction]`, `[extraction.llm]`, `[clustering]`, `[trackr]`, `[trackr.llm]`

`[trackr.llm]` falls back to `[extraction.llm]` when blank.

**Env overrides:** `CONFIG_PATH`, `HUNTR_JOBS_PATH`, `DATABASE_PATH`, `QDRANT_HOST`, `QDRANT_PORT`, `EMBEDDING_ENDPOINT`, `LLM_ENDPOINT`

---

## HTTP API

### Core
- `GET /` — Dashboard (HTML)
- `GET /api/health` — Health check
- `POST /api/ingest` — Run ingestion pipeline
- `GET /api/ingest/status` — Ingest stats
- `GET /api/dashboard` — Stats JSON
- `GET /api/pipeline/status` — Pipeline phase
- `GET /api/settings` / `POST /api/settings` — Score band (min/max)
- `GET /docs/how-to` / `GET /docs/api` / `GET /docs/technical` — Documentation pages

### Briefs
- `GET /api/briefs` — List briefs
- `GET /api/briefs/{id}` — Get brief
- `GET /api/briefs/{id}/export` — Download as Markdown
- `GET /briefs/{id}` — Brief detail page (HTML)
- `POST /api/briefs/generate` — Generate brief

### Trackr
- `GET /trackr/` — Portfolio dashboard (HTML, seeds projects from briefs)
- `GET /trackr/{id}` — Project detail page (HTML)
- `GET /trackr/{id}/export` — Download project as Markdown
- `GET /trackr/feed.xml` — Atom feed of published projects
- `GET /api/trackr/projects` — List projects JSON
- `GET /api/trackr/projects/{id}` — Get project JSON
- `POST /api/trackr/projects/{id}/transition` — Stage transition (`{"to":"stage"}`)
- `PUT /api/trackr/projects/{id}` — Update metadata (gitea_url, live_url, notes)
- `POST /api/trackr/projects/{id}/generate-post` — Generate LinkedIn draft via Ollama
- `PUT /api/trackr/projects/{id}/post-draft` — Save LinkedIn draft

### Stage Machine
```
candidate -> in_progress | parked | archived
in_progress -> published | parked | candidate
parked -> candidate | archived
published -> archived
archived -> (terminal)
```

Invalid transitions return HTTP 422 with `ErrInvalidTransition`.

---

## Build & Deployment

```bash
make build       # Native compile
make dev         # Live reload with Air
make arm         # Cross-compile for ARM64 (Raspberry Pi)
make deploy      # rsync + SSH to deploy target, restart systemd
```

**Production:** systemd service (`projctr.service`) on Raspberry Pi (see `.env` for deploy target).
**Gitea:** Self-hosted Gitea instance (see `.env` for URL).

---

## Database

Tables: `descriptions`, `pain_points`, `technologies`, `pain_point_technologies`, `clusters`, `cluster_members`, `briefs`, `projects`, `settings`

- Migrations run idempotently on startup (ALTER TABLE errors ignored for existing columns)
- WAL mode for concurrent reads
- Deduplication: exact (SHA-256) + fuzzy (Qdrant cosine > 0.95)
