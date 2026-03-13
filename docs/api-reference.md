# Projctr API Reference

**Version:** 0.1.0
**Base URL:** `http://your-host.local:8090`
**Local URL:** `http://localhost:8090`

All JSON responses use `Content-Type: application/json`. Error responses return a plain-text body with a standard HTTP status code. Datetime fields are RFC 3339 / ISO 8601 strings (e.g. `"2026-03-11T14:00:00Z"`).

---

## Contents

1. [System](#1-system)
   - [GET /api/health](#get-apihealth)
   - [GET /](#get-)
   - [GET /api/dashboard](#get-apidashboard)
   - [GET /api/pipeline/status](#get-apipelinestatus)
2. [Ingest](#2-ingest)
   - [POST /api/ingest](#post-apiingest)
   - [GET /api/ingest/status](#get-apiingeststatus)
3. [Briefs](#3-briefs)
   - [GET /api/briefs](#get-apibriefs)
   - [POST /api/briefs/generate](#post-apibriefsgенerate)
   - [GET /api/briefs/{id}](#get-apibriefs-id)
   - [GET /api/briefs/{id}/export](#get-apibriefs-idexport)
   - [GET /briefs/{id}](#get-briefs-id)
4. [Data Models](#4-data-models)
   - [Brief](#brief)
   - [Description](#description)
   - [PainPoint](#painpoint)
   - [Technology](#technology)
   - [Cluster](#cluster)
   - [Project](#project)
   - [PipelineStatus](#pipelinestatus)
5. [Planned Routes (Not Yet Implemented)](#5-planned-routes-not-yet-implemented)

---

## 1. System

### GET /api/health

Health check. Always returns `200 OK` when the server is running.

**Response `200 OK`**

```json
{ "status": "ok" }
```

**curl example**

```bash
curl http://your-host.local:8090/api/health
```

---

### GET /

Serves the HTML dashboard. The page renders client-side by fetching `/api/dashboard` for aggregate stats and `/api/briefs` for the project brief list. Includes a "Run Ingest + Pipeline" button that calls `POST /api/ingest`.

**Response `200 OK`** — `text/html; charset=utf-8`

No JSON body. The page is a self-contained HTML document with embedded JavaScript.

**curl example**

```bash
curl http://your-host.local:8090/
```

---

### GET /api/dashboard

Returns aggregate statistics for the dashboard UI. The fields `duplicates_skipped` and `last_ingest_added` are `null` until `POST /api/ingest` has been called at least once since server startup (they are held in memory only, not persisted).

**Response `200 OK`**

```json
{
  "descriptions_ingested": 142,
  "duplicates_skipped": 8,
  "last_ingest_added": 5,
  "pain_points_extracted": 317,
  "clusters_found": 24,
  "briefs_generated": 19,
  "pipeline_phase": "idle"
}
```

**Response fields**

| Field | Type | Description |
|---|---|---|
| `descriptions_ingested` | `integer` | Total job descriptions currently in the database |
| `duplicates_skipped` | `integer \| null` | Duplicates skipped during the last ingest run; `null` until first run |
| `last_ingest_added` | `integer \| null` | New descriptions added during the last ingest run; `null` until first run |
| `pain_points_extracted` | `integer` | Total pain points in the database |
| `clusters_found` | `integer` | Total clusters in the database |
| `briefs_generated` | `integer` | Total briefs in the database |
| `pipeline_phase` | `string` | Current or last pipeline phase (see [PipelineStatus](#pipelinestatus)) |

**Error codes**

| Code | Condition |
|---|---|
| `500` | Database read failure |

**curl example**

```bash
curl http://your-host.local:8090/api/dashboard
```

---

### GET /api/pipeline/status

Returns the live status of the post-ingest pipeline (extraction → clustering → brief generation). The pipeline runs asynchronously in the background after a successful `POST /api/ingest` that added at least one new description.

**Response `200 OK`**

```json
{
  "phase": "done",
  "pain_points": 12,
  "clusters": 4,
  "briefs": 3,
  "last_run": "2026-03-11T14:23:01Z",
  "last_error": ""
}
```

**Response fields**

| Field | Type | Description |
|---|---|---|
| `phase` | `string` | `idle \| extracting \| clustering \| generating \| done \| error` |
| `pain_points` | `integer` | Pain points extracted in the current/last run |
| `clusters` | `integer` | Clusters produced in the current/last run |
| `briefs` | `integer` | Briefs generated in the current/last run |
| `last_run` | `datetime` | Timestamp of when the last pipeline run started |
| `last_error` | `string` | Error message if `phase` is `"error"`, otherwise omitted or empty |

**curl example**

```bash
curl http://your-host.local:8090/api/pipeline/status
```

---

## 2. Ingest

### POST /api/ingest

Runs the ingestion pipeline. Reads sub-threshold Huntr job JSON files from the path configured in `huntr.jobs_path`, deduplicates using SHA-256 content hash, and — when the embedding service is available — also performs fuzzy deduplication using Qdrant cosine-similarity (threshold: 0.95). New descriptions are persisted to SQLite.

If at least one new description is ingested, the full post-ingest pipeline (extraction → clustering → brief generation) is triggered **asynchronously** in the background. The endpoint returns immediately without waiting for the pipeline to complete. Monitor pipeline progress via `GET /api/pipeline/status`.

**Request body**

No request body required.

**Response `200 OK`**

```json
{
  "ingested": 5,
  "skipped": 3
}
```

**Response fields**

| Field | Type | Description |
|---|---|---|
| `ingested` | `integer` | New descriptions stored this run |
| `skipped` | `integer` | Entries skipped due to exact or fuzzy deduplication |

**Error codes**

| Code | Condition |
|---|---|
| `405` | Request method is not `POST` |
| `500` | Ingestion pipeline failure (file read error, database error, etc.) |

**curl example**

```bash
curl -X POST http://your-host.local:8090/api/ingest
```

---

### GET /api/ingest/status

Returns the total count of job descriptions currently stored in the database. Unlike `GET /api/dashboard`, this endpoint does not require the ingest pipeline to have been run since startup.

**Response `200 OK`**

```json
{
  "descriptions_count": 142
}
```

**Response fields**

| Field | Type | Description |
|---|---|---|
| `descriptions_count` | `integer` | Total descriptions in the database |

**Error codes**

| Code | Condition |
|---|---|
| `500` | Database read failure |

**curl example**

```bash
curl http://your-host.local:8090/api/ingest/status
```

---

## 3. Briefs

### GET /api/briefs

Returns all project briefs in the database. Returns an empty JSON array `[]` when no briefs exist yet.

**Response `200 OK`** — array of [Brief](#brief) objects

```json
[
  {
    "id": 1,
    "cluster_id": 3,
    "source_company": "Acme Corp",
    "source_role": "Senior Backend Engineer",
    "title": "Observability Sidecar for Legacy Services",
    "problem_statement": "Teams operating legacy services lack lightweight tooling to attach OpenTelemetry-compatible observability without full rewrites.",
    "suggested_approach": "Build a sidecar process that wraps existing services, intercepts HTTP and gRPC traffic, and emits spans to a configured OTLP endpoint.",
    "technology_stack": "[\"Go\",\"OpenTelemetry\",\"Prometheus\",\"Docker\"]",
    "project_layout": "observability-sidecar/\n  cmd/sidecar/\n  internal/proxy/\n  internal/telemetry/\n  Dockerfile\n  README.md",
    "complexity": "medium",
    "impact_score": 0.82,
    "linkedin_angle": "How I added distributed tracing to a 10-year-old service in a weekend",
    "is_edited": false,
    "date_generated": "2026-03-11T14:00:00Z",
    "date_modified": null
  }
]
```

**Error codes**

| Code | Condition |
|---|---|
| `500` | Database read failure |

**curl example**

```bash
curl http://your-host.local:8090/api/briefs
```

---

### POST /api/briefs/generate

Generates a project brief from a cluster or a single description and persists it. The request body determines the source:

- **`cluster_id` provided** — generate from the identified cluster.
- **`description_id` provided** — create a synthetic single-member cluster from the description, then generate a brief. The brief's `source_company` and `source_role` are populated from the description's `Sector` and `RoleTitle` fields respectively.
- **Neither provided** — use the first available cluster. Returns `400` if no clusters exist.

**Request body**

```json
{
  "cluster_id": 7
}
```

or

```json
{
  "description_id": 42
}
```

or (use first available cluster)

```json
{}
```

**Request fields**

| Field | Type | Required | Description |
|---|---|---|---|
| `cluster_id` | `integer \| null` | No | ID of an existing cluster to generate from |
| `description_id` | `integer \| null` | No | ID of a single description; creates a synthetic cluster automatically |

**Response `201 Created`** — the created [Brief](#brief) object

```json
{
  "id": 12,
  "cluster_id": 7,
  "source_company": "",
  "source_role": "",
  "title": "Real-Time Anomaly Detection Pipeline",
  "problem_statement": "Data-heavy applications lack accessible tooling for detecting statistical anomalies in streaming data without expensive managed services.",
  "suggested_approach": "Implement a configurable stream processor using a sliding-window Z-score algorithm, backed by Redis for state and exposing a webhook for alert delivery.",
  "technology_stack": "[\"Go\",\"Redis\",\"Kafka\",\"Docker\"]",
  "project_layout": "anomaly-detector/\n  cmd/processor/\n  internal/window/\n  internal/alert/\n  docker-compose.yml",
  "complexity": "large",
  "impact_score": null,
  "linkedin_angle": "",
  "is_edited": false,
  "date_generated": "2026-03-11T15:30:00Z",
  "date_modified": null
}
```

**Error codes**

| Code | Condition |
|---|---|
| `400` | Malformed JSON body |
| `400` | Neither `cluster_id` nor `description_id` provided and no clusters exist in the database |
| `404` | Specified `cluster_id` does not exist |
| `404` | Specified `description_id` does not exist |
| `500` | Database write failure |

**curl examples**

Generate from a cluster:
```bash
curl -X POST http://your-host.local:8090/api/briefs/generate \
  -H "Content-Type: application/json" \
  -d '{"cluster_id": 7}'
```

Generate from a single description:
```bash
curl -X POST http://your-host.local:8090/api/briefs/generate \
  -H "Content-Type: application/json" \
  -d '{"description_id": 42}'
```

Use first available cluster:
```bash
curl -X POST http://your-host.local:8090/api/briefs/generate \
  -H "Content-Type: application/json" \
  -d '{}'
```

---

### GET /api/briefs/{id}

Returns a single brief by its integer ID.

**Path parameters**

| Parameter | Type | Description |
|---|---|---|
| `id` | `integer` | Brief primary key |

**Response `200 OK`** — [Brief](#brief) object

```json
{
  "id": 1,
  "cluster_id": 3,
  "source_company": "Acme Corp",
  "source_role": "Senior Backend Engineer",
  "title": "Observability Sidecar for Legacy Services",
  "problem_statement": "...",
  "suggested_approach": "...",
  "technology_stack": "[\"Go\",\"OpenTelemetry\",\"Prometheus\"]",
  "project_layout": "...",
  "complexity": "medium",
  "impact_score": 0.82,
  "linkedin_angle": "...",
  "is_edited": false,
  "date_generated": "2026-03-11T14:00:00Z",
  "date_modified": null
}
```

**Error codes**

| Code | Condition |
|---|---|
| `400` | `id` is not a valid integer |
| `404` | No brief with the given ID exists |
| `500` | Database read failure |

**curl example**

```bash
curl http://your-host.local:8090/api/briefs/1
```

---

### GET /api/briefs/{id}/export

Downloads a brief as a Markdown file. The response body contains a formatted Markdown document with sections for Problem Statement, Suggested Approach, Technology Stack, Project Layout, and Complexity. If the brief has a source role and company, these appear as a bold subheading beneath the title.

**Path parameters**

| Parameter | Type | Description |
|---|---|---|
| `id` | `integer` | Brief primary key |

**Response `200 OK`**

```
Content-Type: text/markdown
Content-Disposition: attachment; filename=brief-{id}.md
```

**Response body example**

```markdown
# Observability Sidecar for Legacy Services

**Senior Backend Engineer — Acme Corp**

## Problem Statement

Teams operating legacy services lack lightweight tooling...

## Suggested Approach

Build a sidecar process that wraps existing services...

## Technology Stack

["Go","OpenTelemetry","Prometheus","Docker"]

## Project Layout

observability-sidecar/
  cmd/sidecar/
  internal/proxy/
  ...

## Complexity

medium
```

**Error codes**

| Code | Condition |
|---|---|
| `400` | `id` is not a valid integer |
| `404` | No brief with the given ID exists |
| `500` | Database read failure |

**curl example**

```bash
curl -O -J http://your-host.local:8090/api/briefs/1/export
```

---

### GET /briefs/{id}

Serves an HTML detail page for a brief. Displays all brief fields in a structured layout with a "Download as Markdown" link that points to `GET /api/briefs/{id}/export`.

**Path parameters**

| Parameter | Type | Description |
|---|---|---|
| `id` | `integer` | Brief primary key |

**Response `200 OK`** — `text/html; charset=utf-8`

The HTML page includes: title, source role and company (if present), complexity, problem statement, suggested approach, technology stack, project layout, and LinkedIn angle (if present).

**Error codes**

| Code | Condition |
|---|---|
| `400` | `id` is not a valid integer |
| `404` | No brief with the given ID exists |
| `500` | Database read failure |

**curl example**

```bash
curl http://your-host.local:8090/briefs/1
```

---

## 4. Data Models

### Brief

A generated project brief derived from a pain point cluster.

| Field | JSON key | Type | Description |
|---|---|---|---|
| ID | `id` | `int64` | Primary key |
| ClusterID | `cluster_id` | `int64` | ID of the source cluster |
| SourceCompany | `source_company` | `string` | Company name from the originating Huntr job; empty for cluster-derived briefs |
| SourceRole | `source_role` | `string` | Job title from the originating Huntr job; empty for cluster-derived briefs |
| Title | `title` | `string` | Project title |
| ProblemStatement | `problem_statement` | `string` | Description of the problem the project addresses |
| SuggestedApproach | `suggested_approach` | `string` | High-level implementation strategy |
| TechnologyStack | `technology_stack` | `string` | Recommended technologies serialised as a JSON array string (e.g. `"[\"Go\",\"Redis\"]"`) |
| ProjectLayout | `project_layout` | `string` | Recommended directory structure in Markdown format |
| Complexity | `complexity` | `"small" \| "medium" \| "large"` | Estimated project size |
| ImpactScore | `impact_score` | `float64 \| null` | Computed impact/priority score; `null` if not yet calculated |
| LinkedInAngle | `linkedin_angle` | `string` | Suggested LinkedIn post framing; may be empty |
| IsEdited | `is_edited` | `boolean` | `true` if the brief has been manually edited after generation |
| DateGenerated | `date_generated` | `datetime` | ISO 8601 timestamp of when the brief was generated |
| DateModified | `date_modified` | `datetime \| null` | ISO 8601 timestamp of last manual edit; `null` if never edited |

---

### Description

A job description ingested from a Huntr scored JSON file. Not currently exposed via a dedicated API endpoint (see [Planned Routes](#5-planned-routes-not-yet-implemented)).

| Field | Type | Description |
|---|---|---|
| `ID` | `int64` | Primary key |
| `HuntrID` | `string` | Huntr job link, used as the unique identifier for the source record |
| `RoleTitle` | `string` | Job title |
| `Sector` | `string` | Industry sector or company name (Huntr's mapping) |
| `SalaryMin` | `int \| null` | Minimum salary in the job posting |
| `SalaryMax` | `int \| null` | Maximum salary in the job posting |
| `Location` | `string` | Job location |
| `SourceBoard` | `string` | Origin job board (e.g. LinkedIn, Indeed) |
| `HuntrScore` | `float64` | Huntr relevance/fit score |
| `RawText` | `string` | Full raw job description text |
| `DateScraped` | `datetime` | When Huntr scraped the listing |
| `DateIngested` | `datetime` | When Projctr ingested the description |
| `ContentHash` | `string` | SHA-256 hash of the normalised description text, used as the exact-dedup key |

---

### PainPoint

A structured pain point extracted from a Description by the extraction pipeline. Not currently exposed via a dedicated API endpoint.

| Field | Type | Description |
|---|---|---|
| `ID` | `int64` | Primary key |
| `DescriptionID` | `int64` | Foreign key to the source Description |
| `ChallengeText` | `string` | The extracted pain point or challenge statement |
| `Domain` | `string` | Thematic domain (e.g. observability, data pipeline, auth) |
| `OutcomeText` | `string` | Desired outcome or success criterion extracted from context |
| `Confidence` | `float64` | Extraction confidence score (0.0–1.0) |
| `QdrantPointID` | `string` | Qdrant vector ID for this pain point; empty when Qdrant is not configured |
| `DateExtracted` | `datetime` | When the pain point was extracted |

---

### Technology

A normalised technology keyword matched during extraction. Linked to PainPoints via a join table.

| Field | Type | Description |
|---|---|---|
| `ID` | `int64` | Primary key |
| `Name` | `string` | Normalised technology name (e.g. `"PostgreSQL"`, `"Kubernetes"`) |
| `Category` | `string` | `language \| framework \| platform \| tool \| database \| methodology` |

---

### Cluster

A group of semantically similar PainPoints identified by the clustering pipeline. Not currently exposed via a dedicated list API endpoint.

| Field | Type | Description |
|---|---|---|
| `ID` | `int64` | Primary key |
| `Summary` | `string` | Human-readable description of the cluster theme |
| `Frequency` | `int` | Number of pain points in this cluster |
| `AvgSalary` | `float64 \| null` | Average salary across the source descriptions in the cluster |
| `RecencyScore` | `float64 \| null` | Score reflecting how recently this pattern has appeared in job listings |
| `GapType` | `string` | `skill_extension \| skill_acquisition \| domain_expansion \| mixed` |
| `GapScore` | `float64 \| null` | Composite gap-fit score for portfolio prioritisation |
| `DateClustered` | `datetime` | When the cluster was created |

---

### Project

Tracks a Brief through the portfolio pipeline stages. Not currently exposed via the API (see [Planned Routes](#5-planned-routes-not-yet-implemented)).

| Field | Type | Description |
|---|---|---|
| `ID` | `int64` | Primary key |
| `BriefID` | `int64` | Foreign key to the source Brief |
| `Stage` | `string` | `candidate \| selected \| in_progress \| complete \| published \| archived` |
| `RepositoryURL` | `string` | Gitea repository URL |
| `LinkedInURL` | `string` | URL of the published LinkedIn post |
| `Notes` | `string` | Free-form notes |
| `DateCreated` | `datetime` | |
| `DateSelected` | `datetime \| null` | When the project was promoted from `candidate` |
| `DateStarted` | `datetime \| null` | When work began |
| `DateCompleted` | `datetime \| null` | When the project was completed |
| `DatePublished` | `datetime \| null` | When the LinkedIn post went live |

---

### PipelineStatus

Returned by `GET /api/pipeline/status`. Reflects the in-memory state of the post-ingest background pipeline.

| Field | JSON key | Type | Description |
|---|---|---|---|
| Phase | `phase` | `string` | `idle \| extracting \| clustering \| generating \| done \| error` |
| PainPoints | `pain_points` | `integer` | Pain points extracted in the current or last run |
| Clusters | `clusters` | `integer` | Clusters produced in the current or last run |
| Briefs | `briefs` | `integer` | Briefs generated in the current or last run |
| LastRun | `last_run` | `datetime` | Timestamp of when the last run started |
| LastError | `last_error` | `string` | Error message when `phase` is `"error"`; omitted otherwise |

**Phase lifecycle:**

```
idle → extracting → clustering → generating → done
                                            ↘ error (any phase can transition to error)
```

The server starts with `phase: "idle"`. After `POST /api/ingest` adds new descriptions, the pipeline transitions through the phases in sequence. On server restart, state resets to `idle`.

---

## 5. Planned Routes (Not Yet Implemented)

The following routes are defined in the project roadmap but have no registered handler yet. All will return a `404` or no response until implemented.

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/descriptions` | List all ingested job descriptions |
| `GET` | `/api/descriptions/{id}` | Get a single description by ID |
| `GET` | `/api/pain-points` | List all extracted pain points |
| `POST` | `/api/extract` | Trigger pain point extraction manually |
| `GET` | `/api/clusters` | List all pain point clusters |
| `GET` | `/api/clusters/{id}` | Get a single cluster by ID |
| `POST` | `/api/cluster` | Trigger clustering algorithm manually |
| `POST` | `/api/clusters/{id}/merge` | Merge two clusters |
| `PUT` | `/api/briefs/{id}` | Manually edit an existing brief |
| `GET` | `/api/projects` | List all portfolio pipeline projects |
| `PATCH` | `/api/projects/{id}` | Update a project's stage |
| `GET` | `/api/settings` | Retrieve application configuration |
| `PUT` | `/api/settings` | Update application configuration |
