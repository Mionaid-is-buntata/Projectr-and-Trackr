# From Rejection Signal to Portfolio Project: How I Built Projctr

---

## The Holiday Doom-Scroll

I was sitting at Blu, Ghadira Bay by the sea, phone/tablet open — because apparently that's who I am on holiday — scrolling through the codebase for Huntr. For anyone who hasn't cloned it: Huntr parses your CV, scrapes job boards, and scores each role against your profile. It's genuinely useful. It turns the endless spray-and-pray of job applications into something you can actually navigate.

But I kept thinking about the low-scoring vacancies. The roles I'd clearly never get, where the gap was a handful of specific technologies. "Not enough experience with Kubernetes." "No background in event-driven architectures." "Limited exposure to time-series databases." And I found myself asking: why does *this* company need that? What problem are they trying to solve that requires it?

That's when the framing shifted. A job description isn't a pass/fail test. It's a company telling you, in public, what they're currently struggling with. The delta between the CV and that description isn't a rejection — it's a brief. If they need Kubernetes experience, Then build something that runs on Kubernetes. Show up to the interview having already tried to solve their problem. Yes?

---

## The Real Problem With Job Hunting

CVs are static snapshots of who you were six months ago. Job descriptions are live signals from the market about what companies need right now. Most candidates read a job description as a checklist to pass. I started seeing them as specs to build from.

Huntr does the heavy lifting of surfacing the gap — it scores each role and flags the ones where you don't match well. But it stops there. A low score just tells you "gap exists." It doesn't tell you what to build. With enough sub-threshold descriptions, you can see the patterns: the same technologies, the same platforms, the same capability clusters appearing across companies you'd actually want to work for. That pattern is a curriculum. I just needed something to extract it.

---

## Introducing Projctr

Projctr reads Huntr's sub-threshold job descriptions and turns them into actionable portfolio project briefs.

```
Huntr JSON files
     ↓
Ingestion (deduplicate job descriptions)
     ↓
Extraction (identify technical pain points)
     ↓
Clustering (group similar pain points)
     ↓
Brief Generation (produce project specs)
     ↓
Dashboard (browse + export briefs)
```

Each stage is independent. The pipeline runs to completion even if optional services are unavailable. You always get output.

---

## How It Works

### Stage 1: Ingestion

Huntr stores scored job descriptions as JSON files. Projctr reads only the sub-threshold ones — those below a configurable score cutoff — so you're always working with the meaningful gaps, not the full-match roles.

Before anything else, duplicates get filtered. The same role often appears across multiple job boards with slight variations in wording. Projctr deduplicates in two tiers: exact (SHA-256 hash of the description content) then fuzzy (vector similarity via Qdrant, using all-MiniLM-L6-v2 embeddings). If the vector service is unavailable, it falls back to exact-only. The pipeline always continues.

### Stage 2: Pain Point Extraction

Each job description gets scanned for technical requirements that fall outside the CV. The default mode uses a rules-based extractor: it splits the description into sentences, looks for trigger phrases ("experience with", "must have", "proficiency in"), and matches against a dictionary of 46 technologies across languages, frameworks, platforms, and tools. The dictionary is a plain TOML file — extend it without touching code.

There's an optional LLM mode backed by Ollama (running Gemma2:9B on a separate machine) that extracts semantically richer pain points — things the rules extractor would miss because they're phrased unusually or described in terms of business outcomes rather than technology names.

Every pain point that comes out of this stage has a confidence score, a domain label (language, framework, platform, infrastructure…), and an outcome phrase: "to enable real-time processing", "to support CI/CD workflows". The outcome phrase is what makes a brief feel purposeful rather than just a tech checklist.

### Stage 3: Semantic Clustering

With hundreds of job descriptions, you accumulate hundreds of overlapping pain points. Clustering surfaces the signal — the things the market keeps asking for.

Pain points are embedded using the same all-MiniLM-L6-v2 model, then grouped by cosine distance using DBSCAN. Because Projctr uses the same embedding model as Huntr, the semantic space is aligned: similar concepts land near each other, even when phrased differently across job descriptions. If embeddings are unavailable, pain points are grouped by domain instead.

Each cluster gets a gap type. *Skill extension* means the gap is adjacent to something you already know — you could cover it in a weekend project. *Skill acquisition* means a genuinely new domain. *Domain expansion* means applying your existing skills in a context you haven't worked in. This shapes what the brief asks you to build.

### Stage 4: Brief Generation

One brief per cluster. Each brief includes:

- **A project title** derived from the cluster's most representative pain point
- **A suggested technology stack** matched to the gap type — not just "use Kubernetes", but why and in what kind of project
- **A complexity rating** (small / medium / large) based on how frequently the cluster appeared across descriptions — if twenty companies all need the same thing, that's a large project worth the investment
- **A Markdown project scaffold** with suggested structure, so you start with a skeleton rather than a blank file
- **A LinkedIn angle** — a one-liner written for when you post the finished project. The brief closes the loop back to the platform that generated it.

---

## The Stack

Go 1.24. Single binary, no runtime dependencies beyond what you choose to enable. SQLite in WAL mode for storage — no database server to manage, no infrastructure overhead.

It runs on a Raspberry Pi5, deployed via rsync and systemd. Cross-compiled for ARM64 from a Fedora workstation with `make deploy`. Embeddings and optional LLM extraction run on a second machine via Ollama. Qdrant handles vector similarity search when it's available — but the app degrades gracefully without it at every stage.

The HTTP interface is a chi router and a minimal dashboard served from the same binary. No separate frontend build step.

---

## Why This Matters

The job description is the syllabus. Employers write down exactly what they need, in public, every time they post a role. Projctr just treats that document seriously.

A few design choices made this work:

- **Hybrid extraction** — rules-based by default, LLM as an enhancement, never a hard requirement
- **Graceful degradation at every stage** — the pipeline always produces output, regardless of which optional services are running
- **Same embedding model as Huntr** — semantic alignment between the two apps out of the box, without any extra coordination
- **Human-editable tech dictionary** — a plain TOML file you extend without touching code

---

## What's Next

The full loop is live: ingest → extract → cluster → brief → export to Markdown. Next up is a project portfolio tracker to move briefs through stages (candidate → in progress → published), and a LinkedIn post generator that drafts the "I built this" post directly from the brief itself — closing the loop from job description back to the platform where companies are watching.

Most people respond to a job description with their CV. Some people respond with a project they built *because* of it.

Which one do you think gets the call?

---

*Interested in the approach or building something similar? Drop a comment or connect*
