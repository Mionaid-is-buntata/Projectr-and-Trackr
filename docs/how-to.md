# Projctr — How-To Guide

**Audience:** The developer running this system on the local network.
**Production URL:** http://<DEPLOY_HOST>:8090
**Updated:** 2026-03-11

---

## Contents

1. [Prerequisites](#1-prerequisites)
2. [Accessing the Dashboard](#2-accessing-the-dashboard)
3. [Running Your First Ingest](#3-running-your-first-ingest)
4. [Understanding the Dashboard](#4-understanding-the-dashboard)
5. [Viewing Project Briefs](#5-viewing-project-briefs)
6. [Exporting a Brief as Markdown](#6-exporting-a-brief-as-markdown)
7. [Manually Generating a Brief](#7-manually-generating-a-brief)
8. [Monitoring Pipeline Progress](#8-monitoring-pipeline-progress)
9. [Configuration Changes](#9-configuration-changes)
10. [Local Development](#10-local-development)
11. [Deploying Updates to the Raspberry Pi](#11-deploying-updates-to-the-raspberry-pi)
12. [Troubleshooting](#12-troubleshooting)

---

## 1. Prerequisites

Before using Projctr, the following must be running:

### Infrastructure checklist

| Component | Host | Required for | How to verify |
|-----------|------|--------------|---------------|
| Projctr service | Raspberry Pi (`<DEPLOY_HOST>`) | Everything | `curl http://your-pi.local:8090/api/health` → `{"status":"ok"}` |
| NAS mount | Raspberry Pi at `/mnt/nas` | Ingest | `ssh $DEPLOY_USER@$DEPLOY_HOST "ls /mnt/nas/huntr-data/jobs/scored/"` |
| Ollama embeddings | Raspberry Pi at `localhost:11434` | Clustering, fuzzy dedup | `ssh $DEPLOY_USER@$DEPLOY_HOST "curl -s http://localhost:11434/"` |
| Ollama LLM (local) | Raspberry Pi or LLM host at `<LLM_ENDPOINT>` | Extraction (mode `"llm"` or `"both"`) | `curl -s <LLM_ENDPOINT>/` |
| **Francis** (Ollama) | `francis.local:11434` | Brief generation, "Refine with Francis" | `curl -s http://francis.local:11434/` |

> **Note:** Francis is optional. When offline, brief generation falls back to the rule-based path automatically (5-second TCP dial timeout). The "Refine with Francis" button on brief pages will show an error message if Francis is unreachable — it will not silently degrade.

### Check the systemd service on the deploy target

```bash
ssh $DEPLOY_USER@$DEPLOY_HOST "sudo systemctl status projctr"
```

If the service is stopped, start it:

```bash
ssh $DEPLOY_USER@$DEPLOY_HOST "sudo systemctl start projctr"
```

---

## 2. Accessing the Dashboard

Open a browser on any device on the local network:

```
http://your-pi.local:8090/
```

Or by IP:

```
http://<DEPLOY_HOST>:8090/
```

The dashboard loads automatically and fetches live stats and the brief list via the API. No login is required.

---

## 3. Running Your First Ingest

Ingest reads all Huntr job JSON files from the NAS, deduplicates them, stores new job descriptions, then automatically runs the full pipeline (extract pain points → cluster → generate briefs).

### Option A: Dashboard button

1. Open the dashboard at http://your-pi.local:8090/
2. Click **Run Ingest + Pipeline**
3. An alert will confirm: `Ingested: N, skipped: M. Pipeline running in background.`
4. The page stats update immediately with the new description count
5. Wait 10–60 seconds for the pipeline to complete, then refresh to see new briefs

### Option B: curl

```bash
curl -s -X POST http://your-pi.local:8090/api/ingest | python3 -m json.tool
```

Expected response:

```json
{
  "ingested": 12,
  "skipped": 3
}
```

- `ingested`: new descriptions stored this run
- `skipped`: duplicates detected (exact SHA-256 hash match or Qdrant fuzzy match above 0.95 cosine similarity)

> **If `ingested` is 0:** either all files have already been processed, or the NAS mount has no new files since the last run. The post-ingest pipeline (extraction, clustering, brief generation) only runs when `ingested > 0`.

---

## 4. Understanding the Dashboard

The stats bar at the top of the dashboard shows:

| Stat | What it means |
|------|---------------|
| **Descriptions** | Total job descriptions stored in the SQLite database across all ingest runs |
| **Pain points** | Total extracted pain points (challenges identified from job descriptions) |
| **Clusters** | Total semantic clusters of similar pain points |
| **Briefs** | Total project briefs generated from clusters |
| **Skipped (last run)** | Duplicates skipped in the most recent ingest; `—` until ingest has been run since startup |
| **Pipeline** | Current pipeline phase: `idle`, `extracting`, `clustering`, `generating`, `done`, or `error` |

The **Project Ideas** list below the stats shows all generated briefs. Each entry displays the brief title, the source role and company (if generated from a specific description), the complexity rating, and the technology stack.

---

## 5. Refining Briefs with Francis

Briefs generated without Francis (those showing `generation_source: "rules"` or `"local_llm"`) can be upgraded at any time by sending them through Francis's `mixtral:latest` model, which produces sharper problem statements, focused build plans, and better LinkedIn angles.

### From the browser

On any brief detail page (`/briefs/{id}`) or Trackr project page (`/trackr/{id}`), a **"Refine with Francis (mixtral)"** button appears when:
- Francis is configured in `config.toml` (`[trackr.llm]` section)
- The brief has not already been refined by Francis

Click the button. The page shows "Sending to Francis…" while the request is in flight (this may take 30–90 seconds for `mixtral:latest`), then "Done! Reloading…" on success. If Francis is offline, an error message appears inline and the brief is unchanged.

### Via curl

```bash
curl -s -X POST http://your-pi.local:8090/api/briefs/3/refine | python3 -m json.tool
```

Returns the updated brief JSON on success, or HTTP 503 if Francis is unreachable.

### Bulk-refining all non-Francis briefs

```bash
# Get IDs of briefs not yet refined by Francis
IDS=$(curl -s http://your-pi.local:8090/api/briefs | python3 -c "
import json, sys
briefs = json.load(sys.stdin)
print(' '.join(str(b['id']) for b in briefs if b.get('generation_source') != 'francis'))
")

for id in $IDS; do
  echo "Refining brief $id..."
  curl -s -X POST http://your-pi.local:8090/api/briefs/$id/refine > /dev/null
done
```

---

## 6. Clearing Obsolete Project Ideas

When the extraction prompts or brief generation logic changes significantly, previously generated briefs will reflect the old (weaker) approach. Clear them and re-run ingest to generate fresh ones.

### From the dashboard

1. Open `http://your-pi.local:8090/trackr/` → click **Clear Trackr** to remove all stage-tracking data (briefs are preserved)
2. Open `http://your-pi.local:8090/` → the admin panel has **Clear All Ideas** to wipe briefs, clusters, and pain points

### Via curl

```bash
# Clear all Trackr project records (stage, notes, URLs — briefs kept)
curl -s -X POST http://your-pi.local:8090/api/admin/clear-trackr

# Clear all briefs, clusters, and pain points (full reset)
curl -s -X POST http://your-pi.local:8090/api/admin/clear-ideas
```

After clearing, run ingest to regenerate everything with the current prompts:

```bash
curl -s -X POST http://your-pi.local:8090/api/ingest
```

---

## 7. Viewing Project Briefs

### From the dashboard

Click any project title in the **Project Ideas** list. This opens the brief detail page at `/briefs/{id}`.

### Brief detail page

Each brief page shows:

- **Title** and source role/company
- **Complexity** — `small`, `medium`, or `large`
- **Problem Statement** — the skill gap or challenge the project addresses
- **Suggested Approach** — high-level implementation plan
- **Technology Stack** — recommended tools and frameworks
- **Project Layout** — recommended directory structure
- **LinkedIn Angle** — suggested framing for a post about the project (if present)
- A **Download as Markdown** button at the bottom

### Via the API

List all briefs:

```bash
curl -s http://your-pi.local:8090/api/briefs | python3 -m json.tool
```

Fetch a specific brief by ID:

```bash
curl -s http://your-pi.local:8090/api/briefs/3 | python3 -m json.tool
```

---

## 8. Exporting a Brief as Markdown

### From the browser

On any brief detail page (`/briefs/{id}`), click **Download as Markdown** at the bottom of the page. The file is saved as `brief-{id}.md`.

### Via curl

```bash
curl -s http://your-pi.local:8090/api/briefs/3/export -o brief-3.md
```

The downloaded file contains: Title, Problem Statement, Suggested Approach, Technology Stack, Project Layout, and Complexity.

---

## 8. Manually Generating a Brief

The pipeline generates briefs automatically after ingest. Use the manual endpoint when you want to generate a brief from a specific cluster or description — for example, after the pipeline has run but you want to retry a specific item.

### Generate from a cluster ID

First, find a cluster ID. Use `GET /api/clusters` once that route is implemented, or query the database directly:

```bash
ssh $DEPLOY_USER@$DEPLOY_HOST "sqlite3 ~/projctr/projctr.db 'SELECT id, summary FROM clusters LIMIT 10;'"
```

Then generate:

```bash
curl -s -X POST http://your-pi.local:8090/api/briefs/generate \
  -H "Content-Type: application/json" \
  -d '{"cluster_id": 5}' | python3 -m json.tool
```

### Generate from a description ID

If clustering has not run yet but you want a brief from a single job description:

```bash
curl -s -X POST http://your-pi.local:8090/api/briefs/generate \
  -H "Content-Type: application/json" \
  -d '{"description_id": 12}' | python3 -m json.tool
```

This creates a synthetic single-item cluster and generates a brief from it.

### Generate from the first available cluster (no body required)

```bash
curl -s -X POST http://your-pi.local:8090/api/briefs/generate \
  -H "Content-Type: application/json" \
  -d '{}' | python3 -m json.tool
```

All successful responses return HTTP 201 with the created brief JSON.

---

## 9. Monitoring Pipeline Progress

After triggering an ingest that adds new descriptions, the pipeline runs asynchronously in the background. Poll the pipeline status to track progress:

```bash
curl -s http://your-pi.local:8090/api/dashboard | python3 -m json.tool
```

Watch the `pipeline_phase` field:

| Phase | Meaning |
|-------|---------|
| `idle` | No pipeline run since startup, or last run completed cleanly before a restart |
| `extracting` | Extracting pain points from unprocessed descriptions |
| `clustering` | Running DBSCAN clustering over pain point embeddings |
| `generating` | Generating briefs for new clusters that don't have one yet |
| `done` | Pipeline completed successfully this session |
| `error` | A stage failed; check server logs for the error message |

### Check server logs on the deploy target

```bash
ssh $DEPLOY_USER@$DEPLOY_HOST "sudo journalctl -u projctr -f"
```

Or view the last 100 lines:

```bash
ssh $DEPLOY_USER@$DEPLOY_HOST "sudo journalctl -u projctr -n 100"
```

The log shows per-phase counts, e.g.:
```
pipeline: extracted 47 pain points
pipeline: 12 total clusters
pipeline: generated 4 new briefs
```

---

## 10. Configuration Changes

The configuration file on the deploy target lives at `~/projctr/config.toml`. Edit it over SSH and restart the service to apply changes.

```bash
ssh $DEPLOY_USER@$DEPLOY_HOST "nano ~/projctr/config.toml"
ssh $DEPLOY_USER@$DEPLOY_HOST "sudo systemctl restart projctr"
```

### Change the score threshold

The `score_threshold` controls which Huntr jobs are considered sub-threshold (i.e., skills gaps worth analysing). Jobs with a score **below** this value are ingested.

```toml
[huntr]
score_threshold = 300   # lower = fewer but more relevant descriptions
```

Changing this only affects future ingest runs. It does not retroactively re-filter already-ingested descriptions.

### Switch extraction mode

```toml
[extraction]
mode = "rules"   # fast, no LLM required
# mode = "llm"   # uses the LLM host (<LLM_HOST>) via Ollama
# mode = "both"  # runs rules first, then supplements with LLM
```

- `rules` — keyword-based extraction using `config/tech-dict.toml`; fast, no network calls
- `llm` — uses llama3 on the LLM host via Ollama; higher quality but slower and requires the LLM host to be up
- `both` — runs rules-based pass first, then LLM pass; use for maximum coverage

> **Warning:** Changing from `rules` to `llm` or `both` requires Ollama to be running on the LLM host (<LLM_HOST>:11434). If the LLM host is unavailable, the extraction phase will error and the pipeline will not produce new pain points.

### Point LLM extraction at a different host

If the LLM host is under load or unavailable, switch extraction to the local Ollama on the deploy target:

```toml
[extraction.llm]
enabled  = true
endpoint = "http://localhost:11434"
model    = "llama3"
```

### Configure Francis for brief generation

Francis is the high-powered Ollama host used to synthesise focused project briefs and the "Refine with Francis" feature. Set the `[trackr.llm]` section in `config.toml`:

```toml
[trackr.llm]
endpoint = "http://francis.local:11434"
model    = "mixtral:latest"
```

When Francis is configured:
- New briefs generated by the pipeline will be synthesised by `mixtral:latest` (falls back to rule-based if Francis is offline at generation time)
- The "Refine with Francis (mixtral)" button appears on all brief and Trackr project pages where `generation_source != "francis"`

When `[trackr.llm]` is left blank, it inherits from `[extraction.llm]`. Set it to use a different, more capable model on a separate machine.

**Env overrides:** `TRACKR_LLM_ENDPOINT`, `TRACKR_LLM_MODEL`

---

### Adjust clustering sensitivity

```toml
[clustering]
min_cluster_size     = 3     # minimum pain points needed to form a cluster
similarity_threshold = 0.65  # cosine similarity cutoff (0–1; higher = tighter clusters)
```

Lowering `min_cluster_size` to `2` or `1` will produce more clusters from sparse data. Increasing `similarity_threshold` will merge only very similar pain points, producing more specific clusters.

---

## 11. Local Development

Development is typically done via SSH directly on the deploy target, using VS Code Remote-SSH or Cursor.

### Start the dev server with live reload

Air watches for Go source, HTML, and CSS changes and restarts the server automatically.

```bash
# On the deploy target (via SSH or remote editor terminal)
cd ~/projctr
make dev
```

`make dev` is equivalent to:

```bash
HUNTR_JOBS_PATH=./testdata/jobs/scored air
```

It uses the `testdata/` directory as the jobs source so live development does not depend on the NAS being mounted. Override this if you want to develop against real data:

```bash
HUNTR_JOBS_PATH=/mnt/nas/huntr-data/jobs/scored air
```

> **Note:** `make dev` assumes Air is installed. Install it with:
> ```bash
> go install github.com/air-verse/air@latest
> ```

### Override the jobs path without editing config.toml

The `HUNTR_JOBS_PATH` environment variable overrides `huntr.jobs_path` in config at runtime:

```bash
HUNTR_JOBS_PATH=/some/other/path ./projctr
```

### Build and run natively on the Raspberry Pi

```bash
make build   # compiles projctr binary for ARM64 (native, runs on the Raspberry Pi)
./projctr    # run the compiled binary
```

### Run tests

```bash
make test         # standard test run
make test-race    # with race detector
```

---

## 12. Deploying Updates to the Raspberry Pi

Run the following from your **workstation** (your-workstation.local), not from the Raspberry Pi.

```bash
# In the project directory on the workstation:
make deploy
```

This does the following in sequence:

1. Cross-compiles for ARM64 (`GOOS=linux GOARCH=arm64`) → produces `projctr-arm64`
2. Rsyncs the binary, templates, static assets, `config.toml`, and `config/` to `$DEPLOY_USER@$DEPLOY_HOST:~/projctr/`
3. Renames the binary to `projctr` and sets it executable
4. Restarts the systemd service: `sudo systemctl restart projctr`

To run the steps individually:

```bash
make arm     # cross-compile only; produces projctr-arm64 in the project root
```

```bash
# Manual rsync (if deploy fails partway through):
rsync -avz --exclude='.git' \
  projctr-arm64 templates/ static/ config.toml config/ \
  $DEPLOY_USER@$DEPLOY_HOST:~/projctr/

ssh $DEPLOY_USER@$DEPLOY_HOST "mv ~/projctr/projctr-arm64 ~/projctr/projctr && chmod +x ~/projctr/projctr"
ssh $DEPLOY_USER@$DEPLOY_HOST "sudo systemctl restart projctr"
```

### Verify the deployment

```bash
curl -s http://your-pi.local:8090/api/health
# {"status":"ok"}
```

```bash
ssh $DEPLOY_USER@$DEPLOY_HOST "sudo systemctl status projctr"
# Active: active (running)
```

> **Note:** `config.toml` is included in the rsync. If you have made local changes to the config on the deploy target that you do not want overwritten, either back up the remote config first or remove `config.toml` from the rsync command.

---

## 13. Troubleshooting

### No project briefs appearing after ingest

**Symptoms:** Ingest reports `ingested: N` but the dashboard shows no briefs, or the pipeline phase stays on `done` with 0 briefs.

**Steps:**

1. Check that descriptions were actually stored:
   ```bash
   curl -s http://your-pi.local:8090/api/ingest/status
   # {"descriptions_count": N}
   ```

2. Check whether pain points were extracted:
   ```bash
   curl -s http://your-pi.local:8090/api/dashboard | python3 -m json.tool
   # Look at "pain_points_extracted"
   ```

3. If `pain_points_extracted` is 0, check the extraction logs:
   ```bash
   ssh $DEPLOY_USER@$DEPLOY_HOST "sudo journalctl -u projctr -n 50"
   ```

4. The DBSCAN clustering step is currently a stub. Clusters may not be created automatically from pain points — if clustering produces 0 clusters, no briefs will be generated by the pipeline. Use the manual generate endpoint as a workaround:
   ```bash
   curl -s -X POST http://your-pi.local:8090/api/briefs/generate \
     -H "Content-Type: application/json" \
     -d '{"description_id": 1}'
   ```

---

### NAS not mounted on the deploy target

**Symptoms:** Ingest returns `ingested: 0, skipped: 0` even though new Huntr files exist, or the service fails to start.

**Verify the mount:**

```bash
ssh $DEPLOY_USER@$DEPLOY_HOST "ls /mnt/nas/huntr-data/jobs/scored/"
```

**If the mount is missing, remount manually:**

```bash
ssh $DEPLOY_USER@$DEPLOY_HOST "sudo mount -a"
```

**Check `/etc/fstab` on the deploy target** to confirm the NAS mount entry is correct. The systemd service has `RequiresMountsFor=/mnt/nas` — if the mount is absent at startup, the service will not start.

**Check NAS availability:**

```bash
ping <NAS_HOST>
```

---

### Ollama unavailable (extraction fails)

**Symptoms:** Pipeline phase goes to `error` after `extracting`, logs show connection refused to `<LLM_HOST>:11434` or `localhost:11434`.

**If using `mode = "llm"` or `mode = "both"`:**

Check Ollama on the LLM host:
```bash
curl -s http://<LLM_HOST>:11434/
```

If the LLM host is down, temporarily switch extraction to rules-only in `config.toml`:
```toml
[extraction]
mode = "rules"
```

Then restart the service and re-run ingest:
```bash
ssh $DEPLOY_USER@$DEPLOY_HOST "sudo systemctl restart projctr"
curl -s -X POST http://your-pi.local:8090/api/ingest
```

**If using Ollama for embeddings** (`embedding.endpoint = "http://localhost:11434/api/embeddings"`):

Check Ollama on the deploy target:
```bash
ssh $DEPLOY_USER@$DEPLOY_HOST "curl -s http://localhost:11434/"
```

If it is not running:
```bash
ssh $DEPLOY_USER@$DEPLOY_HOST "ollama serve &"
```

---

### Service fails to start after deploy

**Check the service status and logs:**

```bash
ssh $DEPLOY_USER@$DEPLOY_HOST "sudo systemctl status projctr"
ssh $DEPLOY_USER@$DEPLOY_HOST "sudo journalctl -u projctr -n 30"
```

**Common causes:**

- Binary not executable: `ssh $DEPLOY_USER@$DEPLOY_HOST "chmod +x ~/projctr/projctr"`
- Config parse error: verify `config.toml` is valid TOML and all paths exist
- Port already in use: `ssh $DEPLOY_USER@$DEPLOY_HOST "sudo lsof -i :8090"`
- Database path wrong: confirm `database.path` in `config.toml` is writable

---

### Ingest finds 0 new files despite new Huntr data

**Verify the jobs path:**

```bash
curl -s http://your-pi.local:8090/api/dashboard | python3 -m json.tool
```

Check that `huntr.jobs_path` in `config.toml` points at the correct NAS directory:

```bash
ssh $DEPLOY_USER@$DEPLOY_HOST "ls /mnt/nas/huntr-data/jobs/scored/ | tail -5"
```

Huntr saves files as `jobs_scored_YYYYMMDD_HHMMSS.json`. If the directory contains new files but ingest returns 0, all descriptions in those files may already be in the database (all hashes match). You can verify by querying the database:

```bash
ssh $DEPLOY_USER@$DEPLOY_HOST "sqlite3 ~/projctr/projctr.db 'SELECT COUNT(*) FROM descriptions;'"
```

---

### Dashboard shows stale data

The dashboard stats (`duplicates_skipped`, `last_ingest_added`, `pipeline_phase`) are held in memory and **reset to null/idle on service restart**. They only reflect activity since the last startup. The `descriptions_ingested`, `pain_points_extracted`, `clusters_found`, and `briefs_generated` counts come from the database and are always current.

If stats look wrong after a restart, run ingest once to re-populate the in-memory fields.
