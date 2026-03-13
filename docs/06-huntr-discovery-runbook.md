# Huntr Discovery Runbook

**Version:** 2.0  
**Date:** March 2026  
**Status:** Run this before writing the ingestion or gap analysis layers

---

## Purpose

Projctr reads from two Huntr data stores:

1. **Job descriptions**: JSON files in `jobs/scored/` on the NAS
2. **CV embeddings**: ChromaDB (embedded in Huntr Docker containers)

Neither schema is documented from Projctr's perspective — they are owned by Huntr and subject to change. This runbook verifies paths, schemas, and embedding model details on the live Huntr instance on the Raspberry Pi.

---

## 0. Prerequisites — NAS Mount

Projctr runs on the Raspberry Pi. Verify the NAS is mounted:

```bash
ssh $DEPLOY_USER@your-pi.local  # or $DEPLOY_HOST

# Check mount
mount | grep huntr-data
ls -la /mnt/nas/huntr-data/
```

Record:

- NAS mount path: **`/mnt/nas/huntr-data`**
- Accessible as user `your-user`: **`________________`**

---

## 1. Huntr Jobs — JSON Discovery

### 1.1 Locate Scored Jobs

```bash
ls -la /mnt/nas/huntr-data/jobs/scored/
ls -t /mnt/nas/huntr-data/jobs/scored/*.json | head -5
```

Record:

- Jobs path: **`/mnt/nas/huntr-data/jobs/scored/`**
- File pattern: **`jobs_scored_YYYYMMDD_HHMMSS.json`**
- Latest file: **`________________`**

### 1.2 Inspect JSON Schema

```bash
# Get latest file and pretty-print first job
LATEST=$(ls -t /mnt/nas/huntr-data/jobs/scored/*.json | head -1)
cat "$LATEST" | python3 -c "import json,sys; d=json.load(sys.stdin); print(json.dumps(d[0] if isinstance(d,list) else d, indent=2))" | head -80
```

### 1.3 Confirmed JSON Schema

| Field | Type | Notes |
|-------|------|-------|
| `title` | string | Job title |
| `company` | string | |
| `location` | string | |
| `work_type` | string | Remote, Hybrid, etc. |
| `salary` | string | Human-readable |
| `salary_num` | number | Numeric (pence or pounds) |
| `description` | string | Raw job description |
| `responsibilities` | string | |
| `skills` | string | Comma-separated |
| `benefits` | string | |
| `link` | string | Job URL |
| `source` | string | Reed, LinkedIn, etc. |
| `score` | number | Huntr match score |
| `score_breakdown` | object | tech_stack_score, domain_score, etc. |

### 1.4 Sub-Threshold Filter

Projctr ingests jobs with `score < 300` (configurable). Count:

```bash
LATEST=$(ls -t /mnt/nas/huntr-data/jobs/scored/*.json | head -1)
cat "$LATEST" | python3 -c "
import json,sys
jobs=json.load(sys.stdin)
sub= [j for j in jobs if j.get('score',999) < 300]
print(f'Total: {len(jobs)}, Sub-300: {len(sub)}')
"
```

Record: **Total `____` / Sub-300 `____`**

---

## 2. Huntr ChromaDB — Discovery

ChromaDB stores CV embeddings. On the Raspberry Pi, Huntr runs in Docker; ChromaDB lives in a Docker volume.

### 2.1 Locate ChromaDB

```bash
# Huntr Docker volume (Raspberry Pi)
docker inspect huntr-web 2>/dev/null | grep -A5 Mounts
ls -la /var/lib/docker/volumes/app_chromadb-data/_data/
```

Record:

- ChromaDB path: **`/var/lib/docker/volumes/app_chromadb-data/_data`**
- ChromaDB file: **`chroma.sqlite3`**
- Size: **`________________`**

Note: Projctr needs read access. If running as a non-root user, add the user to the `docker` group or use a bind mount.

### 2.2 Huntr Config — Collection and Embedding Model

```bash
cat /mnt/nas/huntr-data/config/config.json | python3 -c "
import json,sys
c=json.load(sys.stdin)
print('CV collection:', c.get('cv',{}).get('vector_db',{}).get('active_collection'))
print('Embedding model:', c.get('embeddings',{}).get('model'))
print('Vector DB path (in container):', c.get('cv',{}).get('chunked_processing',{}).get('vector_db_path'))
"
```

Record:

- Active collection: **`cv_20260126_223955`**
- Embedding model: **`sentence-transformers/all-MiniLM-L6-v2`**
- Vector dimensions: **`384`** (all-MiniLM-L6-v2)

### 2.3 Verify Projctr Can Use the Same Model

Gap analysis requires Projctr's pain point vectors to be in the same semantic space as Huntr's CV vectors. Confirm:

- Projctr must use **sentence-transformers/all-MiniLM-L6-v2** (or equivalent)
- Dimensions: **384**
- Embedding endpoint: Huntr may use local Python (sentence-transformers); Projctr needs an HTTP endpoint (Ollama, etc.) or must run the same model locally

---

## 3. Results Summary

### Jobs (JSON)

| Item | Value |
|------|-------|
| Jobs path | `/mnt/nas/huntr-data/jobs/scored/` |
| File pattern | `jobs_scored_YYYYMMDD_HHMMSS.json` |
| Score field | `score` |
| Description field | `description` |
| Title field | `title` |
| Source field | `source` |

### ChromaDB

| Item | Value |
|------|-------|
| ChromaDB path | `/var/lib/docker/volumes/app_chromadb-data/_data` |
| Collection name | `cv_20260126_223955` |
| Vector dimensions | 384 |
| Embedding model | `sentence-transformers/all-MiniLM-L6-v2` |

---

## 4. Next Steps After Discovery

1. Update `config.toml` with `huntr.jobs_path` and `chromadb.url`
2. Update `docs/07-embedding-model-decision.md` with model and dimensions
3. Implement `internal/huntr/client.go` (JSON reader)
4. Add Chroma HTTP sidecar — see **`docs/09-chroma-http-sidecar.md`**
5. Create `projctr_pain_points` Qdrant collection with 384 dimensions
