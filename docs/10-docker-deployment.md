# Docker Deployment for Local Testing

Run Projctr with Qdrant in Docker for local testing.

## Quick Start

```bash
# Build and start
docker compose up -d

# If the API is not reachable on the host (curl fails) but the container is "Up",
# port publishing can be stuck — recreate the stack:
# docker compose down && docker compose up -d

# Verify
curl http://localhost:8090/api/health
# From another machine: http://<host-ip>:8090/
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

## ARM64 hosts (16K page size) and Qdrant

On some ARM64 Linux kernels (e.g. Raspberry Pi 5, Apple Silicon VMs), the page size is **16 KiB** instead of 4 KiB. The official **linux/arm64** Qdrant image can then crash in a loop with:

```text
<jemalloc>: Unsupported system page size
Aborted (core dumped)
```

`docker-compose.yml` pins Qdrant to **`platform: linux/amd64`** so the x86_64 binary (4K pages under emulation) runs instead.

On Linux (unlike Docker Desktop), you must install **QEMU user-mode + binfmt** once so amd64 containers can start:

```bash
docker run --privileged --rm tonistiigi/binfmt:latest --install all
```

After that, `docker compose up -d` should show Qdrant **Up** and logs like `Qdrant HTTP listening on 6333`.

> **Reboots:** binfmt entries may need to be re-applied after some OS upgrades; run the same command again if Qdrant fails with `exec format error` on the amd64 image.

## Docker `data-root` on the right disk (Linux)

Image layers, build cache, and container metadata all live under Docker’s **data root** (default: `/var/lib/docker` on the root filesystem). If `/etc/docker/daemon.json` sets `"data-root"` to a **small or nearly full disk** (e.g. a dedicated SSD mounted at `/mnt/docker-data`), builds can fail with `no space left on device` while the **main OS disk** still has free space.

**Recommendation:** keep `data-root` on the **larger** filesystem (usually the same volume as `/`, e.g. `/var/lib/docker` on `/dev/sdb…`), not on a tiny auxiliary drive used only for legacy experiments.

1. Stop Docker: `sudo systemctl stop docker` (and `docker.socket` if present).
2. Copy the existing tree:  
   `sudo rsync -aH /old/path/docker/ /var/lib/docker/`  
   (or merge into an existing `/var/lib/docker` after a rename backup.)
3. Point the daemon at the default location, e.g. `/etc/docker/daemon.json`:
   ```json
   {}
   ```
   (Empty object = use default `/var/lib/docker`.)
4. Start Docker: `sudo systemctl start docker`, then `docker compose up -d` in the project directory.

After verifying containers and volumes, you can remove the old `data-root` directory to reclaim space on the small disk.
