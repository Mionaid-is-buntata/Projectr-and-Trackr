package config

import (
	"bufio"
	"os"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

// Config holds all application configuration loaded from config.toml and
// environment variable overrides.
type Config struct {
	Server     ServerConfig
	Database   DatabaseConfig
	Huntr      HuntrConfig
	ChromaDB   ChromaDBConfig
	Qdrant     QdrantConfig
	Embedding  EmbeddingConfig
	Extraction ExtractionConfig
	Clustering ClusteringConfig
	Ingestion  IngestionConfig
	Trackr     TrackrConfig
}

type TrackrConfig struct {
	Enabled bool            `toml:"enabled"`
	LLM     TrackrLLMConfig `toml:"llm"`
}

type TrackrLLMConfig struct {
	Endpoint string `toml:"endpoint"`
	Model    string `toml:"model"`
}

type ServerConfig struct {
	Port string
	Host string
}

type DatabaseConfig struct {
	Path string
}

type HuntrConfig struct {
	JobsPath string  `toml:"jobs_path"` // Path to jobs/scored/ JSON files
	ScoreMin float64 `toml:"score_min"` // Lower bound (inclusive). Jobs below this are too weak a match.
	ScoreMax float64 `toml:"score_max"` // Upper bound (exclusive). Jobs at or above this are already a strong match.
}

type ChromaDBConfig struct {
	URL        string `toml:"url"`        // HTTP URL (e.g. http://localhost:8000) — preferred
	Path       string `toml:"path"`       // Path for PersistentClient (alternative)
	Collection string `toml:"collection"` // CV collection name from Huntr
}

type QdrantConfig struct {
	Host                  string `toml:"host"`
	Port                  int    `toml:"port"`
	Collection            string `toml:"collection"`             // Pain points collection
	DescriptionCollection string `toml:"description_collection"` // Description embeddings for fuzzy dedup
	VectorDimensions      int    `toml:"vector_dimensions"`
}

type EmbeddingConfig struct {
	Model    string `toml:"model"`
	Endpoint string `toml:"endpoint"`
}

type ExtractionConfig struct {
	Mode           string    `toml:"mode"` // "rules" or "llm"
	TechDictionary string    `toml:"tech_dictionary"`
	LLM            LLMConfig `toml:"llm"`
}

type LLMConfig struct {
	Enabled  bool   `toml:"enabled"`
	Endpoint string `toml:"endpoint"`
	Model    string `toml:"model"`
}

type ClusteringConfig struct {
	MinClusterSize      int     `toml:"min_cluster_size"`
	SimilarityThreshold float64 `toml:"similarity_threshold"`
}

type IngestionConfig struct {
	Schedule string `toml:"schedule"`
	Time     string `toml:"time"`
}

// loadDotEnv reads a .env file and sets any env vars not already set in the environment.
// Shell environment always takes precedence. Silently skips if the file doesn't exist.
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return // file absent — not an error
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		// Strip surrounding quotes
		if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') ||
			(value[0] == '\'' && value[len(value)-1] == '\'')) {
			value = value[1 : len(value)-1]
		}
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}
}

// Load reads configuration from the given TOML file path.
// It first loads .env (if present) so env vars are available for overrides.
// Shell environment always takes precedence over .env.
func Load(path string) (*Config, error) {
	loadDotEnv(".env")

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if _, err := toml.Decode(string(data), &cfg); err != nil {
		return nil, err
	}

	// Environment overrides (shell / .env)
	if v := os.Getenv("DATABASE_PATH"); v != "" {
		cfg.Database.Path = v
	}
	if v := os.Getenv("HUNTR_JOBS_PATH"); v != "" {
		cfg.Huntr.JobsPath = v
	}
	if v := os.Getenv("CHROMADB_URL"); v != "" {
		cfg.ChromaDB.URL = v
	}
	if v := os.Getenv("CHROMADB_COLLECTION"); v != "" {
		cfg.ChromaDB.Collection = v
	}
	if v := os.Getenv("EMBEDDING_ENDPOINT"); v != "" {
		cfg.Embedding.Endpoint = v
	}
	if v := os.Getenv("LLM_ENDPOINT"); v != "" {
		cfg.Extraction.LLM.Endpoint = v
	}
	if v := os.Getenv("QDRANT_HOST"); v != "" {
		cfg.Qdrant.Host = v
	}
	if v := os.Getenv("QDRANT_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil && p > 0 {
			cfg.Qdrant.Port = p
		}
	}
	if v := os.Getenv("QDRANT_COLLECTION"); v != "" {
		cfg.Qdrant.Collection = v
	}
	if v := os.Getenv("QDRANT_DESCRIPTION_COLLECTION"); v != "" {
		cfg.Qdrant.DescriptionCollection = v
	}
	if v := os.Getenv("TRACKR_LLM_ENDPOINT"); v != "" {
		cfg.Trackr.LLM.Endpoint = v
	}
	if v := os.Getenv("TRACKR_LLM_MODEL"); v != "" {
		cfg.Trackr.LLM.Model = v
	}

	return &cfg, nil
}
