# Trackr — Product Document

## Problem

Projctr surfaces project ideas from job market data and generates detailed briefs. The gap is that nothing tracks what happens next. Briefs accumulate with no signal about which ones are being built, which have shipped, and which were quietly shelved. There is no record of outcomes and no mechanism to announce shipped work to the professional audience that generated the demand in the first place.

## Solution

Trackr extends Projctr with two capabilities:

1. **Portfolio Stage Tracker** — move briefs through an explicit lifecycle from idea to published project, with metadata (repo URL, live URL, dates, notes) attached at each stage.

2. **LinkedIn Post Generator** — draft a professional "I built this" announcement directly from the brief content, closing the loop from job description back to the platform where the companies are watching.

## Users

Single developer (the system owner). No multi-user, authentication, or access control requirements in v1.

## Success Criteria

- Every Projctr brief is visible in Trackr as a `candidate` project with zero manual import required
- A brief can be moved through a stage transition in under 10 seconds
- A LinkedIn post draft can be generated from a brief and copied to clipboard in under 30 seconds
- Trackr does not break or slow down any existing Projctr functionality
- The system runs on a Raspberry Pi (ARM64) within the existing systemd service

## Lifecycle Stages

```
candidate → in_progress → published → archived
              ↕
            parked → archived
```

| Stage | Meaning |
|---|---|
| `candidate` | Brief has been reviewed; worth considering |
| `in_progress` | Actively building this project |
| `parked` | Deprioritised; not abandoned, not active |
| `published` | Project is live / shipped |
| `archived` | Closed out; no further action |

## Out of Scope (v1)

- Drag-and-drop Kanban UI (stage transitions via buttons only)
- Gitea App or OAuth integration (repo URL is a manually entered string)
- Posting to LinkedIn via API (post text is drafted and copied manually)
- Analytics, reporting, or dashboards beyond a stage-grouped list
- Email or push notifications
- Multi-user support or authentication

## Open Questions (v1 deferred)

- Should `published` projects surface on a public-facing page? (Phase 3 candidate)
- Should ingest automatically seed `candidate` records for new briefs, or only on first dashboard load? (Likely: on dashboard load for simplicity)
