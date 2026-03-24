# Projctr + Trackr

Analyses sub-threshold job descriptions from Huntr to extract, cluster, and
prioritise portfolio project ideas. **Trackr** adds a portfolio stage tracker
and LinkedIn post generator.

## Quick Start

```bash
cp config.example.toml config.toml   # edit paths and endpoints
cp .env.example .env                 # set personal values
go mod tidy
make dev                             # live reload via Air
```

Open `http://localhost:8090` — the main dashboard lists briefs and links to Trackr.

## Features

- **Ingestion** — reads Huntr JSON job files, deduplicates (exact + fuzzy)
- **Extraction** — rules-based (+ optional LLM) pain point extraction
- **Clustering** — embeds pain points, groups with DBSCAN
- **Brief generation** — creates project briefs with tech stack and layout
- **Trackr dashboard** — moves projects through candidate / in_progress / parked / published / archived
- **LinkedIn drafts** — generates post drafts via Ollama, save and copy
- **Markdown export** — download any project as `.md`
- **Atom feed** — `GET /trackr/feed.xml` for published projects

## Development

```bash
make dev        # live reload (Air)
make build      # native compile
make test       # run tests
make lint       # golangci-lint
```

## Deployment

Run these **from your development machine** (full clone of this repo), not from `/opt/projctr` on the Pi:

```bash
make arm        # cross-compile for ARM64 (Raspberry Pi)
make deploy     # rsync to deploy target and restart systemd service
```

Target: Raspberry Pi (ARM64), managed by systemd (`infrastructure/projctr.service`). Configure `DEPLOY_USER`, `DEPLOY_HOST`, and `DEPLOY_DIR` in `.env`.

On the Pi, `/opt/projctr` only contains the binary, config, and docs — use `make help` there for `restart` / `status` / `logs` after a workstation deploy.

**If you use Docker Compose on the Pi** (`docker-compose.yml` in this repo), port **8090** is served by the **container**, not by `projctr.service`. `make deploy` updates `/opt/projctr/projctr` only; you must **rebuild the image** or ingest will still hit an old binary:

```bash
# On the Pi, from this repository directory
git pull
docker compose build --no-cache projctr && docker compose up -d projctr
```

Use **either** systemd **or** Docker for Projctr on one host — not both (they contend for 8090).

## Architecture

- **Backend**: Go 1.24, chi router, server-rendered inline HTML
- **Database**: SQLite (WAL mode, idempotent migrations)
- **Vector DB**: Qdrant (optional, for fuzzy dedup)
- **LLM**: Ollama (optional, for extraction and LinkedIn drafts)
- **Port**: 8090

See `CLAUDE.md` for full API reference and directory structure.
