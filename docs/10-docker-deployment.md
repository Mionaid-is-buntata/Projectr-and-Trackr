# Docker Deployment for Local Testing

Run Projctr with Qdrant in Docker for local testing.

## Quick Start

```bash
# Build and start
docker compose up -d

# Verify
curl http://localhost:8090/api/health
curl http://localhost:8090/api/dashboard

# Ingest from test data (mounts ./testdata/jobs/scored)
curl -X POST http://localhost:8090/api/ingest

# Generate a brief from a description (after ingest)
curl -X POST http://localhost:8090/api/briefs/generate \
  -H "Content-Type: application/json" \
  -d '{"description_id": 1}'
```

## Services

| Service  | Port | Description                    |
|----------|------|--------------------------------|
| projctr  | 8090 | Projctr API and dashboard      |
| qdrant   | 6333 (REST), 6334 (gRPC) | Vector DB for fuzzy dedup |

## Volumes

- **projctr_data** — SQLite DB and data
- **qdrant_data** — Qdrant storage
- **./testdata/jobs/scored** — Mounted read-only for job ingestion

## Configuration

Uses `config.docker.toml` with:

- `huntr.jobs_path` = `/data/jobs/scored` (overridable via `HUNTR_JOBS_PATH`)
- `qdrant.host` = `qdrant` (Docker service name)
- `database.path` = `/data/projctr.db`

## Test Data

Sample jobs in `testdata/jobs/scored/jobs_scored_20260301_120000.json` — 2 sub-threshold jobs (score 150, 200).

## Stop

```bash
docker compose down
```
