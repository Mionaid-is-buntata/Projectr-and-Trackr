# Embedding Model Decision Record

**Version:** 2.0  
**Date:** March 2026  
**Status:** Confirmed — Huntr uses sentence-transformers/all-MiniLM-L6-v2

---

## Decision

Projctr will use the **same embedding model as Huntr**.

This is not optional. Gap analysis works by computing cosine similarity between Projctr's pain point vectors and Huntr's CV vectors. If the two sets of vectors are produced by different models, the similarity scores are meaningless. Model compatibility is a hard architectural constraint.

**Confirmed:**
- Model: `sentence-transformers/all-MiniLM-L6-v2`
- Vector dimensions: 384
- Huntr stores CV embeddings in ChromaDB (not Qdrant)

---

## Why Model Reuse Is the Right Approach

Huntr already embeds the user's CV using a local model. That embedding represents the semantic space against which portfolio gaps are measured. For the comparison to be valid, pain point embeddings must exist in the same semantic space — which means the same model, same tokenisation, same normalisation.

Using a different model — even one with identical dimensions — would produce vectors in a different space. The similarity scores would be numerically valid but semantically incoherent. A cluster might score 0.8 similarity to the CV not because the skills genuinely overlap, but because two unrelated models happened to place their outputs nearby in their respective spaces.

---

## Confirmed Values

| Field | Value |
|-------|-------|
| Model name | `sentence-transformers/all-MiniLM-L6-v2` |
| Vector dimensions | 384 |
| Distance metric | Cosine (ChromaDB default) |
| Huntr CV store | ChromaDB (Docker volume on Raspberry Pi) |
| Projctr pain points | Qdrant `projctr_pain_points` (384 dimensions) |

---

## Impact on Qdrant Collection

Projctr's `projctr_pain_points` collection must use 384 dimensions to match Huntr's CV embeddings.

```toml
[qdrant]
vector_dimensions = 384   # Match sentence-transformers/all-MiniLM-L6-v2

[embedding]
model    = "sentence-transformers/all-MiniLM-L6-v2"
endpoint = ""   # Fill in: Ollama or embedding service
```

---

## Embedding Call Pattern

Once the endpoint is known, the embedding call will follow this shape. Shown here for Ollama — adjust if Huntr uses a different serving mechanism:

```go
// internal/clustering/embeddings.go

type embeddingRequest struct {
    Model  string `json:"model"`
    Prompt string `json:"prompt"`
}

type embeddingResponse struct {
    Embedding []float32 `json:"embedding"`
}

func (s *Service) embed(ctx context.Context, text string) ([]float32, error) {
    body, _ := json.Marshal(embeddingRequest{
        Model:  s.cfg.Embedding.Model,
        Prompt: text,
    })
    resp, err := s.http.PostContext(ctx, s.cfg.Embedding.Endpoint, "application/json", bytes.NewReader(body))
    // decode resp into embeddingResponse...
    return decoded.Embedding, err
}
```

The endpoint URL and model name come from config, so the implementation does not change when the exact values are confirmed — only `config.toml` is updated.

---

## Fallback When the Embedding Endpoint Is Unavailable

If the embedding service is temporarily down (e.g., Ollama stopped, the host machine is off):

- Ingestion still works — descriptions are stored in SQLite
- Extraction still works — pain points are stored in SQLite without Qdrant vectors
- Clustering and gap analysis are unavailable until the endpoint recovers
- Pain points with `qdrant_point_id = NULL` in SQLite are the pending queue
- A manual re-embedding trigger in the UI processes the backlog once the endpoint recovers

---

## Pre-Development Checklist

- [ ] Run `06-huntr-discovery-runbook.md` §2.4 to identify model name and endpoint
- [ ] Confirm the endpoint is reachable from where Projctr will run
- [ ] Confirm vector dimensions match the Huntr CV collection
- [ ] Set `vector_dimensions` in `config.toml`
- [ ] Set `huntr_cv_collection` in `config.toml`
- [ ] Set `embedding.model` and `embedding.endpoint` in `config.toml`
- [ ] Update the decision table in this document with confirmed values
