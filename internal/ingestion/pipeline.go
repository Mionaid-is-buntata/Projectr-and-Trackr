package ingestion

import (
	"context"
	"log"
	"time"

	"github.com/campbell/projctr/internal/clustering"
	"github.com/campbell/projctr/internal/huntr"
	"github.com/campbell/projctr/internal/models"
	"github.com/campbell/projctr/internal/repository"
	"github.com/campbell/projctr/internal/vectordb"
)

// Result holds ingestion pipeline stats.
type Result struct {
	Ingested int
	Skipped  int // ContentHash + optional FuzzyDeduplicate
}

// Pipeline runs ingestion with optional fuzzy deduplication.
type Pipeline struct {
	Reader   *huntr.JobReader
	Store    *repository.DescriptionStore
	Embedder *clustering.Embedder // optional: enables fuzzy dedup when set and Ready()
	Qdrant   *vectordb.Client     // optional: required for fuzzy dedup
}

// Run fetches sub-threshold jobs, deduplicates (ContentHash + optional FuzzyDeduplicate),
// and stores new descriptions.
func (p *Pipeline) Run(ctx context.Context) (*Result, error) {
	raw, err := p.Reader.FetchSubThreshold()
	if err != nil {
		return nil, err
	}

	useFuzzy := p.Embedder != nil && p.Embedder.Ready() && p.Qdrant != nil

	var ingested, skipped int
	var fuzzyDegraded bool // true after first fuzzy-dedup failure; suppresses repeated log lines
	now := time.Now()

	for _, r := range raw {
		hash := ContentHash(r.RawText)
		exists, err := p.Store.HasContentHash(hash)
		if err != nil {
			return nil, err
		}
		if exists {
			skipped++
			continue
		}

		// Same Huntr job (stable URL) can appear with updated text → new content_hash
		// but UNIQUE(huntr_id) forbids a second row; treat as skipped duplicate.
		hasHuntr, err := p.Store.HasHuntrID(r.HuntrID)
		if err != nil {
			return nil, err
		}
		if hasHuntr {
			skipped++
			continue
		}

		var vec []float32
		if useFuzzy && !fuzzyDegraded {
			var embedErr error
			vec, embedErr = p.Embedder.Embed(r.RawText)
			if embedErr != nil {
				log.Printf("embedding failed, disabling fuzzy dedup for this run: %v", embedErr)
				fuzzyDegraded = true
			} else {
				similar, err := p.Qdrant.IsSimilarDescription(ctx, vec)
				if err != nil {
					log.Printf("qdrant similarity check failed, disabling fuzzy dedup for this run: %v", err)
					fuzzyDegraded = true
				} else if similar {
					skipped++
					continue
				}
			}
		}

		d := &models.Description{
			HuntrID:      r.HuntrID,
			RoleTitle:    r.RoleTitle,
			Sector:       r.Sector,
			SalaryMin:    r.SalaryMin,
			SalaryMax:    r.SalaryMax,
			Location:     r.Location,
			SourceBoard:  r.SourceBoard,
			HuntrScore:   r.HuntrScore,
			RawText:      r.RawText,
			DateScraped:  time.Time{},
			DateIngested: now,
			ContentHash:  hash,
		}

		id, err := p.Store.Insert(d)
		if err != nil {
			return nil, err
		}
		ingested++

		if useFuzzy && len(vec) > 0 {
			_ = p.Qdrant.UpsertDescription(ctx, id, vec)
		}
	}

	return &Result{Ingested: ingested, Skipped: skipped}, nil
}

// Run is a convenience that runs the pipeline with minimal deps (ContentHash only).
func Run(reader *huntr.JobReader, store *repository.DescriptionStore) (*Result, error) {
	return (&Pipeline{Reader: reader, Store: store}).Run(context.Background())
}
