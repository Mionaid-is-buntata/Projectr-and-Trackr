# Huntr Discovery Runbook

**Version:** 2.0  
**Date:** March 2026  
**Status:** Run this before writing the ingestion or gap analysis layers

---

## Purpose

Projctr reads from one Huntr data store:

1. **Job descriptions**: JSON files in `jobs/scored/` on the NAS

The schema is not documented from Projctr's perspective — it is owned by Huntr and subject to change. This runbook verifies paths and schemas on the live Huntr instance on the Raspberry Pi.

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

## 2. Results Summary

### Jobs (JSON)

| Item | Value |
|------|-------|
| Jobs path | `/mnt/nas/huntr-data/jobs/scored/` |
| File pattern | `jobs_scored_YYYYMMDD_HHMMSS.json` |
| Score field | `score` |
| Description field | `description` |
| Title field | `title` |
| Source field | `source` |

---

## 3. Next Steps After Discovery

1. Update `config.toml` with `huntr.jobs_path`
2. Update `docs/07-embedding-model-decision.md` with model and dimensions
3. Implement `internal/huntr/client.go` (JSON reader)
4. Create `projctr_pain_points` Qdrant collection with 384 dimensions
