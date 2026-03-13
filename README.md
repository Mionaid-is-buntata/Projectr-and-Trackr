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

```bash
make arm        # cross-compile for ARM64 (Raspberry Pi)
make deploy     # rsync to deploy target and restart systemd service
```

Target: Raspberry Pi (ARM64), managed by systemd (`infrastructure/projctr.service`). Configure deploy target in `.env`.

## Architecture

- **Backend**: Go 1.24, chi router, server-rendered inline HTML
- **Database**: SQLite (WAL mode, idempotent migrations)
- **Vector DB**: Qdrant (optional, for fuzzy dedup)
- **LLM**: Ollama (optional, for extraction and LinkedIn drafts)
- **Port**: 8090

See `CLAUDE.md` for full API reference and directory structure.
