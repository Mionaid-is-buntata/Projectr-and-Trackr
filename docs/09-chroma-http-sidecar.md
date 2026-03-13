# Chroma HTTP Sidecar for Projctr

**Purpose:** Projctr needs HTTP access to Huntr's ChromaDB for gap analysis. Huntr runs Chroma embedded; this runbook adds a Chroma HTTP sidecar so Projctr can read CV embeddings.

---

## Option A: Add to Huntr Docker Compose (Recommended)

Add a Chroma HTTP container to Huntr's stack that mounts the same ChromaDB volume.

### 1. Locate Huntr docker-compose

On the Raspberry Pi: `/home/your-user/Huntr-AI/docker-compose.yml` (or equivalent).

### 2. Add Chroma HTTP Service

```yaml
services:
  # ... existing huntr-web, huntr-processor, huntr-scraper ...

  chroma-http:
    image: chromadb/chroma:latest
    container_name: huntr-chroma-http
    volumes:
      - app_chromadb-data:/chroma_data
    command: ["chromadb", "run", "--path", "/chroma_data", "--host", "0.0.0.0", "--port", "8000"]
    ports:
      - "8000:8000"
    restart: unless-stopped
    networks:
      - app_default  # or whatever Huntr uses
```

### 3. Verify Volume Name

Huntr uses `app_chromadb-data`. Ensure the sidecar mounts the same volume:

```bash
docker volume ls | grep chromadb
```

### 4. Projctr Config

In Projctr's `config.toml`:

```toml
[chromadb]
url = "http://localhost:8000"   # When Projctr runs on the same host
# Or if Projctr runs in Docker on same network:
# url = "http://huntr-chroma-http:8000"
collection = "cv_20260126_223955"
```

---

## Option B: Run Chroma HTTP Standalone

If modifying Huntr's compose is not preferred:

```bash
# On the Raspberry Pi — mount the Docker volume path
docker run -d --name chroma-http \
  -v /var/lib/docker/volumes/app_chromadb-data/_data:/chroma_data \
  -p 8000:8000 \
  chromadb/chroma:latest \
  chromadb run --path /chroma_data --host 0.0.0.0 --port 8000
```

---

## Verification

```bash
curl http://localhost:8000/api/v2/heartbeat
```

Expected: `{"nanosecond heartbeat": ...}`

---

## Notes

- Chroma HTTP and Huntr's embedded Chroma both access the same SQLite file. Chroma supports multiple readers; avoid writes from both simultaneously.
- If Huntr is writing (e.g. during CV processing), Projctr's reads may briefly block. Gap analysis is typically run on a schedule or on-demand when Huntr is idle.
