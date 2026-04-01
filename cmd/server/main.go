package main

import (
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/campbell/projctr/internal/briefs"
	"github.com/campbell/projctr/internal/clustering"
	"github.com/campbell/projctr/internal/config"
	"github.com/campbell/projctr/internal/database"
	"github.com/campbell/projctr/internal/extraction"
	"github.com/campbell/projctr/internal/handlers"
	"github.com/campbell/projctr/internal/huntr"
	"github.com/campbell/projctr/internal/linkedin"
	"github.com/campbell/projctr/internal/ingestion"
	"github.com/campbell/projctr/internal/pipeline"
	"github.com/campbell/projctr/internal/repository"
	"github.com/campbell/projctr/internal/trackr"
	"github.com/campbell/projctr/internal/vectordb"
)

func main() {
	cfgPath := os.Getenv("CONFIG_PATH")
	if cfgPath == "" {
		cfgPath = "config.toml"
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	db, err := database.Open(cfg.Database.Path)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer db.Close()

	if err := database.Migrate(db); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	// Repositories
	descStore := repository.NewDescriptionStore(db)
	painPointStore := repository.NewPainPointStore(db)
	clusterStore := repository.NewClusterStore(db)
	briefStore := repository.NewBriefStore(db)
	settingsStore := repository.NewSettingsStore(db)

	// Load score range from DB (falls back to config.toml defaults if not yet persisted)
	scoreMin := settingsStore.GetFloat("score_min", cfg.Huntr.ScoreMin)
	scoreMax := settingsStore.GetFloat("score_max", cfg.Huntr.ScoreMax)

	// Huntr reader + embedder
	jobReader := huntr.NewJobReader(cfg.Huntr.JobsPath, scoreMin, scoreMax)
	embedder := clustering.NewEmbedder(cfg.Embedding.Model, cfg.Embedding.Endpoint)

	// Optional Qdrant client (fuzzy dedup)
	var qdrantClient *vectordb.Client
	if qc, err := vectordb.New(cfg.Qdrant); err == nil {
		qdrantClient = qc
		defer qdrantClient.Close()
	} else {
		log.Printf("Qdrant unavailable (fuzzy dedup disabled): %v", err)
	}

	// Ingestion pipeline
	ingestPipeline := &ingestion.Pipeline{
		Reader:   jobReader,
		Store:    descStore,
		Embedder: embedder,
		Qdrant:   qdrantClient,
	}

	// Extraction service
	rulesExt, err := extraction.NewRulesExtractor(cfg.Extraction.TechDictionary)
	if err != nil {
		log.Fatalf("rules extractor: %v", err)
	}
	var llmExt *extraction.LLMExtractor
	if cfg.Extraction.LLM.Enabled {
		llmExt = extraction.NewLLMExtractor(cfg.Extraction.LLM.Endpoint, cfg.Extraction.LLM.Model)
		log.Printf("LLM extraction enabled: %s / %s", cfg.Extraction.LLM.Endpoint, cfg.Extraction.LLM.Model)
	}
	extractSvc := extraction.NewService(cfg.Extraction.Mode, rulesExt, llmExt)

	// Clustering service
	dbscan := clustering.NewDBSCAN(cfg.Clustering.MinClusterSize, 1.0-cfg.Clustering.SimilarityThreshold)
	clusterSvc := clustering.NewService(painPointStore, clusterStore, embedder, dbscan)

	// Brief generator — uses Francis (trackr.llm) when configured, falls back to rule-based.
	briefLLMEndpoint := cfg.Trackr.LLM.Endpoint
	if briefLLMEndpoint == "" {
		briefLLMEndpoint = cfg.Extraction.LLM.Endpoint
	}
	briefLLMModel := cfg.Trackr.LLM.Model
	if briefLLMModel == "" {
		briefLLMModel = cfg.Extraction.LLM.Model
	}
	var briefGen *briefs.Generator
	if briefLLMEndpoint != "" && briefLLMModel != "" {
		briefSource := "local_llm"
		if cfg.Trackr.LLM.Endpoint != "" {
			briefSource = "francis"
		}
		briefGen = briefs.NewGeneratorWithLLM(clusterStore, briefSource, briefLLMEndpoint, briefLLMModel)
		log.Printf("Brief generator: LLM refinement enabled via %s / %s (source: %s)", briefLLMEndpoint, briefLLMModel, briefSource)
	} else {
		briefGen = briefs.NewGenerator(clusterStore)
		log.Printf("Brief generator: rule-based (no LLM configured)")
	}

	// Post-ingest pipeline service
	pipelineSvc := pipeline.New(descStore, painPointStore, clusterStore, extractSvc, clusterSvc, briefGen, briefStore)

	// Trackr service
	projectStore := repository.NewProjectStore(db)
	trackrSvc := trackr.NewService(projectStore, briefStore)

	// Auto-seed projects after pipeline generates new briefs
	pipelineSvc.OnComplete = func() {
		briefs, err := briefStore.List()
		if err != nil {
			log.Printf("trackr auto-seed: list briefs: %v", err)
			return
		}
		for _, b := range briefs {
			if _, err := trackrSvc.EnsureProject(b); err != nil {
				log.Printf("trackr auto-seed: brief %d: %v", b.ID, err)
			}
		}
		log.Printf("trackr auto-seed: ensured projects for %d briefs", len(briefs))
	}

	// LinkedIn generator (falls back to extraction LLM config if trackr-specific is blank)
	var linkedinGen *linkedin.Generator
	if cfg.Trackr.Enabled {
		llmEndpoint := cfg.Trackr.LLM.Endpoint
		if llmEndpoint == "" {
			llmEndpoint = cfg.Extraction.LLM.Endpoint
		}
		llmModel := cfg.Trackr.LLM.Model
		if llmModel == "" {
			llmModel = cfg.Extraction.LLM.Model
		}
		if llmEndpoint != "" && llmModel != "" {
			linkedinGen = linkedin.NewGenerator(llmEndpoint, llmModel)
			log.Printf("Trackr LinkedIn generator enabled: %s / %s", llmEndpoint, llmModel)
		}
	}

	r := chi.NewRouter()
	handlers.Register(r, &handlers.Dependencies{
		Pipeline:       ingestPipeline,
		DescStore:      descStore,
		PainPointStore: painPointStore,
		ClusterStore:   clusterStore,
		PipelineSvc:    pipelineSvc,
		JobReader:      jobReader,
		SettingsStore:  settingsStore,
		BriefsDeps: &handlers.BriefsDeps{
			Generator:    briefGen,
			BriefStore:   briefStore,
			ClusterStore: clusterStore,
			DescStore:    descStore,
			ProjectStore: projectStore,
			Trackr:       trackrSvc,
		},
		TrackrDeps: &handlers.TrackrDeps{
			Service:      trackrSvc,
			BriefStore:   briefStore,
			ProjectStore: projectStore,
			Generator:    linkedinGen,
			BriefsDeps:   &handlers.BriefsDeps{
				Generator:    briefGen,
				BriefStore:   briefStore,
				ClusterStore: clusterStore,
				DescStore:    descStore,
				ProjectStore: projectStore,
				Trackr:       trackrSvc,
			},
		},
	})

	addr := cfg.Server.Host + ":" + cfg.Server.Port
	log.Printf("Projctr listening on %s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("server: %v", err)
	}
}
