package pipeline

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/yourname/projctr/internal/briefs"
	"github.com/yourname/projctr/internal/clustering"
	"github.com/yourname/projctr/internal/extraction"
	"github.com/yourname/projctr/internal/models"
	"github.com/yourname/projctr/internal/repository"
)

// Status tracks the current phase and outcome of the post-ingest pipeline.
type Status struct {
	Phase      string    `json:"phase"`       // idle | extracting | clustering | generating | done | error
	PainPoints int       `json:"pain_points"` // total extracted this run
	Clusters   int       `json:"clusters"`    // total clusters this run
	Briefs     int       `json:"briefs"`      // total briefs generated this run
	LastRun    time.Time `json:"last_run"`
	LastError  string    `json:"last_error,omitempty"`
}

// Current holds the live pipeline status, readable from handlers.
var Current = &Status{Phase: "idle"}

// Service runs extraction → clustering → brief generation after each ingest.
type Service struct {
	DescStore    *repository.DescriptionStore
	PainPoints   *repository.PainPointStore
	ClusterStore *repository.ClusterStore
	Extractor    *extraction.Service
	Clusterer    *clustering.Service
	BriefGen     *briefs.Generator
	BriefStore   *repository.BriefStore

	OnComplete func() // called after successful pipeline run (e.g. auto-seed projects)
	mu         sync.Mutex
	running    bool
}

// New creates a PipelineService.
func New(
	desc *repository.DescriptionStore,
	pp *repository.PainPointStore,
	clusters *repository.ClusterStore,
	extractor *extraction.Service,
	clusterer *clustering.Service,
	briefGen *briefs.Generator,
	briefStore *repository.BriefStore,
) *Service {
	return &Service{
		DescStore:    desc,
		PainPoints:   pp,
		ClusterStore: clusters,
		Extractor:    extractor,
		Clusterer:    clusterer,
		BriefGen:     briefGen,
		BriefStore:   briefStore,
	}
}

// RunPostIngest runs extraction → clustering → brief generation.
// Returns immediately if already running (prevents concurrent runs).
func (s *Service) RunPostIngest(ctx context.Context) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		log.Printf("pipeline: already running, skipping")
		return
	}
	s.running = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
	}()

	Current = &Status{Phase: "extracting", LastRun: time.Now()}

	// Phase 1: Extract pain points from unprocessed descriptions
	ppCount, err := s.extractPhase(ctx)
	if err != nil {
		log.Printf("pipeline: extraction error: %v", err)
		Current = &Status{Phase: "error", LastError: err.Error(), LastRun: time.Now()}
		return
	}
	Current.PainPoints = ppCount
	log.Printf("pipeline: extracted %d pain points", ppCount)

	// Phase 2: Cluster pain points
	Current.Phase = "clustering"
	if err := s.Clusterer.Cluster(); err != nil {
		log.Printf("pipeline: clustering error: %v", err)
		Current = &Status{Phase: "error", LastError: err.Error(), LastRun: time.Now(), PainPoints: ppCount}
		return
	}
	clusterCount, _ := s.ClusterStore.Count()
	Current.Clusters = clusterCount
	log.Printf("pipeline: %d total clusters", clusterCount)

	// Phase 3: Generate briefs for new clusters
	Current.Phase = "generating"
	briefCount, err := s.generateBriefs()
	if err != nil {
		log.Printf("pipeline: brief generation error: %v", err)
		Current = &Status{Phase: "error", LastError: err.Error(), LastRun: time.Now(), PainPoints: ppCount, Clusters: clusterCount}
		return
	}
	Current.Briefs = briefCount
	log.Printf("pipeline: generated %d new briefs", briefCount)

	Current.Phase = "done"
	Current.LastRun = time.Now()

	if s.OnComplete != nil {
		s.OnComplete()
	}
}

// extractPhase runs extraction on all unprocessed descriptions.
// Returns the number of new pain points stored.
func (s *Service) extractPhase(ctx context.Context) (int, error) {
	descs, err := s.DescStore.ListUnextracted()
	if err != nil {
		return 0, err
	}

	total := 0
	for _, desc := range descs {
		select {
		case <-ctx.Done():
			return total, ctx.Err()
		default:
		}

		points, err := s.Extractor.Extract(desc.RawText)
		if err != nil {
			log.Printf("pipeline: extract desc %d: %v", desc.ID, err)
			continue
		}

		for i := range points {
			points[i].DescriptionID = desc.ID

			ppID, err := s.PainPoints.Insert(&points[i])
			if err != nil {
				log.Printf("pipeline: insert pain point: %v", err)
				continue
			}
			total++

			// Link matched technologies
			techs := s.Extractor.ExtractTechnologies(points[i].ChallengeText)
			for _, tech := range techs {
				techID, err := s.PainPoints.InsertTechnology(&models.Technology{
					Name:     tech.Name,
					Category: tech.Category,
				})
				if err != nil {
					log.Printf("pipeline: insert technology %s: %v", tech.Name, err)
					continue
				}
				if err := s.PainPoints.LinkTechnology(ppID, techID); err != nil {
					log.Printf("pipeline: link technology: %v", err)
				}
			}
		}
	}
	return total, nil
}

// generateBriefs generates briefs for all clusters that don't have one yet.
func (s *Service) generateBriefs() (int, error) {
	newClusters, err := s.ClusterStore.ListWithoutBriefs()
	if err != nil {
		return 0, err
	}

	count := 0
	for _, cluster := range newClusters {
		brief := s.BriefGen.GenerateFromCluster(cluster)
		id, err := s.BriefStore.Insert(brief)
		if err != nil {
			log.Printf("pipeline: insert brief for cluster %d: %v", cluster.ID, err)
			continue
		}
		brief.ID = id
		final := s.BriefGen.FinalizeBriefTitle(id, brief.ProblemStatement)
		if err := s.BriefStore.SetGeneratedTitle(id, final); err != nil {
			log.Printf("pipeline: set brief title for cluster %d: %v", cluster.ID, err)
		}
		count++
	}
	return count, nil
}
